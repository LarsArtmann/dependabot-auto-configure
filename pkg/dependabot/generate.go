package dependabot

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/larsartmann/go-finding"
	autoconfigure "github.com/larsartmann/linter-autoconfigure-sdk"
)

// MaxModuleEntries caps how many gomod entries generation creates. Beyond
// this, Dependabot's per-repo PR budget gets absurd; the cap is reported as
// a finding instead of silently generating dozens of entries.
const MaxModuleEntries = 20

// ConfigIssue is this package's issue vocabulary, aliased from the shared
// autoconfigure SDK so findings and suggestions speak one shape.
type ConfigIssue = autoconfigure.ConfigIssue

// RepoShape is what detection knows about a repository: the inputs
// generation needs. GoModuleDirs holds directories containing a go.mod,
// with the empty string denoting the repository root.
type RepoShape struct {
	GoModuleDirs     []string
	HasGitHubActions bool
	HasNPM           bool
}

// CapInfo reports whether generation had to cap the number of gomod entries.
type CapInfo struct {
	Capped       bool
	TotalModules int
}

// canonicalDir normalizes a detected module directory into Dependabot's
// directory syntax: "/" for the root, "/"-prefixed and cleaned otherwise.
func canonicalDir(dir string) string {
	if dir == "" || dir == "." {
		return "/"
	}

	return "/" + strings.TrimPrefix(strings.TrimPrefix(dir, "./"), "/")
}

// Generate builds the desired configuration for a repository shape. Entries
// are emitted in a stable order: root gomod, remaining gomod directories
// sorted, GitHub Actions, npm. When the module count exceeds
// MaxModuleEntries only the root gomod entry is generated and the returned
// CapInfo reports the cap.
func Generate(shape RepoShape) (Config, CapInfo) {
	cfg := Config{Version: CurrentVersion}
	cap := CapInfo{TotalModules: len(shape.GoModuleDirs)}

	dirs := make([]string, 0, len(shape.GoModuleDirs))
	dirs = append(dirs, shape.GoModuleDirs...)

	if len(dirs) > MaxModuleEntries {
		cap.Capped = true
		dirs = []string{""}
	} else {
		sort.Slice(dirs, func(i, j int) bool {
			ci, cj := canonicalDir(dirs[i]), canonicalDir(dirs[j])
			if ci == "/" {
				return true
			}
			if cj == "/" {
				return false
			}

			return ci < cj
		})
	}

	for _, dir := range dirs {
		cfg.Updates = append(cfg.Updates, Update{
			PackageEcosystem:      EcosystemGoModules,
			Directory:             canonicalDir(dir),
			Schedule:              &Schedule{Interval: IntervalWeekly},
			OpenPullRequestsLimit: DefaultOpenPullRequestsLimit,
			Groups:                MinorAndPatchGroups(),
		})
	}

	if shape.HasGitHubActions {
		cfg.Updates = append(cfg.Updates, Update{
			PackageEcosystem:      EcosystemGitHubActions,
			Directory:             "/",
			Schedule:              &Schedule{Interval: IntervalWeekly},
			OpenPullRequestsLimit: DefaultOpenPullRequestsLimit,
			Groups:                ActionGroups(),
		})
	}

	if shape.HasNPM {
		cfg.Updates = append(cfg.Updates, Update{
			PackageEcosystem:      EcosystemNPM,
			Directory:             "/",
			Schedule:              &Schedule{Interval: IntervalWeekly},
			OpenPullRequestsLimit: DefaultOpenPullRequestsLimit,
			Groups:                MinorAndPatchGroups(),
		})
	}

	return cfg, cap
}

// desiredGroups returns the canonical groups for an ecosystem.
func desiredGroups(eco Ecosystem) *Groups {
	if eco == EcosystemGitHubActions {
		return ActionGroups()
	}

	return MinorAndPatchGroups()
}

// Equal reports whether two configs are semantically identical (same
// version and same entries in the same order). Formatting differences
// (quoting, indentation) do not count, so a hand-written but semantically
// canonical config is a no-op, never a rewrite.
func Equal(a, b Config) bool {
	return reflect.DeepEqual(a, b)
}

// Reconcile merges desired entries into an existing configuration without
// destroying user intent: existing entries keep their non-canonical choices
// (a monthly schedule stays monthly), only missing fields are filled from
// the desired counterpart, missing entries are appended, and orphan entries
// (configurations for ecosystems this tool did not detect) are preserved.
// Reconcile assumes a safe (schema-known) existing config; callers gate it
// on DecodeResult.Unsafe.
func Reconcile(existing, desired Config) Config {
	out := Config{Version: CurrentVersion}

	for _, u := range existing.Updates {
		idx := desired.Find(u.PackageEcosystem, u.Directory)

		if u.Schedule == nil && idx >= 0 {
			u.Schedule = &Schedule{Interval: IntervalWeekly}
		}

		if u.OpenPullRequestsLimit == 0 && idx >= 0 {
			u.OpenPullRequestsLimit = desired.Updates[idx].OpenPullRequestsLimit
		}

		if u.Groups.Empty() && idx >= 0 {
			u.Groups = desiredGroups(u.PackageEcosystem)
		}

		out.Updates = append(out.Updates, u)
	}

	for _, want := range desired.Updates {
		if existing.Find(want.PackageEcosystem, want.Directory) >= 0 {
			continue
		}

		out.Updates = append(out.Updates, want)
	}

	return out
}

// Diff compares an existing configuration (nil = file missing) against the
// desired one and returns the issues a user or BuildFlow should see. Issues
// carry suggestions so they arrive as fixable findings, not dead ends.
func Diff(existing *Config, dec DecodeResult, desired Config, cap CapInfo) []autoconfigure.ConfigIssue {
	file := finding.FilePath(".github/dependabot.yml")
	issues := make([]autoconfigure.ConfigIssue, 0, 4)

	if existing == nil {
		if len(desired.Updates) == 0 {
			return issues
		}

		issues = append(issues, ConfigIssue{
			Rule:       "dependabot-config-missing",
			Message:    fmt.Sprintf("no %s found, but the repository has %d configurable ecosystem(s)", file, len(desired.Updates)),
			Severity:   finding.SeverityError,
			File:       file,
			Suggestion: "run dependabot-auto-configure to generate a grouped weekly configuration",
		})

		return issues
	}

	if dec.Config.Version != CurrentVersion {
		issues = append(issues, ConfigIssue{
			Rule:       "dependabot-version-outdated",
			Message:    fmt.Sprintf("dependabot config version is %d, but %d is the only version GitHub supports", dec.Config.Version, CurrentVersion),
			Severity:   finding.SeverityError,
			File:       file,
			Suggestion: fmt.Sprintf("set version to %d", CurrentVersion),
		})
	}

	for _, want := range desired.Updates {
		idx := existing.Find(want.PackageEcosystem, want.Directory)
		if idx < 0 {
			issues = append(issues, ConfigIssue{
				Rule:       "dependabot-entry-missing",
				Message:    fmt.Sprintf("no updates entry for ecosystem %q in directory %q", want.PackageEcosystem, want.Directory),
				Severity:   finding.SeverityWarning,
				File:       file,
				Suggestion: fmt.Sprintf("add a %q entry for %q with weekly schedule, limit %d, and grouped minor/patch updates", want.PackageEcosystem, want.Directory, DefaultOpenPullRequestsLimit),
			})

			continue
		}

		got := existing.Updates[idx]

		if got.Schedule == nil {
			issues = append(issues, ConfigIssue{
				Rule:       "dependabot-schedule-missing",
				Message:    fmt.Sprintf("entry %q/%q has no schedule interval", want.PackageEcosystem, want.Directory),
				Severity:   finding.SeverityWarning,
				File:       file,
				Suggestion: fmt.Sprintf("set schedule interval to %q", IntervalWeekly),
			})
		}

		if got.OpenPullRequestsLimit == 0 {
			issues = append(issues, ConfigIssue{
				Rule:       "dependabot-limit-missing",
				Message:    fmt.Sprintf("entry %q/%q has no open-pull-requests-limit, so Dependabot defaults to 5 silently", want.PackageEcosystem, want.Directory),
				Severity:   finding.SeverityWarning,
				File:       file,
				Suggestion: fmt.Sprintf("set open-pull-requests-limit to %d explicitly", DefaultOpenPullRequestsLimit),
			})
		}

		if got.Groups.Empty() {
			issues = append(issues, ConfigIssue{
				Rule:       "dependabot-grouping-missing",
				Message:    fmt.Sprintf("entry %q/%q has no update groups, so minor and patch bumps flood the PR queue", want.PackageEcosystem, want.Directory),
				Severity:   finding.SeverityWarning,
				File:       file,
				Suggestion: fmt.Sprintf("add a %q group (minor+patch) or a %q pattern group for actions", GroupMinorAndPatch, GroupActions),
			})
		}
	}

	for _, got := range existing.Updates {
		if desired.Find(got.PackageEcosystem, got.Directory) < 0 {
			issues = append(issues, ConfigIssue{
				Rule:     "dependabot-entry-orphan",
				Message:  fmt.Sprintf("entry for ecosystem %q in directory %q matches nothing detected in the repository (kept as-is)", got.PackageEcosystem, got.Directory),
				Severity: finding.SeverityInfo,
				File:     file,
			})
		}
	}

	if cap.Capped {
		issues = append(issues, ConfigIssue{
			Rule:       "dependabot-modules-capped",
			Message:    fmt.Sprintf("repository has %d Go modules; generation capped at %d entries (root only)", cap.TotalModules, MaxModuleEntries),
			Severity:   finding.SeverityInfo,
			File:       file,
			Suggestion: fmt.Sprintf("configure the remaining module directories manually or raise the %d-entry cap", MaxModuleEntries),
		})
	}

	return issues
}

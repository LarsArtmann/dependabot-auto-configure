package dependabot

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/go-faster/yaml"
	"github.com/larsartmann/go-finding"
	autoconfigure "github.com/larsartmann/linter-autoconfigure-sdk"
)

// MaxModuleEntries caps how many gomod entries generation creates. Beyond
// this, Dependabot's per-repo PR budget gets absurd; the cap is reported as
// a finding instead of silently generating dozens of entries.
const MaxModuleEntries = 20

const initialDiffCapacity = 4

// ConfigIssue is this package's issue vocabulary, aliased from the shared
// autoconfigure SDK so findings and suggestions speak one shape.
type ConfigIssue = autoconfigure.ConfigIssue

// RepoShape is what detection knows about a repository: the inputs
// generation needs. Directory lists hold the directories containing a
// manifest for their ecosystem, with the empty string denoting the
// repository root.
type RepoShape struct {
	GoModuleDirs     []string
	HasGitHubActions bool
	NPMDirs          []string
	PipDirs          []string
	CargoDirs        []string
	GradleDirs       []string
}

// CapInfo reports whether generation had to cap the number of gomod entries.
type CapInfo struct {
	Capped       bool
	TotalModules int
}

// CanonicalDir normalizes a detected module directory into Dependabot's
// directory syntax: "/" for the root, "/"-prefixed and cleaned otherwise.
func CanonicalDir(dir string) string {
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
	capInfo := CapInfo{TotalModules: len(shape.GoModuleDirs)}

	dirs := make([]string, 0, len(shape.GoModuleDirs))
	dirs = append(dirs, shape.GoModuleDirs...)

	if len(dirs) > MaxModuleEntries {
		capInfo.Capped = true
		dirs = []string{""}
	} else {
		sort.Slice(dirs, func(i, j int) bool {
			ci, cj := CanonicalDir(dirs[i]), CanonicalDir(dirs[j])
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
			Directory:             CanonicalDir(dir),
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

	for _, eco := range []struct {
		ecosystem Ecosystem
		dirs      []string
	}{
		{EcosystemNPM, shape.NPMDirs},
		{EcosystemPip, shape.PipDirs},
		{EcosystemCargo, shape.CargoDirs},
		{EcosystemGradle, shape.GradleDirs},
	} {
		cfg.Updates = append(cfg.Updates, ecosystemEntries(eco.ecosystem, eco.dirs)...)
	}

	return cfg, capInfo
}

// ecosystemEntries builds one canonical entry per directory, root first.
func ecosystemEntries(eco Ecosystem, dirs []string) []Update {
	sorted := make([]string, 0, len(dirs))
	sorted = append(sorted, dirs...)
	sort.Slice(sorted, func(i, j int) bool {
		ci, cj := CanonicalDir(sorted[i]), CanonicalDir(sorted[j])
		if ci == "/" {
			return true
		}

		if cj == "/" {
			return false
		}

		return ci < cj
	})

	entries := make([]Update, 0, len(sorted))
	for _, dir := range sorted {
		entries = append(entries, Update{
			PackageEcosystem:      eco,
			Directory:             CanonicalDir(dir),
			Schedule:              &Schedule{Interval: IntervalWeekly},
			OpenPullRequestsLimit: DefaultOpenPullRequestsLimit,
			Groups:                MinorAndPatchGroups(),
		})
	}

	return entries
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
// (quoting, indentation, node positions and comments inside carried
// unmodeled nodes, empty-vs-absent lists) do not count, so a hand-written
// but semantically canonical config is a no-op, never a rewrite.
func Equal(a, b Config) bool {
	return reflect.DeepEqual(semanticConfig(a), semanticConfig(b))
}

// semanticConfig copies c into a canonical comparable form: raw YAML nodes
// stripped of position, style, and comment metadata, and empty-or-nil
// collections normalized to nil. Comparison then sees content only.
func semanticConfig(c Config) Config {
	out := c
	out.Unmodeled = semanticNodeMap(c.Unmodeled)
	out.Updates = copyUpdatesSemantic(c.Updates)

	return out
}

func copyUpdatesSemantic(updates []Update) []Update {
	if len(updates) == 0 {
		return nil
	}

	out := make([]Update, len(updates))
	for i, u := range updates {
		u.Directories = normalizeStrings(u.Directories)
		u.Labels = normalizeStrings(u.Labels)
		u.Unmodeled = semanticNodeMap(u.Unmodeled)

		if u.Schedule != nil {
			schedule := *u.Schedule
			schedule.Unmodeled = semanticNodeMap(schedule.Unmodeled)
			u.Schedule = &schedule
		}

		if u.CommitMessage != nil {
			commitMessage := *u.CommitMessage
			commitMessage.Unmodeled = semanticNodeMap(commitMessage.Unmodeled)
			u.CommitMessage = &commitMessage
		}

		if u.Groups != nil {
			groups := *u.Groups
			groups.Unmodeled = semanticNodeMap(groups.Unmodeled)

			if groups.MinorAndPatch != nil {
				group := *groups.MinorAndPatch
				group.UpdateTypes = normalizeUpdateTypes(group.UpdateTypes)
				group.Unmodeled = semanticNodeMap(group.Unmodeled)
				groups.MinorAndPatch = &group
			}

			if groups.Actions != nil {
				group := *groups.Actions
				group.Patterns = normalizeStrings(group.Patterns)
				group.Unmodeled = semanticNodeMap(group.Unmodeled)
				groups.Actions = &group
			}

			u.Groups = &groups
		}

		out[i] = u
	}

	return out
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	return values
}

func normalizeUpdateTypes(values []UpdateType) []UpdateType {
	if len(values) == 0 {
		return nil
	}

	return values
}

func semanticNodeMap(m map[string]yaml.Node) map[string]yaml.Node {
	if len(m) == 0 {
		return nil
	}

	out := make(map[string]yaml.Node, len(m))
	for key, node := range m {
		out[key] = semanticNode(node)
	}

	return out
}

// semanticNode copies node with position, style, and comment metadata
// zeroed, recursively through content and aliases: two nodes compare equal
// when their content is equal, wherever it sat in the file.
func semanticNode(node yaml.Node) yaml.Node {
	node.Line, node.Column, node.Style = 0, 0, 0
	node.HeadComment, node.LineComment, node.FootComment = "", "", ""

	if node.Alias != nil {
		alias := semanticNode(*node.Alias)
		node.Alias = &alias
	}

	if node.Content != nil {
		node.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			if child == nil {
				continue
			}

			semantic := semanticNode(*child)
			node.Content[i] = &semantic
		}
	}

	return node
}

// scheduleMissing reports whether an entry has no usable schedule: either
// no schedule block at all or one with an empty interval, which GitHub
// rejects.
func scheduleMissing(u Update) bool {
	return u.Schedule == nil || u.Schedule.Interval == ""
}

// Reconcile merges desired entries into an existing configuration without
// destroying user intent: existing entries keep their non-canonical choices
// (a monthly schedule stays monthly, unmodeled fields ride along verbatim),
// only missing fields are filled from the desired counterpart, missing
// entries are appended, and orphan entries (configurations for ecosystems
// this tool did not detect) are preserved. Reconcile assumes a safe
// (round-trippable) existing config; callers gate it on DecodeResult.Unsafe.
func Reconcile(existing, desired Config) Config {
	out := Config{Version: CurrentVersion, Unmodeled: existing.Unmodeled}

	for _, update := range existing.Updates {
		idx := desiredIndexOf(update, desired)

		if scheduleMissing(update) && idx >= 0 {
			if update.Schedule == nil {
				update.Schedule = &Schedule{Interval: IntervalWeekly}
			} else {
				update.Schedule.Interval = IntervalWeekly
			}
		}

		if update.OpenPullRequestsLimit == 0 && idx >= 0 {
			update.OpenPullRequestsLimit = desired.Updates[idx].OpenPullRequestsLimit
		}

		if update.Groups.Empty() && !update.HasUnmodeledGroups() && idx >= 0 {
			update.Groups = desiredGroups(update.PackageEcosystem)
		}

		out.Updates = append(out.Updates, update)
	}

	for _, want := range desired.Updates {
		if existing.Find(want.PackageEcosystem, want.Directory) >= 0 {
			continue
		}

		out.Updates = append(out.Updates, want)
	}

	return out
}

// desiredIndexOf returns the index of the desired entry an existing entry
// covers (same ecosystem; the desired directory is the entry's directory or
// matched by its `directories` globs), or -1 for user-managed entries.
func desiredIndexOf(update Update, desired Config) int {
	for i, want := range desired.Updates {
		if update.PackageEcosystem == want.PackageEcosystem && update.coversDir(want.Directory) {
			return i
		}
	}

	return -1
}

// ecosystemDirs maps the canonical Dependabot directory of every detected
// manifest directory for the given ecosystem. Ecosystems RepoShape does
// not model have no directories, so their entries are user-managed and
// audited by the orphan rule instead of silently trusted.
func ecosystemDirs(shape RepoShape, eco Ecosystem) map[string]bool {
	var list []string

	switch eco {
	case EcosystemGoModules:
		list = shape.GoModuleDirs
	case EcosystemGitHubActions:
		// A single root-level entry tracked by HasGitHubActions, not a
		// directory list.
		return map[string]bool{}
	case EcosystemNPM:
		list = shape.NPMDirs
	case EcosystemPip:
		list = shape.PipDirs
	case EcosystemCargo:
		list = shape.CargoDirs
	case EcosystemGradle:
		list = shape.GradleDirs
	default:
		return map[string]bool{}
	}

	dirs := make(map[string]bool, len(list))
	for _, dir := range list {
		dirs[CanonicalDir(dir)] = true
	}

	return dirs
}

// entryDetected reports whether an existing updates entry corresponds to
// something detection found. Entries using the glob `directories` form
// match any detected directory their patterns cover; an unparseable
// pattern counts as detected (a false orphan is noise, a false entry-
// missing would duplicate coverage).
func entryDetected(u Update, shape RepoShape) bool {
	if u.PackageEcosystem == EcosystemGitHubActions {
		return shape.HasGitHubActions
	}

	detected := ecosystemDirs(shape, u.PackageEcosystem)
	if detected[u.Directory] {
		return true
	}

	for _, pattern := range u.Directories {
		for dir := range detected {
			if match, err := path.Match(pattern, dir); err != nil || match {
				return true
			}
		}
	}

	return false
}

// Diff compares an existing configuration (nil = file missing) against the
// desired one and returns the issues a user or BuildFlow should see. Issues
// carry suggestions so they arrive as fixable findings, not dead ends. file
// is the config path findings should point at. shape is the DETECTED
// repository shape — orphan detection uses it rather than desired, so the
// module cap never mislabels a real module entry as an orphan.
func Diff(
	existing *Config,
	dec DecodeResult,
	desired Config,
	shape RepoShape,
	capInfo CapInfo,
	file finding.FilePath,
) []autoconfigure.ConfigIssue {
	issues := make([]autoconfigure.ConfigIssue, 0, initialDiffCapacity)

	if existing == nil {
		if len(desired.Updates) == 0 {
			return issues
		}

		issues = append(issues, ConfigIssue{
			Rule: "dependabot-config-missing",
			Message: fmt.Sprintf(
				"no %s found, but the repository has %d configurable ecosystem(s)",
				file,
				len(desired.Updates),
			),
			Severity:   finding.SeverityError,
			File:       file,
			Suggestion: "run dependabot-auto-configure to generate a grouped weekly configuration",
		})

		return issues
	}

	if invalid := dec.Config.Validate(); invalid != nil {
		issues = append(issues, ConfigIssue{
			Rule:    "dependabot-entry-invalid",
			Message: fmt.Sprintf("existing config has an invalid entry and was left untouched: %v", invalid),
			// Warning, not error: repair refuses to write these by design,
			// so a higher severity would be a permanent gate failure.
			Severity:   finding.SeverityWarning,
			File:       file,
			Suggestion: "give every updates entry a package-ecosystem and a directory (or directories), or remove the broken entry",
		})
	}

	if dec.Config.Version != CurrentVersion {
		issues = append(issues, ConfigIssue{
			Rule: "dependabot-version-outdated",
			Message: fmt.Sprintf(
				"dependabot config version is %d, but %d is the only version GitHub supports",
				dec.Config.Version,
				CurrentVersion,
			),
			Severity:   finding.SeverityError,
			File:       file,
			Suggestion: fmt.Sprintf("set version to %d", CurrentVersion),
		})
	}

	for _, want := range desired.Updates {
		idx := existing.Find(want.PackageEcosystem, want.Directory)
		if idx < 0 {
			issues = append(issues, ConfigIssue{
				Rule: "dependabot-entry-missing",
				Message: fmt.Sprintf(
					"no updates entry for ecosystem %q in directory %q",
					want.PackageEcosystem,
					want.Directory,
				),
				Severity: finding.SeverityWarning,
				File:     file,
				Suggestion: fmt.Sprintf(
					"add a %q entry for %q with weekly schedule, limit %d, and grouped minor/patch updates",
					want.PackageEcosystem,
					want.Directory,
					DefaultOpenPullRequestsLimit,
				),
			})

			continue
		}

		got := existing.Updates[idx]

		issues = append(issues, entryIssues(got, want, file)...)
	}

	for _, got := range existing.Updates {
		// Orphan detection compares against detection, NOT against the
		// (possibly capped) desired config: when the module cap truncates
		// generation to root-only, entries for genuinely detected modules
		// are user-managed, not orphans.
		if entryDetected(got, shape) {
			continue
		}

		issues = append(issues, ConfigIssue{
			Rule: "dependabot-entry-orphan",
			Message: fmt.Sprintf(
				"entry for ecosystem %q in directory %q matches nothing detected in the repository (kept as-is)",
				got.PackageEcosystem,
				got.Directory,
			),
			Severity: finding.SeverityInfo,
			File:     file,
			Suggestion: "remove the entry if it is stale, or keep it — this tool leaves " +
				"ecosystems and directories it does not detect (pip, cargo, nested npm, ...) alone",
		})
	}

	if capInfo.Capped {
		issues = append(issues, ConfigIssue{
			Rule: "dependabot-modules-capped",
			Message: fmt.Sprintf(
				"repository has %d Go modules; generation capped at %d entries (root only)",
				capInfo.TotalModules,
				MaxModuleEntries,
			),
			Severity: finding.SeverityInfo,
			File:     file,
			Suggestion: fmt.Sprintf(
				"configure the remaining module directories manually or raise the %d-entry cap",
				MaxModuleEntries,
			),
		})
	}

	return issues
}

// entryIssues reports the canonical fields an existing entry is missing:
// schedule interval, explicit PR limit, and update groups. Entries whose
// groups use names outside the canonical model are not "missing" groups —
// Update.HasUnmodeledGroups holds the grouping-missing finding, and since
// v0.4.0 the names ride in Groups.Unmodeled, preserved verbatim.
func entryIssues(got Update, want Update, file finding.FilePath) []autoconfigure.ConfigIssue {
	var issues []autoconfigure.ConfigIssue

	if scheduleMissing(got) {
		issues = append(issues, ConfigIssue{
			Rule: "dependabot-schedule-missing",
			Message: fmt.Sprintf(
				"entry %q/%q has no schedule interval, so Dependabot runs "+
					"on its implicit default instead of the explicit weekly cadence",
				want.PackageEcosystem,
				want.Directory,
			),
			Severity:   finding.SeverityWarning,
			File:       file,
			Suggestion: fmt.Sprintf("set schedule interval to %q", IntervalWeekly),
		})
	}

	if got.OpenPullRequestsLimit == 0 {
		issues = append(issues, ConfigIssue{
			Rule: "dependabot-limit-missing",
			Message: fmt.Sprintf(
				"entry %q/%q has no open-pull-requests-limit, so Dependabot defaults to 5 silently",
				want.PackageEcosystem,
				want.Directory,
			),
			Severity:   finding.SeverityWarning,
			File:       file,
			Suggestion: fmt.Sprintf("set open-pull-requests-limit to %d explicitly", DefaultOpenPullRequestsLimit),
		})
	}

	if got.Groups.Empty() && !got.HasUnmodeledGroups() {
		issues = append(issues, ConfigIssue{
			Rule: "dependabot-grouping-missing",
			Message: fmt.Sprintf(
				"entry %q/%q has no update groups, so minor and patch bumps flood the PR queue",
				want.PackageEcosystem,
				want.Directory,
			),
			Severity: finding.SeverityWarning,
			File:     file,
			Suggestion: fmt.Sprintf(
				"add a %q group (minor+patch) or a %q pattern group for actions",
				GroupMinorAndPatch,
				GroupActions,
			),
		})
	}

	return issues
}

package dependabot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		shape         dependabot.RepoShape
		wantDirs      []string // (ecosystem, directory) pairs flattened
		wantEcosystem []string
		wantCapped    bool
	}{
		{
			name:          "empty repo generates nothing",
			shape:         dependabot.RepoShape{},
			wantEcosystem: []string{},
		},
		{
			name:          "root go module only",
			shape:         dependabot.RepoShape{GoModuleDirs: []string{""}},
			wantEcosystem: []string{"gomod"},
			wantDirs:      []string{"/"},
		},
		{
			name: "multi-module repo sorted with root first",
			shape: dependabot.RepoShape{
				GoModuleDirs:     []string{"modules/utils", "", "modules/checks"},
				HasGitHubActions: true,
			},
			wantEcosystem: []string{"gomod", "gomod", "gomod", "github-actions"},
			wantDirs:      []string{"/", "/modules/checks", "/modules/utils", "/"},
		},
		{
			name:          "npm only",
			shape:         dependabot.RepoShape{HasNPM: true},
			wantEcosystem: []string{"npm"},
			wantDirs:      []string{"/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, capInfo := dependabot.Generate(tt.shape)

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Generate() produced invalid config: %v", err)
			}

			if capInfo.Capped != tt.wantCapped {
				t.Errorf("CapInfo.Capped = %v, want %v", capInfo.Capped, tt.wantCapped)
			}

			if len(cfg.Updates) != len(tt.wantEcosystem) {
				t.Fatalf(
					"Generate() produced %d entries, want %d: %+v",
					len(cfg.Updates),
					len(tt.wantEcosystem),
					cfg.Updates,
				)
			}

			for i, u := range cfg.Updates {
				if string(u.PackageEcosystem) != tt.wantEcosystem[i] || u.Directory != tt.wantDirs[i] {
					t.Errorf(
						"entry %d = (%s, %s), want (%s, %s)",
						i,
						u.PackageEcosystem,
						u.Directory,
						tt.wantEcosystem[i],
						tt.wantDirs[i],
					)
				}
			}
		})
	}
}

func TestGenerateCapsModuleCount(t *testing.T) {
	t.Parallel()

	shape := dependabot.RepoShape{GoModuleDirs: []string{""}}
	for i := range dependabot.MaxModuleEntries + 5 {
		shape.GoModuleDirs = append(shape.GoModuleDirs, "module"+string(rune('a'+i)))
	}

	cfg, capInfo := dependabot.Generate(shape)

	if !capInfo.Capped || capInfo.TotalModules != len(shape.GoModuleDirs) {
		t.Errorf("CapInfo = %+v, want capped=true total=%d", capInfo, len(shape.GoModuleDirs))
	}

	if len(cfg.Updates) != 1 || cfg.Updates[0].Directory != "/" {
		t.Errorf("capped config = %+v, want single root entry", cfg.Updates)
	}
}

func TestCanonicalEntriesAreComplete(t *testing.T) {
	t.Parallel()

	cfg, _ := dependabot.Generate(
		dependabot.RepoShape{GoModuleDirs: []string{""}, HasGitHubActions: true, HasNPM: true},
	)

	for _, update := range cfg.Updates {
		if update.Schedule == nil || update.Schedule.Interval != dependabot.IntervalWeekly {
			t.Errorf("entry %s/%s missing weekly schedule", update.PackageEcosystem, update.Directory)
		}

		if update.OpenPullRequestsLimit != dependabot.DefaultOpenPullRequestsLimit {
			t.Errorf(
				"entry %s/%s limit = %d, want %d",
				update.PackageEcosystem,
				update.Directory,
				update.OpenPullRequestsLimit,
				dependabot.DefaultOpenPullRequestsLimit,
			)
		}

		if update.Groups.Empty() {
			t.Errorf("entry %s/%s has no groups", update.PackageEcosystem, update.Directory)
		}
	}

	actions := cfg.Updates[cfg.Find(dependabot.EcosystemGitHubActions, "/")]
	if actions.Groups.Actions == nil {
		t.Errorf("actions entry groups = %+v, want actions pattern group", actions.Groups)
	}

	gomod := cfg.Updates[cfg.Find(dependabot.EcosystemGoModules, "/")]
	if gomod.Groups.MinorAndPatch == nil {
		t.Errorf("gomod entry groups = %+v, want minor-and-patch group", gomod.Groups)
	}
}

func mustConfig(t *testing.T, yaml string) dependabot.Config {
	t.Helper()

	dec, err := dependabot.Decode([]byte(yaml))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	return dec.Config
}

func TestDiff(t *testing.T) {
	t.Parallel()

	shape := dependabot.RepoShape{GoModuleDirs: []string{""}, HasGitHubActions: true}
	desired, _ := dependabot.Generate(shape)

	tests := []struct {
		name      string
		existing  *string
		wantRules []string
	}{
		{
			name:      "missing config",
			existing:  nil,
			wantRules: []string{"dependabot-config-missing"},
		},
		{
			name: "canonical config has zero issues",
			existing: new(strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				"    schedule:",
				"      interval: weekly",
				"    open-pull-requests-limit: 5",
				"    groups:",
				"      minor-and-patch:",
				"        update-types:",
				"          - minor",
				"          - patch",
				"  - package-ecosystem: github-actions",
				"    directory: /",
				"    schedule:",
				"      interval: weekly",
				"    open-pull-requests-limit: 5",
				"    groups:",
				"      actions:",
				"        patterns:",
				"          - \"*\"",
				"",
			}, "\n")),
			wantRules: []string{},
		},
		{
			name: "bare entries report every gap",
			existing: new(strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				"  - package-ecosystem: github-actions",
				"    directory: /",
				"",
			}, "\n")),
			wantRules: []string{
				"dependabot-schedule-missing",
				"dependabot-limit-missing",
				"dependabot-grouping-missing",
				"dependabot-schedule-missing",
				"dependabot-limit-missing",
				"dependabot-grouping-missing",
			},
		},
		{
			name: "outdated version",
			existing: new(strings.Join([]string{
				"version: 1",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				"    schedule:",
				"      interval: weekly",
				"    open-pull-requests-limit: 5",
				"    groups:",
				"      minor-and-patch:",
				"        update-types:",
				"          - minor",
				"          - patch",
				"",
			}, "\n")),
			wantRules: []string{"dependabot-version-outdated", "dependabot-entry-missing"},
		},
		{
			name: "orphan entry preserved with info",
			existing: new(strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				"    schedule:",
				"      interval: weekly",
				"    open-pull-requests-limit: 5",
				"    groups:",
				"      minor-and-patch:",
				"        update-types:",
				"          - minor",
				"          - patch",
				"  - package-ecosystem: pip",
				"    directory: /tools",
				"    schedule:",
				"      interval: weekly",
				"",
			}, "\n")),
			wantRules: []string{"dependabot-entry-missing", "dependabot-entry-orphan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				existing *dependabot.Config
				dec      dependabot.DecodeResult
			)

			if tt.existing != nil {
				decoded, err := dependabot.Decode([]byte(*tt.existing))
				if err != nil {
					t.Fatalf("Decode() error = %v", err)
				}

				existing = &decoded.Config
				dec = decoded
			}

			issues := dependabot.Diff(existing, dec, desired, shape, dependabot.CapInfo{}, ".github/dependabot.yml")

			var gotRules []string
			for _, issue := range issues {
				gotRules = append(gotRules, string(issue.Rule))
			}

			if len(gotRules) != len(tt.wantRules) {
				t.Fatalf("Diff() rules = %v, want %v", gotRules, tt.wantRules)
			}

			for i := range gotRules {
				if gotRules[i] != tt.wantRules[i] {
					t.Errorf("Diff() rule %d = %s, want %s", i, gotRules[i], tt.wantRules[i])
				}
			}
		})
	}
}

func TestDiffMissingConfigButEmptyShape(t *testing.T) {
	t.Parallel()

	issues := dependabot.Diff(
		nil,
		dependabot.DecodeResult{},
		dependabot.Config{},
		dependabot.RepoShape{},
		dependabot.CapInfo{},
		".github/dependabot.yml",
	)
	if len(issues) != 0 {
		t.Errorf("Diff() on empty shape = %v issues, want 0", len(issues))
	}
}

// TestDiffCappedModuleEntryIsNotOrphan guards the orphan-vs-cap fix: with
// more than MaxModuleEntries Go modules, Generate truncates desired to
// root-only, but an existing entry for a genuinely DETECTED module is
// user-managed, not an orphan. Flagging it would contradict the cap
// finding's own advice to "configure the remaining module directories
// manually" (observed dogfooding on BuildFlow: 32 modules, every module
// entry mislabeled orphan).
func TestDiffCappedModuleEntryIsNotOrphan(t *testing.T) {
	t.Parallel()

	dirs := []string{""}

	for i := 1; i <= dependabot.MaxModuleEntries; i++ {
		dirs = append(dirs, fmt.Sprintf("mod%d", i))
	}

	shape := dependabot.RepoShape{GoModuleDirs: dirs}

	desired, capInfo := dependabot.Generate(shape)
	if !capInfo.Capped {
		t.Fatal("fixture not capped, want Capped=true")
	}

	cappedYAML := strings.Join([]string{
		"version: 2",
		"updates:",
		"  - package-ecosystem: gomod",
		"    directory: /",
		"    schedule:",
		"      interval: weekly",
		"    open-pull-requests-limit: 5",
		"    groups:",
		"      minor-and-patch:",
		"        update-types:",
		"          - minor",
		"          - patch",
		"  - package-ecosystem: gomod",
		"    directory: /mod1",
		"    schedule:",
		"      interval: weekly",
		"  - package-ecosystem: gomod",
		"    directory: /gone",
		"    schedule:",
		"      interval: weekly",
		"",
	}, "\n")

	existing := mustConfig(t, cappedYAML)

	decoded, err := dependabot.Decode([]byte(cappedYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	issues := dependabot.Diff(&existing, decoded, desired, shape, capInfo, ".github/dependabot.yml")

	var orphans []string

	for _, issue := range issues {
		if string(issue.Rule) == "dependabot-entry-orphan" {
			orphans = append(orphans, issue.Message)
		}
	}

	if len(orphans) != 1 || !strings.Contains(orphans[0], "/gone") {
		t.Errorf("Diff() orphans = %v, want exactly /gone (detected /mod1 must NOT be orphan under the cap)", orphans)
	}
}

func TestReconcile(t *testing.T) {
	t.Parallel()
	existing := mustConfig(t, strings.Join([]string{
		"version: 2",
		"updates:",
		"  - package-ecosystem: gomod",
		"    directory: /",
		"    schedule:",
		"      interval: monthly",
		"  - package-ecosystem: pip",
		"    directory: /tools",
		"    schedule:",
		"      interval: weekly",
		"",
	}, "\n"))

	desired, _ := dependabot.Generate(dependabot.RepoShape{GoModuleDirs: []string{""}, HasNPM: true})

	got := dependabot.Reconcile(existing, desired)

	if got.Version != dependabot.CurrentVersion {
		t.Errorf("Reconcile() version = %d, want %d", got.Version, dependabot.CurrentVersion)
	}

	root := got.Updates[got.Find(dependabot.EcosystemGoModules, "/")]
	if root.Schedule.Interval != "monthly" {
		t.Errorf("Reconcile() kept schedule = %q, want monthly preserved", root.Schedule.Interval)
	}

	if root.OpenPullRequestsLimit != dependabot.DefaultOpenPullRequestsLimit {
		t.Errorf("Reconcile() limit = %d, want filled from desired", root.OpenPullRequestsLimit)
	}

	if root.Groups.Empty() {
		t.Error("Reconcile() groups empty, want filled from desired")
	}

	if got.Find(dependabot.EcosystemNPM, "/") < 0 {
		t.Error("Reconcile() missing appended npm entry")
	}

	if got.Find("pip", "/tools") < 0 {
		t.Error("Reconcile() dropped orphan pip entry")
	}
}

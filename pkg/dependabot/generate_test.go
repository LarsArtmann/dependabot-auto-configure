package dependabot_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

func TestGenerate(t *testing.T) {
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
			cfg, cap := dependabot.Generate(tt.shape)

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Generate() produced invalid config: %v", err)
			}

			if cap.Capped != tt.wantCapped {
				t.Errorf("CapInfo.Capped = %v, want %v", cap.Capped, tt.wantCapped)
			}

			if len(cfg.Updates) != len(tt.wantEcosystem) {
				t.Fatalf("Generate() produced %d entries, want %d: %+v", len(cfg.Updates), len(tt.wantEcosystem), cfg.Updates)
			}

			for i, u := range cfg.Updates {
				if string(u.PackageEcosystem) != tt.wantEcosystem[i] || u.Directory != tt.wantDirs[i] {
					t.Errorf("entry %d = (%s, %s), want (%s, %s)", i, u.PackageEcosystem, u.Directory, tt.wantEcosystem[i], tt.wantDirs[i])
				}
			}
		})
	}
}

func TestGenerateCapsModuleCount(t *testing.T) {
	shape := dependabot.RepoShape{GoModuleDirs: []string{""}}
	for i := range dependabot.MaxModuleEntries + 5 {
		shape.GoModuleDirs = append(shape.GoModuleDirs, "module"+string(rune('a'+i)))
	}

	cfg, cap := dependabot.Generate(shape)

	if !cap.Capped || cap.TotalModules != len(shape.GoModuleDirs) {
		t.Errorf("CapInfo = %+v, want capped=true total=%d", cap, len(shape.GoModuleDirs))
	}

	if len(cfg.Updates) != 1 || cfg.Updates[0].Directory != "/" {
		t.Errorf("capped config = %+v, want single root entry", cfg.Updates)
	}
}

func TestCanonicalEntriesAreComplete(t *testing.T) {
	cfg, _ := dependabot.Generate(dependabot.RepoShape{GoModuleDirs: []string{""}, HasGitHubActions: true, HasNPM: true})

	for _, u := range cfg.Updates {
		if u.Schedule == nil || u.Schedule.Interval != dependabot.IntervalWeekly {
			t.Errorf("entry %s/%s missing weekly schedule", u.PackageEcosystem, u.Directory)
		}

		if u.OpenPullRequestsLimit != dependabot.DefaultOpenPullRequestsLimit {
			t.Errorf("entry %s/%s limit = %d, want %d", u.PackageEcosystem, u.Directory, u.OpenPullRequestsLimit, dependabot.DefaultOpenPullRequestsLimit)
		}

		if u.Groups.Empty() {
			t.Errorf("entry %s/%s has no groups", u.PackageEcosystem, u.Directory)
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
	desired, _ := dependabot.Generate(dependabot.RepoShape{GoModuleDirs: []string{""}, HasGitHubActions: true})

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

			issues := dependabot.Diff(existing, dec, desired, dependabot.CapInfo{})

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
	issues := dependabot.Diff(nil, dependabot.DecodeResult{}, dependabot.Config{}, dependabot.CapInfo{})
	if len(issues) != 0 {
		t.Errorf("Diff() on empty shape = %v issues, want 0", len(issues))
	}
}

func TestReconcile(t *testing.T) {
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

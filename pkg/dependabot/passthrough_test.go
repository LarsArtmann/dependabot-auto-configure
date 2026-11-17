package dependabot_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

// passthroughYAML carries one example of every unmodeled-but-real construct
// the fleet sweep found blocking repair: top-level registries and
// enable-beta-ecosystems, entry fields ignore/reviewers/assignees/
// target-branch/auto-merge/allow, an unknown schedule key, the unknown
// commit-message key prefix-development, and a custom-named group. None of
// these are typed; all of them must survive a Decode→Reconcile→Encode
// round-trip byte-for-value while the canonical fields fill in.
const passthroughYAML = `version: 2
registries:
  docker-hub:
    type: docker-registry
    url: registry.hub.docker.com
enable-beta-ecosystems: true
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: monthly
      cron: "0 6 * * 1"
    reviewers:
      - lars
    assignees:
      - lars
    target-branch: main
    auto-merge: true
    allow:
      - dependency-name: github.com/larsartmann/*
    ignore:
      - dependency-name: github.com/x/y
        versions: ["2.x"]
    commit-message:
      prefix: "chore(deps)"
      prefix-development: "chore(dev-deps)"
    groups:
      go-deps:
        patterns:
          - "github.com/larsartmann/*"
`

func TestDecodeUnmodeledConstructsSafe(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(passthroughYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if res.Unsafe {
		t.Fatalf("Decode() Unsafe = true, want false. Reasons: %v", res.UnsafeReasons)
	}

	if len(res.Config.Unmodeled) != 2 {
		t.Errorf("Decode() top-level Unmodeled = %v, want registries + enable-beta-ecosystems", res.Config.Unmodeled)
	}

	entry := res.Config.Updates[0]
	for _, key := range []string{
		"reviewers", "assignees", "target-branch", "auto-merge", "allow", "ignore",
	} {
		if _, carried := entry.Unmodeled[key]; !carried {
			t.Errorf("Decode() entry Unmodeled missing %q", key)
		}
	}

	if _, carried := entry.Schedule.Unmodeled["cron"]; !carried {
		t.Error("Decode() schedule Unmodeled missing \"cron\"")
	}

	if _, carried := entry.CommitMessage.Unmodeled["prefix-development"]; !carried {
		t.Error("Decode() commit-message Unmodeled missing \"prefix-development\"")
	}

	if !entry.HasUnmodeledGroups() {
		t.Error("Decode() HasUnmodeledGroups() = false, want true (custom name go-deps)")
	}
}

// TestPassthroughRepairFillsCanonicalFieldsWithoutDropping is the v0.4.0
// contract in one test: a config full of unmodeled schema decodes SAFE, so
// repair fills the missing canonical fields (weekly schedule is already
// present-but-monthly, so it stays monthly; the limit and groups fill in),
// appends the missing github-actions entry, and re-emits every unmodeled
// construct verbatim.
func TestPassthroughRepairFillsCanonicalFieldsWithoutDropping(t *testing.T) {
	t.Parallel()

	existing := mustConfig(t, passthroughYAML)
	desired, _ := dependabot.Generate(dependabot.RepoShape{
		GoModuleDirs:     []string{""},
		HasGitHubActions: true,
	})

	got := dependabot.Reconcile(existing, desired)

	root := got.Updates[got.Find(dependabot.EcosystemGoModules, "/")]
	if root.Schedule == nil || root.Schedule.Interval != "monthly" {
		t.Errorf("Reconcile() overwrote the monthly interval: %+v", root.Schedule)
	}

	if root.OpenPullRequestsLimit != dependabot.DefaultOpenPullRequestsLimit {
		t.Errorf(
			"Reconcile() limit = %d, want filled to %d",
			root.OpenPullRequestsLimit,
			dependabot.DefaultOpenPullRequestsLimit,
		)
	}

	if root.HasUnmodeledGroups() {
		// The custom group stays unmodeled; Reconcile must not graft a
		// canonical group next to it (the user grouped on purpose).
		if !root.Groups.Empty() {
			t.Error("Reconcile() added canonical groups alongside the user's custom group, want groups left alone")
		}
	}

	encoded, err := got.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for _, want := range []string{
		"registries:", "docker-hub", "enable-beta-ecosystems: true",
		"reviewers:", "assignees:", "target-branch: main", "auto-merge: true",
		"allow:", "ignore:", "versions:", "prefix-development:",
		"go-deps:", "github.com/larsartmann/*", "interval: monthly", "cron:",
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("Encode() dropped %q:\n%s", want, encoded)
		}
	}

	if got.Find(dependabot.EcosystemGitHubActions, "/") < 0 {
		t.Error("Reconcile() missing appended github-actions entry")
	}

	redecoded, err := dependabot.Decode(encoded)
	if err != nil {
		t.Fatalf("re-Decode() error = %v", err)
	}

	if redecoded.Unsafe {
		t.Errorf("re-Decode() Unsafe = true, want false. Reasons: %v", redecoded.UnsafeReasons)
	}

	regot := dependabot.Reconcile(redecoded.Config, desired)
	if !dependabot.Equal(got, regot) {
		t.Errorf("reconcile not idempotent through passthrough encode:\nfirst: %+v\nsecond: %+v", got, regot)
	}

	reencoded, err := regot.Encode()
	if err != nil {
		t.Fatalf("re-Encode() error = %v", err)
	}

	if string(reencoded) != string(encoded) {
		t.Errorf("passthrough encode not byte-stable:\nfirst:\n%s\nsecond:\n%s", encoded, reencoded)
	}
}

// TestPassthroughDiffStaysSilentOnModeledState pins that a config carrying
// only unmodeled constructs and complete canonical fields produces NO
// findings — the warnings BuildFlow saw forever were exactly this shape.
func TestPassthroughDiffStaysSilentOnModeledState(t *testing.T) {
	t.Parallel()

	complete := strings.Replace(passthroughYAML,
		"    schedule:\n      interval: monthly\n",
		"    schedule:\n      interval: weekly\n", 1,
	) + `    open-pull-requests-limit: 5
`

	dec, err := dependabot.Decode([]byte(complete))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if dec.Unsafe {
		t.Fatalf("Decode() Unsafe = true, want false. Reasons: %v", dec.UnsafeReasons)
	}

	desired, _ := dependabot.Generate(dependabot.RepoShape{GoModuleDirs: []string{""}})

	issues := dependabot.Diff(
		&dec.Config,
		dec,
		desired,
		dependabot.RepoShape{GoModuleDirs: []string{""}},
		dependabot.CapInfo{},
		".github/dependabot.yml",
	)
	if len(issues) != 0 {
		t.Errorf("Diff() = %v, want no issues for an unmodeled-but-complete config", issues)
	}
}

func TestDecodeAliasInUnmodeledUnsafe(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(`version: 2
x-common: &common
  a: 1
updates:
  - package-ecosystem: gomod
    directory: /
    extra: *common
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if !res.Unsafe {
		t.Fatal("Decode() Unsafe = false for an alias inside an unmodeled key, want true")
	}

	if len(res.UnsafeReasons) != 1 || !strings.Contains(res.UnsafeReasons[0], "alias") {
		t.Errorf("Decode() UnsafeReasons = %v, want one alias reason", res.UnsafeReasons)
	}
}

func TestDecodeAliasInsideUnmodeledSequenceUnsafe(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(`version: 2
anchors: &list
  - a
updates:
  - package-ecosystem: gomod
    directory: /
    ignore:
      - *list
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if !res.Unsafe {
		t.Error("Decode() Unsafe = false for a nested alias, want true (scan must recurse)")
	}
}

func TestDecodeMergeKeyInUnmodeledUnsafe(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    sched2:
      <<: {interval: weekly}
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if !res.Unsafe {
		t.Fatal("Decode() Unsafe = false for a merge key inside an unmodeled key, want true")
	}

	if len(res.UnsafeReasons) != 1 || !strings.Contains(res.UnsafeReasons[0], "merge key") {
		t.Errorf("Decode() UnsafeReasons = %v, want one merge-key reason", res.UnsafeReasons)
	}
}

// TestDecodeAnchorDefinitionInUnmodeledSafe pins that an anchor DEFINITION
// inside an unmodeled value is fine: it re-emits with its anchor intact,
// and nothing dangling can result (typed fields inline their values).
func TestDecodeAnchorDefinitionInUnmodeledSafe(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    x-common: &common
      a: 1
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if res.Unsafe {
		t.Errorf("Decode() Unsafe = true for an anchor definition, want false. Reasons: %v", res.UnsafeReasons)
	}
}

// directoriesYAML mirrors nixpkgs's real usage of the glob `directories`
// form (the schema alternative to `directory`).
const directoriesYAML = `version: 2
updates:
  - package-ecosystem: github-actions
    directories:
      - "/"
      - ".github/actions/*/*"
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
`

func TestDecodeDirectoriesEntry(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(directoriesYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if res.Unsafe {
		t.Errorf("Decode() Unsafe = true, want false. Reasons: %v", res.UnsafeReasons)
	}

	if got := res.Config.Updates[0].Directories; len(got) != 2 {
		t.Errorf("Decode() Directories = %v, want two patterns", got)
	}

	if err := res.Config.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil (directories satisfies the requirement)", err)
	}
}

// TestFindDirectoriesCoversRoot pins that a glob-form entry counts as the
// desired root entry, so neither Diff nor Reconcile duplicates it.
func TestFindDirectoriesCoversRoot(t *testing.T) {
	t.Parallel()

	cfg := mustConfig(t, directoriesYAML)

	if idx := cfg.Find(dependabot.EcosystemGitHubActions, "/"); idx != 0 {
		t.Errorf("Find(github-actions, /) = %d, want 0 (directories covers root)", idx)
	}

	desired, _ := dependabot.Generate(dependabot.RepoShape{HasGitHubActions: true})

	got := dependabot.Reconcile(cfg, desired)

	actions := 0

	for _, update := range got.Updates {
		if update.PackageEcosystem == dependabot.EcosystemGitHubActions {
			actions++
		}
	}

	if actions != 1 {
		t.Errorf("Reconcile() produced %d github-actions entries, want 1 (no duplicate coverage)", actions)
	}

	if got.Updates[0].Groups.Empty() {
		t.Error("Reconcile() did not fill groups on the directories entry")
	}
}

func TestValidateMissingDirectoryAndDirectories(t *testing.T) {
	t.Parallel()

	cfg := dependabot.Config{
		Version: dependabot.CurrentVersion,
		Updates: []dependabot.Update{{PackageEcosystem: dependabot.EcosystemGoModules}},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() = nil for an entry with neither directory nor directories, want invalid")
	}

	if !strings.Contains(err.Error(), "directory or directories") {
		t.Errorf("Validate() error = %v, want it to name both spellings", err)
	}
}

// TestFindBadGlobPatternCovers pins the conservative direction of
// unparseable glob patterns: assume coverage, because appending a duplicate
// entry for the same ecosystem breaks Dependabot while a missed fill is
// only an info-level gap.
func TestFindBadGlobPatternCovers(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directories:
      - "["
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if idx := res.Config.Find(dependabot.EcosystemGoModules, "/"); idx != 0 {
		t.Errorf("Find(gomod, /) = %d, want 0 (unparseable glob counts as covering)", idx)
	}
}

// TestDiffOrphanDetectionForDirectoriesGlobs pins orphan detection for
// glob-form entries: a pattern covering a detected directory is no orphan;
// one covering nothing reports the orphan info finding, like any
// user-managed entry.
func TestDiffOrphanDetectionForDirectoriesGlobs(t *testing.T) {
	t.Parallel()

	shape := dependabot.RepoShape{GoModuleDirs: []string{"", "cmd/tool"}}
	desired, _ := dependabot.Generate(shape)

	tests := []struct {
		name        string
		directories string
		wantOrphan  bool
	}{
		{name: "glob covering a detected module is no orphan", directories: `      - "/cmd/*"`, wantOrphan: false},
		{name: "glob covering nothing is an orphan", directories: `      - "/nothing/*"`, wantOrphan: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			yaml := strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directories:",
				tt.directories,
				"",
			}, "\n")

			dec, err := dependabot.Decode([]byte(yaml))
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			issues := dependabot.Diff(
				&dec.Config,
				dec,
				desired,
				shape,
				dependabot.CapInfo{},
				".github/dependabot.yml",
			)

			sawOrphan := false

			for _, issue := range issues {
				if issue.Rule == "dependabot-entry-orphan" {
					sawOrphan = true
				}
			}

			if sawOrphan != tt.wantOrphan {
				t.Errorf("Diff() orphan = %v, want %v (issues: %v)", sawOrphan, tt.wantOrphan, issues)
			}
		})
	}
}

package dependabot_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

// customYAML is a canonical grouped config carrying the modeled user
// customizations: PR labels and a schedule day. v0.2.0 moved both from
// "outside the known schema" (suggest-only, repair refuses to write) to
// modeled fields that Decode, Reconcile, and Encode preserve verbatim.
const customYAML = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
      day: monday
      time: "06:00"
      timezone: Europe/Berlin
    open-pull-requests-limit: 5
    labels:
      - dependencies
      - gomod
    groups:
      minor-and-patch:
        update-types:
          - minor
          - patch
`

func TestDecodeCustomizationsSafe(t *testing.T) {
	res, err := dependabot.Decode([]byte(customYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if res.Unsafe {
		t.Error("Decode() Unsafe = true for labels + schedule day/time/timezone, want modeled and safe")
	}

	if len(res.Config.Updates) != 1 {
		t.Fatalf("Decode() updates = %d, want 1", len(res.Config.Updates))
	}

	u := res.Config.Updates[0]
	if u.Schedule == nil || u.Schedule.Day != "monday" || u.Schedule.Time != "06:00" || u.Schedule.Timezone != "Europe/Berlin" {
		t.Errorf("Decode() schedule customizations not populated: %+v", u.Schedule)
	}

	if len(u.Labels) != 2 || u.Labels[0] != "dependencies" || u.Labels[1] != "gomod" {
		t.Errorf("Decode() labels = %v, want [dependencies gomod]", u.Labels)
	}
}

func TestDecodeUnknownScheduleKeyUnsafe(t *testing.T) {
	yaml := strings.Join([]string{
		"version: 2",
		"updates:",
		"  - package-ecosystem: gomod",
		"    directory: /",
		"    schedule:",
		"      interval: weekly",
		"      bogus: value",
		"",
	}, "\n")

	res, err := dependabot.Decode([]byte(yaml))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if !res.Unsafe {
		t.Error("Decode() Unsafe = false for unknown schedule key, want true (rewrite would silently drop it)")
	}
}

func TestEncodeRoundTripPreservesCustomizations(t *testing.T) {
	res, err := dependabot.Decode([]byte(customYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	encoded, err := res.Config.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	again, err := dependabot.Decode(encoded)
	if err != nil {
		t.Fatalf("re-Decode() error = %v", err)
	}

	if !dependabot.Equal(res.Config, again.Config) {
		t.Error("Encode/Decode round-trip lost customizations")
	}

	if again.Unsafe {
		t.Error("re-Decode() Unsafe = true, want false")
	}

	u := again.Config.Updates[0]
	if u.Schedule == nil || u.Schedule.Day != "monday" {
		t.Errorf("round-trip lost schedule day: %+v", u.Schedule)
	}

	if len(u.Labels) != 2 {
		t.Errorf("round-trip lost labels: %v", u.Labels)
	}
}

func TestReconcilePreservesCustomizations(t *testing.T) {
	existing := mustConfig(t, customYAML)
	desired, _ := dependabot.Generate(dependabot.RepoShape{GoModuleDirs: []string{""}, HasGitHubActions: true})

	got := dependabot.Reconcile(existing, desired)

	root := got.Updates[got.Find(dependabot.EcosystemGoModules, "/")]
	if root.Schedule == nil || root.Schedule.Day != "monday" || root.Schedule.Time != "06:00" || root.Schedule.Timezone != "Europe/Berlin" {
		t.Errorf("Reconcile() lost schedule customizations: %+v", root.Schedule)
	}

	if len(root.Labels) != 2 || root.Labels[0] != "dependencies" {
		t.Errorf("Reconcile() lost labels: %v", root.Labels)
	}

	if root.Groups.Empty() {
		t.Error("Reconcile() groups empty, want canonical groups preserved")
	}

	if got.Find(dependabot.EcosystemGitHubActions, "/") < 0 {
		t.Error("Reconcile() missing appended github-actions entry")
	}

	appended := got.Updates[got.Find(dependabot.EcosystemGitHubActions, "/")]
	if appended.Schedule != nil && appended.Schedule.Day != "" {
		t.Errorf("Reconcile() generated a schedule day (%q), want none — customizations are never generated", appended.Schedule.Day)
	}

	if len(appended.Labels) != 0 {
		t.Errorf("Reconcile() generated labels (%v), want none — customizations are never generated", appended.Labels)
	}
}

func TestDiffSilentOnCustomizations(t *testing.T) {
	existing := mustConfig(t, customYAML)
	desired, _ := dependabot.Generate(dependabot.RepoShape{GoModuleDirs: []string{""}})

	dec, err := dependabot.Decode([]byte(customYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	issues := dependabot.Diff(&existing, dec, desired, dependabot.RepoShape{GoModuleDirs: []string{""}}, dependabot.CapInfo{}, ".github/dependabot.yml")
	if len(issues) != 0 {
		t.Errorf("Diff() = %v, want no issues for canonical config carrying customizations", issues)
	}
}

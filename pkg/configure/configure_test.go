package configure_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

const canonicalSelfConfig = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      minor-and-patch:
        update-types:
          - minor
          - patch
  - package-ecosystem: gomod
    directory: /modules/types
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      minor-and-patch:
        update-types:
          - minor
          - patch
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      actions:
        patterns:
          - "*"
`

func repoWithConfig(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "modules", "types"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(root, "modules", "types", "go.mod"),
		[]byte("module example.com/x/types\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(root, ".github", "workflows", "ci.yml"),
		[]byte("on: push\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if content != "" {
		abs := filepath.Join(root, configure.DefaultConfigPath)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func run(t *testing.T, root string, opts configure.Options) configure.Result {
	t.Helper()

	opts.Root = root

	result, err := configure.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	return result
}

func TestRunGeneratesMissingConfig(t *testing.T) {
	t.Parallel()
	root := repoWithConfig(t, "")

	result := run(t, root, configure.Options{})

	if !result.Wrote {
		t.Fatal("Run() did not write config")
	}

	if len(result.Findings) == 0 || result.Findings[0].Rule != "dependabot-config-missing" {
		t.Errorf("Run() findings = %v, want dependabot-config-missing first", result.Findings)
	}

	got, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	wantRoot := "directory: /"
	if !strings.Contains(string(got), wantRoot) || !strings.Contains(string(got), "/modules/types") {
		t.Errorf("generated config missing module entries:\n%s", got)
	}

	if !strings.Contains(string(got), "github-actions") {
		t.Errorf("generated config missing actions entry:\n%s", got)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	t.Parallel()
	root := repoWithConfig(t, "")

	run(t, root, configure.Options{})

	first, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	second := run(t, root, configure.Options{})
	if !second.Unchanged || second.Wrote {
		t.Errorf("second Run() = wrote=%v unchanged=%v, want idempotent no-write", second.Wrote, second.Unchanged)
	}

	secondContent, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if string(first) != string(secondContent) {
		t.Errorf("second Run() changed the file:\nfirst:\n%s\nsecond:\n%s", first, secondContent)
	}
}

func TestRunCheckDoesNotWrite(t *testing.T) {
	t.Parallel()
	root := repoWithConfig(t, "")

	result := run(t, root, configure.Options{Check: true})

	if result.Wrote || !result.PlannedWrite {
		t.Errorf("Run(Check) = wrote=%v planned=%v, want held back", result.Wrote, result.PlannedWrite)
	}

	if !result.ChangesNeeded() {
		t.Error("Run(Check) ChangesNeeded() = false, want true")
	}

	if _, err := os.Stat(filepath.Join(root, ".github", "dependabot.yml")); !os.IsNotExist(err) {
		t.Error("Run(Check) wrote the config file anyway")
	}
}

func TestRunUnsafeConfigIsSuggestOnly(t *testing.T) {
	t.Parallel()

	unsafe := "version: 2\nregistries:\n  npm: {}\nupdates: []\n"
	root := repoWithConfig(t, unsafe)

	result := run(t, root, configure.Options{})

	if !result.UnsafeRepair {
		t.Fatal("Run() UnsafeRepair = false, want true for unknown top-level keys")
	}

	if result.Wrote {
		t.Error("Run() rewrote an unsafe config")
	}

	got, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != unsafe {
		t.Error("Run() modified an unsafe config file")
	}
}

func TestRunReconcilesBareConfigPreservingIntent(t *testing.T) {
	t.Parallel()

	bare := `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: monthly
`
	root := repoWithConfig(t, bare)

	result := run(t, root, configure.Options{})

	if !result.Wrote {
		t.Fatal("Run() did not repair bare config")
	}

	got, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(got)
	if !strings.Contains(content, "interval: monthly") {
		t.Errorf("repair dropped the user's monthly schedule:\n%s", content)
	}

	if !strings.Contains(content, "open-pull-requests-limit: 5") {
		t.Errorf("repair did not fill the missing limit:\n%s", content)
	}

	if !strings.Contains(content, "/modules/types") {
		t.Errorf("repair did not append the missing module entry:\n%s", content)
	}
}

func TestRunCanonicalConfigIsNoOp(t *testing.T) {
	t.Parallel()
	root := repoWithConfig(t, canonicalSelfConfig)

	result := run(t, root, configure.Options{})

	if len(result.Findings) != 0 {
		t.Errorf("Run() on canonical config findings = %v, want none", result.Findings)
	}

	second := run(t, root, configure.Options{})
	if second.Wrote || second.PlannedWrite || !second.Unchanged {
		t.Errorf("second Run() = %+v, want clean no-op after one-time normalization", second)
	}
}

func TestRunOnEmptyRepoIsNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	result := run(t, root, configure.Options{})

	if result.Wrote || len(result.Findings) != 0 || !result.Unchanged {
		t.Errorf("Run() on empty repo = %+v, want no findings and no write", result)
	}
}

func TestRunUnparseableConfigIsSuggestOnly(t *testing.T) {
	t.Parallel()

	seqGroups := `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      - everything:
          patterns:
            - "*"
`
	root := repoWithConfig(t, seqGroups)

	result := run(t, root, configure.Options{})

	if !result.UnsafeRepair || result.Wrote {
		t.Errorf(
			"Run() on unparseable config = unsafe=%v wrote=%v, want suggest-only",
			result.UnsafeRepair,
			result.Wrote,
		)
	}

	if len(result.Findings) == 0 || result.Findings[0].Rule != "dependabot-config-unparseable" {
		t.Errorf("Run() findings = %v, want dependabot-config-unparseable", result.Findings)
	}
}

func TestRunInvalidEntryIsSuggestOnly(t *testing.T) {
	t.Parallel()

	invalid := "version: 2\nupdates:\n  - package-ecosystem: gomod\n"
	root := repoWithConfig(t, invalid)

	result := run(t, root, configure.Options{})

	if !result.UnsafeRepair || result.Wrote {
		t.Fatalf("Run() on invalid entry = unsafe=%v wrote=%v, want suggest-only", result.UnsafeRepair, result.Wrote)
	}

	got, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != invalid {
		t.Error("Run() modified a config with an invalid entry")
	}

	found := false

	for _, f := range result.Findings {
		if string(f.Rule) == "dependabot-entry-invalid" {
			found = true
		}
	}

	if !found {
		t.Errorf("Run() findings = %v, want a dependabot-entry-invalid finding", result.Findings)
	}
}

func TestRunFillsEmptyScheduleInterval(t *testing.T) {
	t.Parallel()

	emptyInterval := "version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n    schedule: {}\n"
	root := repoWithConfig(t, emptyInterval)

	result := run(t, root, configure.Options{})

	if !result.Wrote {
		t.Fatal("Run() did not fill the empty schedule interval")
	}

	got, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(got), "interval: weekly") {
		t.Errorf("repaired config missing weekly interval:\n%s", got)
	}
}

// TestRunRepairWritesBackAllCustomizations is the end-to-end adversarial
// preservation test: the idempotence suite cannot see a lossy FIRST write
// (the v0.2.0 schedule-fill bug shipped that way). A config carrying every
// modeled customization plus an orphan entry is repaired, and the file that
// lands on disk must still carry all of it.
func TestRunRepairWritesBackAllCustomizations(t *testing.T) {
	t.Parallel()

	customized := `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: monthly
      day: monday
      time: "03:00"
      timezone: Europe/Berlin
    open-pull-requests-limit: 3
    labels:
      - dependencies
  - package-ecosystem: npm
    directory: /
    schedule:
      interval: monthly
`
	root := repoWithConfig(t, customized)

	result := run(t, root, configure.Options{})

	if !result.Wrote {
		t.Fatal("Run() did not repair the customized config")
	}

	raw, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatal(err)
	}

	res, err := dependabot.Decode(raw)
	if err != nil {
		t.Fatalf("repaired config does not decode: %v", err)
	}

	if res.Unsafe {
		t.Fatalf("repaired config decodes unsafe: %+v", res)
	}

	idx := res.Config.Find(dependabot.EcosystemGoModules, "/")
	if idx < 0 {
		t.Fatal("repaired config lost the gomod entry")
	}

	entry := res.Config.Updates[idx]

	if entry.Schedule == nil {
		t.Fatal("repaired gomod entry lost its schedule")
	}

	sched := entry.Schedule

	if sched.Interval != "monthly" || sched.Day != "monday" || sched.Time != "03:00" ||
		sched.Timezone != "Europe/Berlin" {
		t.Fatalf("repaired gomod schedule = %+v, want monthly/monday/03:00/Europe/Berlin", sched)
	}

	if entry.OpenPullRequestsLimit != 3 {
		t.Fatalf("repaired gomod limit = %d, want 3", entry.OpenPullRequestsLimit)
	}

	if !slices.Equal(entry.Labels, []string{"dependencies"}) {
		t.Fatalf("repaired gomod labels = %v, want [dependencies]", entry.Labels)
	}

	if res.Config.Find(dependabot.EcosystemNPM, "/") < 0 {
		t.Fatal("repaired config lost the orphan npm entry")
	}
}

// TestMarshalJSONResultPinsWireShape documents the --json contract at the
// library boundary: snake_case keys are a released contract (the sweep
// script consumes them), so this table is intentionally excluded from the
// tagliatelle camelCase rule in .golangci.yml.
func TestMarshalJSONResultPinsWireShape(t *testing.T) {
	t.Parallel()

	root := repoWithConfig(t, "version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n")

	result := run(t, root, configure.Options{Check: true})

	data, err := configure.MarshalJSONResult(result)
	if err != nil {
		t.Fatalf("MarshalJSONResult() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, data)
	}

	for key, want := range map[string]bool{"wrote": false, "planned_write": true, "unsafe_repair": false} {
		got, ok := payload[key].(bool)
		if !ok || got != want {
			t.Errorf("JSON %s = %#v, want %v", key, payload[key], want)
		}
	}

	findings, ok := payload["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("JSON findings = %#v, want a non-empty array", payload["findings"])
	}

	first, ok := findings[0].(map[string]any)
	if !ok {
		t.Fatalf("finding JSON = %#v, want an object", findings[0])
	}

	for _, key := range []string{"rule", "message", "severity", "file"} {
		if _, present := first[key]; !present {
			t.Errorf("finding JSON missing %q key", key)
		}
	}

	if _, present := payload["security_fixes"]; present {
		t.Error("JSON output carries security_fixes, want it omitted when empty")
	}
}

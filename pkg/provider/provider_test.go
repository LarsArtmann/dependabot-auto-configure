package provider_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	"github.com/larsartmann/dependabot-auto-configure/pkg/provider"
	"github.com/larsartmann/go-finding"
	toolsdk "github.com/larsartmann/go-finding/toolsdk"
)

const unmodeledSelfConfig = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    unknown-entry-field: keep-me
registries:
  - type: npm-registry
    url: https://registry.example.com
`

// listFormSelfConfig carries the one remaining unsafe shape: a
// non-mapping `groups:` value, which no rewrite can carry verbatim.
const listFormSelfConfig = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    groups:
      - minor-and-patch
`

func writeGoMod(t *testing.T, dir string) {
	t.Helper()

	if err := os.WriteFile(
		filepath.Join(dir, "go.mod"),
		[]byte("module example.com/fixtures\n\ngo 1.27\n"),
		0o644,
	); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}

func newGoModuleFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeGoMod(t, root)

	return root
}

func detectWithRoot(t *testing.T, root string) ([]finding.Finding, error) {
	t.Helper()

	return provider.Provider.Detect.Detect(finding.WithWorkingDir(context.Background(), root))
}

func repairWithRoot(t *testing.T, root string, dryRun bool) (toolsdk.RepairResult, error) {
	t.Helper()

	ctx := toolsdk.WithDryRun(finding.WithWorkingDir(context.Background(), root), dryRun)

	return provider.Provider.Repair.Repair(ctx)
}

func configBytes(t *testing.T, root string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, configure.DefaultConfigPath))
	if err != nil {
		t.Fatalf("read %s: %v", configure.DefaultConfigPath, err)
	}

	return string(data)
}

func TestProviderSpecContract(t *testing.T) {
	t.Parallel()

	if provider.Provider.Name != configure.ToolName {
		t.Errorf("Name = %q, want %q", provider.Provider.Name, configure.ToolName)
	}

	if strings.TrimSpace(provider.Provider.Description) == "" {
		t.Error("Description is empty")
	}

	if provider.Provider.Detect == nil {
		t.Error("Detect capability missing")
	}

	if provider.Provider.Repair == nil {
		t.Error("Repair capability missing")
	}

	wantInput := ".github/dependabot.yml"
	found := false

	for _, input := range provider.Provider.Inputs {
		if input == wantInput {
			found = true
		}
	}

	if !found {
		t.Errorf("Inputs missing %q (got %v)", wantInput, provider.Provider.Inputs)
	}
}

func TestProviderDiscoverableViaAll(t *testing.T) {
	t.Parallel()

	for _, spec := range toolsdk.All() {
		if spec.Name == configure.ToolName {
			return
		}
	}

	t.Errorf("toolsdk.All() does not contain %q; BuildFlow blank-import discovery would fail", configure.ToolName)
}

func TestDetectReportsMissingConfigWithoutWriting(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	findings, err := detectWithRoot(t, root)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	var sawMissing bool

	for _, f := range findings {
		if f.Rule == "dependabot-config-missing" {
			sawMissing = true
		}
	}

	if !sawMissing {
		t.Fatalf("Detect found no dependabot-config-missing rule; findings: %+v", findings)
	}

	if _, err := os.Stat(filepath.Join(root, configure.DefaultConfigPath)); !os.IsNotExist(err) {
		t.Errorf("Detect wrote %s (check-mode must never write)", configure.DefaultConfigPath)
	}
}

func TestDetectErrorOnUnavailableRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "does-not-exist")

	if _, err := detectWithRoot(t, root); err == nil {
		t.Fatal("Detect on a nonexistent root succeeded; want an infrastructure error")
	}
}

func TestRepairWritesCanonicalConfig(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	result, err := repairWithRoot(t, root, false)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}

	if !strings.Contains(result.Description, "wrote") {
		t.Errorf("Description = %q, want it to report the write", result.Description)
	}

	configBytes(t, root)
}

func TestRepairDryRunHoldsBack(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	result, err := repairWithRoot(t, root, true)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}

	if !strings.Contains(result.Description, "held back by dry-run") {
		t.Errorf("Description = %q, want the dry-run hold-back", result.Description)
	}

	if _, err := os.Stat(filepath.Join(root, configure.DefaultConfigPath)); !os.IsNotExist(err) {
		t.Errorf("dry-run Repair wrote %s", configure.DefaultConfigPath)
	}
}

func TestRepairIsIdempotentOnCanonicalConfig(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	if _, err := repairWithRoot(t, root, false); err != nil {
		t.Fatalf("first Repair: %v", err)
	}

	result, err := repairWithRoot(t, root, false)
	if err != nil {
		t.Fatalf("second Repair: %v", err)
	}

	if !strings.Contains(result.Description, "already canonical") {
		t.Errorf("Description = %q, want the canonical no-op", result.Description)
	}
}

// TestRepairCarriesUnmodeledConstructsThrough pins the v0.4.0 passthrough
// at the BuildFlow boundary: unmodeled schema (unknown entry fields,
// top-level registries) no longer blocks repair — the write lands, the
// constructs survive, and the second repair is the canonical no-op.
func TestRepairCarriesUnmodeledConstructsThrough(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	path := filepath.Join(root, configure.DefaultConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create .github dir: %v", err)
	}

	if err := os.WriteFile(path, []byte(unmodeledSelfConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := repairWithRoot(t, root, false)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}

	if !strings.Contains(result.Description, "wrote grouped weekly") {
		t.Errorf("Description = %q, want the write-through", result.Description)
	}

	after := configBytes(t, root)
	for _, want := range []string{"keep-me", "registries:", "npm-registry", "groups:"} {
		if !strings.Contains(after, want) {
			t.Errorf("repaired config lost %q:\n%s", want, after)
		}
	}

	second, err := repairWithRoot(t, root, false)
	if err != nil {
		t.Fatalf("second Repair: %v", err)
	}

	if !strings.Contains(second.Description, "already canonical") {
		t.Errorf("second Description = %q, want the canonical no-op", second.Description)
	}
}

// TestRepairListFormGroupsIsSuggestOnly pins the remaining unsafe shape at
// the BuildFlow boundary: a non-mapping `groups:` value cannot be carried
// verbatim, so repair must hold back.
func TestRepairListFormGroupsIsSuggestOnly(t *testing.T) {
	t.Parallel()

	root := newGoModuleFixture(t)

	path := filepath.Join(root, configure.DefaultConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create .github dir: %v", err)
	}

	if err := os.WriteFile(path, []byte(listFormSelfConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	before := configBytes(t, root)

	result, err := repairWithRoot(t, root, false)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}

	if !strings.Contains(result.Description, "suggest-only") {
		t.Errorf("Description = %q, want the suggest-only notice", result.Description)
	}

	after := configBytes(t, root)

	if before != after {
		t.Error("Repair rewrote an unsafe config; it must stay suggest-only")
	}
}

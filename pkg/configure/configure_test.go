package configure_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
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

func repoWithConfig(t *testing.T, configPath, content string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "modules", "types"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "modules", "types", "go.mod"), []byte("module example.com/x/types\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "ci.yml"), []byte("on: push\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if content != "" {
		abs := filepath.Join(root, filepath.FromSlash(configPath))
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
	root := repoWithConfig(t, configure.DefaultConfigPath, "")

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
	root := repoWithConfig(t, configure.DefaultConfigPath, "")

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
	root := repoWithConfig(t, configure.DefaultConfigPath, "")

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
	unsafe := "version: 2\nregistries:\n  npm: {}\nupdates: []\n"
	root := repoWithConfig(t, configure.DefaultConfigPath, unsafe)

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
	bare := "version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n    schedule:\n      interval: monthly\n"
	root := repoWithConfig(t, configure.DefaultConfigPath, bare)

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
	root := repoWithConfig(t, configure.DefaultConfigPath, canonicalSelfConfig)

	result := run(t, root, configure.Options{})

	if result.Wrote || result.PlannedWrite || len(result.Findings) != 0 || !result.Unchanged {
		t.Errorf("Run() on canonical config = %+v, want clean no-op", result)
	}
}

func TestRunOnEmptyRepoIsNoOp(t *testing.T) {
	root := t.TempDir()

	result := run(t, root, configure.Options{})

	if result.Wrote || len(result.Findings) != 0 || !result.Unchanged {
		t.Errorf("Run() on empty repo = %+v, want no findings and no write", result)
	}
}

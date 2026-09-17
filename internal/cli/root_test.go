package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/internal/cli"
)

// bareWarningConfig decodes safely and yields only warning findings: its
// single entry has no schedule, limit, or groups.
const bareWarningConfig = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
`

func writeGoMod(t *testing.T, root string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()

	path := filepath.Join(root, ".github", "dependabot.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runCommand invokes the root command in-process and returns the exit code
// the process would carry.
func runCommand(t *testing.T, args ...string) int {
	t.Helper()

	cmd, code := cli.NewRootCmdForTest()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	return *code
}

func TestCheckExitCodes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runCommand(t, "--check", "--root", root); code != 1 {
		t.Errorf("check on missing config: exit = %d, want 1", code)
	}

	if code := runCommand(t, "--root", root); code != 0 {
		t.Errorf("repair run: exit = %d, want 0", code)
	}

	if code := runCommand(t, "--check", "--root", root); code != 0 {
		t.Errorf("check on canonical config: exit = %d, want 0", code)
	}

	if _, err := os.Stat(filepath.Join(root, ".github", "dependabot.yml")); err != nil {
		t.Error("repair run did not produce the config file")
	}
}

func TestCheckNeverWrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runCommand(t, "--check", "--root", root); code != 1 {
		t.Errorf("check: exit = %d, want 1", code)
	}

	if _, err := os.Stat(filepath.Join(root, ".github", "dependabot.yml")); !os.IsNotExist(err) {
		t.Error("check wrote the config file")
	}
}

func TestErrorOnMissingRoot(t *testing.T) {
	t.Parallel()

	cmd, code := cli.NewRootCmdForTest()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--check", "--root", filepath.Join(t.TempDir(), "missing")})

	if err := cmd.Execute(); err == nil {
		t.Error("expected error for missing root, got nil")
	} else if *code != 0 {
		t.Logf("code pointer = %d (Execute maps errors to 2)", *code)
	}
}

func TestFailOnSeverityThresholds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeGoMod(t, root)
	writeConfig(t, root, bareWarningConfig)

	if code := runCommand(t, "--check", "--root", root, "--fail-on", "error"); code != 0 {
		t.Errorf("warning-only findings with --fail-on error: exit = %d, want 0", code)
	}

	if code := runCommand(t, "--check", "--root", root, "--fail-on", "warning"); code != 1 {
		t.Errorf("warning-only findings with --fail-on warning: exit = %d, want 1", code)
	}

	if code := runCommand(t, "--check", "--root", root, "--fail-on", "none"); code != 0 {
		t.Errorf("--fail-on none must never fail on findings: exit = %d, want 0", code)
	}

	if code := runCommand(t, "--check", "--root", root); code != 1 {
		t.Errorf("default policy (any pending change): exit = %d, want 1", code)
	}

	if code := runCommand(t, "--check", "--root", root, "--fail-on", "any"); code != 1 {
		t.Errorf("explicit any policy: exit = %d, want 1", code)
	}
}

func TestFailOnSeverityFailsOnErrorFinding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeGoMod(t, root)

	if code := runCommand(t, "--check", "--root", root, "--fail-on", "error"); code != 1 {
		t.Errorf("error-severity finding with --fail-on error: exit = %d, want 1", code)
	}
}

func TestFailOnRejectsUnknownSeverity(t *testing.T) {
	t.Parallel()

	cmd, _ := cli.NewRootCmdForTest()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--check", "--root", t.TempDir(), "--fail-on", "bogus"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want a FlagValueError")
	}

	flagErr, ok := errors.AsType[*cli.FlagValueError](err)
	if !ok {
		t.Fatalf("Execute() error = %T, want *cli.FlagValueError", err)
	}

	if flagErr.Flag != "--fail-on" || flagErr.Value != "bogus" {
		t.Errorf("FlagValueError = %+v, want flag --fail-on value bogus", flagErr)
	}
}

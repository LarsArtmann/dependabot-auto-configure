package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/internal/cli"
)

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

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestJSONOutputPinsWireShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd, code := cli.NewRootCmdForTest()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--check", "--json", "--root", root})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command execution failed: %v", err)
	}

	if *code != 1 {
		t.Fatalf("check --json: exit = %d, want 1", *code)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, out.String())
	}

	findings, ok := payload["findings"].([]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("JSON findings = %#v, want a non-empty array", payload["findings"])
	}

	if first, ok := findings[0].(map[string]any); ok {
		for _, key := range []string{"rule", "message", "severity", "file"} {
			if _, present := first[key]; !present {
				t.Errorf("finding JSON missing %q key", key)
			}
		}
	} else {
		t.Errorf("finding JSON = %#v, want an object", findings[0])
	}

	for key, want := range map[string]bool{"wrote": false, "planned_write": true, "unsafe_repair": false} {
		got, ok := payload[key].(bool)
		if !ok || got != want {
			t.Errorf("JSON %s = %#v, want %v", key, payload[key], want)
		}
	}

	if _, present := payload["security_fixes"]; present {
		t.Error("JSON output carries security_fixes, want it omitted when empty")
	}
}

type failingWriter struct{}

var errDiskFull = errors.New("disk full")

func (failingWriter) Write([]byte) (int, error) { return 0, errDiskFull }

func TestReportWriteFailureSurfaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd, _ := cli.NewRootCmdForTest()
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--root", root})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an OutputError when stdout fails")
	}

	outErr, ok := errors.AsType[*cli.OutputError](err)
	if !ok {
		t.Fatalf("Execute() error = %T, want *cli.OutputError", err)
	}

	if outErr.Stream != "stdout" {
		t.Errorf("OutputError.Stream = %q, want stdout", outErr.Stream)
	}
}

//nolint:paralleltest // t.Chdir mutates process state, so it cannot run in parallel
func TestExecuteRunsAgainstWorkingDirectory(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(root)

	if code := cli.Execute(context.Background()); code != 0 {
		t.Errorf("Execute() = %d, want 0 on an empty repository", code)
	}

	if _, err := os.Stat(filepath.Join(root, ".github", "dependabot.yml")); err != nil {
		t.Errorf("Execute() did not generate the config: %v", err)
	}
}

package configure_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
)

// TestRunWriteFailureIsReported exercises the atomic-write failure branch:
// a repository whose .github directory is read-only must surface a typed
// write error instead of a silent no-op, and must leave no file behind.
func TestRunWriteFailureIsReported(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions; the failure path cannot be exercised")
	}

	root := t.TempDir()
	writeGoModFile(t, root)

	githubDir := filepath.Join(root, ".github")
	if err := os.MkdirAll(githubDir, 0o500); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.Chmod(githubDir, 0o700); err != nil {
			t.Logf("restore .github permissions: %v", err)
		}
	})

	_, err := configure.Run(context.Background(), configure.Options{Root: root})
	if err == nil {
		t.Fatal("Run() error = nil, want the typed write failure")
	}

	writeErr, ok := errors.AsType[*configure.ConfigWriteError](err)
	if !ok {
		t.Fatalf("Run() error = %T, want *configure.ConfigWriteError", err)
	}

	if writeErr.Step != configure.WriteStepWrite {
		t.Errorf("step = %q, want %q", writeErr.Step, configure.WriteStepWrite)
	}

	if _, statErr := os.Stat(filepath.Join(root, configure.DefaultConfigPath)); !os.IsNotExist(statErr) {
		t.Error("a failed write left a config file behind")
	}
}

func writeGoModFile(t *testing.T, root string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

package detect_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/detect"
)

func writeTree(t *testing.T, files ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, f := range files {
		abs := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(abs), err)
		}

		if err := os.WriteFile(abs, []byte("placeholder\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", abs, err)
		}
	}

	return root
}

func TestShape(t *testing.T) {
	tests := []struct {
		name        string
		files       []string
		wantModules []string
		wantActions bool
		wantNPM     bool
	}{
		{
			name:        "root module, actions, npm",
			files:       []string{"go.mod", ".github/workflows/ci.yml", "package.json"},
			wantModules: []string{""},
			wantActions: true,
			wantNPM:     true,
		},
		{
			name:        "multi-module with yaml workflow",
			files:       []string{"go.mod", "modules/types/go.mod", ".github/workflows/release.yaml"},
			wantModules: []string{"", "modules/types"},
			wantActions: true,
		},
		{
			name:        "testdata and vendor modules are skipped",
			files:       []string{"go.mod", "testdata/fixture/go.mod", "vendor/example.com/x/go.mod", "node_modules/pkg/go.mod"},
			wantModules: []string{""},
		},
		{
			name:        "hidden dirs are skipped except .github",
			files:       []string{"go.mod", ".cache/modules/x/go.mod", ".github/workflows/ci.yml"},
			wantModules: []string{""},
			wantActions: true,
		},
		{
			name:        "nested package.json is not npm",
			files:       []string{"go.mod", "web/package.json"},
			wantModules: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeTree(t, tt.files...)

			shape, err := detect.NewDetector(root).Shape()
			if err != nil {
				t.Fatalf("Shape() error = %v", err)
			}

			if len(shape.GoModuleDirs) != len(tt.wantModules) {
				t.Fatalf("Shape() modules = %v, want %v", shape.GoModuleDirs, tt.wantModules)
			}

			for i, want := range tt.wantModules {
				if shape.GoModuleDirs[i] != want {
					t.Errorf("Shape() module %d = %q, want %q", i, shape.GoModuleDirs[i], want)
				}
			}

			if shape.HasGitHubActions != tt.wantActions {
				t.Errorf("Shape() actions = %v, want %v", shape.HasGitHubActions, tt.wantActions)
			}

			if shape.HasNPM != tt.wantNPM {
				t.Errorf("Shape() npm = %v, want %v", shape.HasNPM, tt.wantNPM)
			}
		})
	}
}

func TestShapeMissingRoot(t *testing.T) {
	if _, err := detect.NewDetector(filepath.Join(t.TempDir(), "does-not-exist")).Shape(); err == nil {
		t.Fatal("Shape() on missing root expected error, got nil")
	}
}

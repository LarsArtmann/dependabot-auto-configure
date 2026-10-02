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
	t.Parallel()

	tests := []struct {
		name        string
		files       []string
		wantModules []string
		wantActions bool
		wantNPM     bool
	}{
		// wantNPM asserts a root npm entry only (len(NPMDirs) checks below cover members).
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
			name: "testdata and vendor modules are skipped",
			files: []string{
				"go.mod",
				"testdata/fixture/go.mod",
				"vendor/example.com/x/go.mod",
				"node_modules/pkg/go.mod",
			},
			wantModules: []string{""},
		},
		{
			name:        "hidden dirs are skipped except .github",
			files:       []string{"go.mod", ".cache/modules/x/go.mod", ".github/workflows/ci.yml"},
			wantModules: []string{""},
			wantActions: true,
		},
		{
			name:        "nested package.json without workspaces or lockfile is not npm",
			files:       []string{"go.mod", "web/package.json"},
			wantModules: []string{""},
		},
	}

	t.Run("workspace members are npm", func(t *testing.T) {
		t.Parallel()
		root := writeTree(t,
			"package.json",
			"apps/web/package.json",
			"packages/lib/package.json",
		)
		if err := os.WriteFile(
			filepath.Join(root, "package.json"),
			[]byte(`{"workspaces": ["apps/*", "packages/*"]}`),
			0o644,
		); err != nil {
			t.Fatal(err)
		}

		shape, err := detect.NewDetector(root).Shape()
		if err != nil {
			t.Fatalf("Shape() error = %v", err)
		}

		want := []string{"", "apps/web", "packages/lib"}
		if len(shape.NPMDirs) != len(want) {
			t.Fatalf("Shape() npm dirs = %v, want %v", shape.NPMDirs, want)
		}
		for i, dir := range want {
			if shape.NPMDirs[i] != dir {
				t.Errorf("Shape() npm dir %d = %q, want %q", i, shape.NPMDirs[i], dir)
			}
		}
	})

	t.Run("nested package.json with lockfile is npm", func(t *testing.T) {
		t.Parallel()
		shape, err := detect.NewDetector(writeTree(t,
			"package.json",
			"web/package.json",
			"web/package-lock.json",
		)).Shape()
		if err != nil {
			t.Fatalf("Shape() error = %v", err)
		}

		if len(shape.NPMDirs) != 2 || shape.NPMDirs[1] != "web" {
			t.Errorf("Shape() npm dirs = %v, want [\"\" \"web\"]", shape.NPMDirs)
		}
	})

	t.Run("pip cargo gradle manifests", func(t *testing.T) {
		t.Parallel()
		shape, err := detect.NewDetector(writeTree(t,
			"requirements.txt",
			"tools/Cargo.toml",
			"android/build.gradle.kts",
		)).Shape()
		if err != nil {
			t.Fatalf("Shape() error = %v", err)
		}

		if len(shape.PipDirs) != 1 || shape.PipDirs[0] != "" {
			t.Errorf("Shape() pip dirs = %v, want [\"\"]", shape.PipDirs)
		}

		if len(shape.CargoDirs) != 1 || shape.CargoDirs[0] != "tools" {
			t.Errorf("Shape() cargo dirs = %v, want [\"tools\"]", shape.CargoDirs)
		}

		if len(shape.GradleDirs) != 1 || shape.GradleDirs[0] != "android" {
			t.Errorf("Shape() gradle dirs = %v, want [\"android\"]", shape.GradleDirs)
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

			rootNPM := len(shape.NPMDirs) > 0 && shape.NPMDirs[0] == ""
			if rootNPM != tt.wantNPM {
				t.Errorf("Shape() npm dirs = %v, want root entry %v", shape.NPMDirs, tt.wantNPM)
			}
		})
	}
}

func TestShapeMissingRoot(t *testing.T) {
	t.Parallel()

	if _, err := detect.NewDetector(filepath.Join(t.TempDir(), "does-not-exist")).Shape(); err == nil {
		t.Fatal("Shape() on missing root expected error, got nil")
	}
}

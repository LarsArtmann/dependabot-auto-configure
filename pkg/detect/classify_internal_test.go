package detect

import "testing"

func TestClassifyUsesSlashSeparatedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rel  string
		want func(walkResult) bool
	}{
		{
			name: "root module",
			rel:  "go.mod",
			want: func(w walkResult) bool { return len(w.goModuleDirs) == 1 && w.goModuleDirs[0] == "" },
		},
		{
			name: "nested module",
			rel:  "modules/types/go.mod",
			want: func(w walkResult) bool {
				return len(w.goModuleDirs) == 1 && w.goModuleDirs[0] == "modules/types"
			},
		},
		{
			name: "workflow yaml",
			rel:  ".github/workflows/ci.yml",
			want: func(w walkResult) bool { return w.hasGitHubActions },
		},
		{
			name: "workflow yaml plural path",
			rel:  ".github/workflows/deploy.yaml",
			want: func(w walkResult) bool { return w.hasGitHubActions },
		},
		{
			name: "workflows file outside .github is not an action",
			rel:  "workflows/ci.yml",
			want: func(w walkResult) bool { return !w.hasGitHubActions },
		},
		{
			// package.json is recorded by classifyPackageJSON (it needs the
			// file content), not by the pure name-based classify.
			name: "package json is left to classifyPackageJSON",
			rel:  "package.json",
			want: func(w walkResult) bool { return len(w.packageJSONDirs) == 0 },
		},
		{
			name: "npm lockfile marks its directory",
			rel:  "web/pnpm-lock.yaml",
			want: func(w walkResult) bool { return w.lockfileDirs["web"] },
		},
		{
			name: "pip manifest",
			rel:  "requirements.txt",
			want: func(w walkResult) bool { return len(w.pipDirs) == 1 && w.pipDirs[0] == "" },
		},
		{
			name: "cargo manifest",
			rel:  "crates/x/Cargo.toml",
			want: func(w walkResult) bool { return len(w.cargoDirs) == 1 && w.cargoDirs[0] == "crates/x" },
		},
		{
			name: "gradle kts manifest",
			rel:  "android/settings.gradle.kts",
			want: func(w walkResult) bool { return len(w.gradleDirs) == 1 && w.gradleDirs[0] == "android" },
		},
		{
			name: "unrelated file",
			rel:  "docs/nested/go.mod.txt",
			want: func(w walkResult) bool {
				return len(w.goModuleDirs) == 0 && !w.hasGitHubActions &&
					len(w.pipDirs) == 0 &&
					len(w.cargoDirs) == 0 && len(w.gradleDirs) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := &walkResult{lockfileDirs: map[string]bool{}}
			classify(tt.rel, w)

			if !tt.want(*w) {
				t.Errorf("classify(%q) produced %+v, want it to satisfy the expectation", tt.rel, *w)
			}
		})
	}
}

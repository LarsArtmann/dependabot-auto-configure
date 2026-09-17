package detect

import (
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

func TestClassifyUsesSlashSeparatedPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rel        string
		want       func(dependabot.RepoShape) bool
		wantModule string
	}{
		{
			name:       "root module",
			rel:        "go.mod",
			want:       func(s dependabot.RepoShape) bool { return len(s.GoModuleDirs) == 1 && s.GoModuleDirs[0] == "" },
			wantModule: "",
		},
		{
			name: "nested module",
			rel:  "modules/types/go.mod",
			want: func(s dependabot.RepoShape) bool {
				return len(s.GoModuleDirs) == 1 && s.GoModuleDirs[0] == "modules/types"
			},
			wantModule: "modules/types",
		},
		{
			name: "workflow yaml",
			rel:  ".github/workflows/ci.yml",
			want: func(s dependabot.RepoShape) bool { return s.HasGitHubActions },
		},
		{
			name: "workflow yaml plural path",
			rel:  ".github/workflows/deploy.yaml",
			want: func(s dependabot.RepoShape) bool { return s.HasGitHubActions },
		},
		{
			name: "workflows file outside .github is not an action",
			rel:  "workflows/ci.yml",
			want: func(s dependabot.RepoShape) bool { return !s.HasGitHubActions },
		},
		{
			name: "package json",
			rel:  "package.json",
			want: func(s dependabot.RepoShape) bool { return s.HasNPM },
		},
		{
			name: "unrelated file",
			rel:  "docs/nested/go.mod.txt",
			want: func(s dependabot.RepoShape) bool { return len(s.GoModuleDirs) == 0 && !s.HasGitHubActions && !s.HasNPM },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			shape := dependabot.RepoShape{}
			classify(tt.rel, &shape)

			if !tt.want(shape) {
				t.Errorf("classify(%q) produced %+v, want it to satisfy the expectation", tt.rel, shape)
			}
		})
	}
}

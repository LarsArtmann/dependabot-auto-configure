package configure_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
)

func TestSlugFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{url: "git@github.com:larsartmann/dependabot-auto-configure.git", want: "larsartmann/dependabot-auto-configure"},
		{url: "ssh://git@github.com/larsartmann/dependabot-auto-configure", want: "larsartmann/dependabot-auto-configure"},
		{url: "https://github.com/larsartmann/dependabot-auto-configure.git", want: "larsartmann/dependabot-auto-configure"},
		{url: "https://github.com/larsartmann/dependabot-auto-configure/", want: "larsartmann/dependabot-auto-configure"},
		{url: "https://gitlab.com/larsartmann/dependabot-auto-configure.git", want: ""},
		{url: "git@bitbucket.org:larsartmann/x.git", want: ""},
		{url: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := configure.SlugFromURL(tt.url); got != tt.want {
				t.Errorf("SlugFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestRepoSlugFromGit(t *testing.T) {
	t.Run("origin remote yields slug", func(t *testing.T) {
		root := t.TempDir()
		gitDir := filepath.Join(root, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatal(err)
		}

		config := "[core]\n\trepositoryformatversion = 0\n[remote \"origin\"]\n\turl = git@github.com:larsartmann/dependabot-auto-configure.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n[branch \"master\"]\n\tremote = origin\n"
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := configure.RepoSlugFromGit(root); got != "larsartmann/dependabot-auto-configure" {
			t.Errorf("RepoSlugFromGit() = %q, want larsartmann/dependabot-auto-configure", got)
		}
	})

	t.Run("no git directory yields empty slug", func(t *testing.T) {
		if got := configure.RepoSlugFromGit(t.TempDir()); got != "" {
			t.Errorf("RepoSlugFromGit() = %q, want empty", got)
		}
	})
}

func TestEnableSecurityFixesWithoutRemoteOrToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	outcome, err := configure.EnableSecurityFixes(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("EnableSecurityFixes() error = %v", err)
	}

	if outcome != configure.SecurityFixesNoRemote {
		t.Errorf("EnableSecurityFixes() = %q, want %q", outcome, configure.SecurityFixesNoRemote)
	}
}

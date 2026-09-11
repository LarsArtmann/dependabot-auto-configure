package configure_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestEnableSecurityFixesOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		want       string
		wantErr    bool
		wantMethod string
	}{
		{name: "no content means enabled", status: http.StatusNoContent, want: configure.SecurityFixesEnabled, wantMethod: http.MethodPut},
		{name: "not found skips", status: http.StatusNotFound, want: configure.SecurityFixesNotFound, wantMethod: http.MethodPut},
		{name: "forbidden skips", status: http.StatusForbidden, want: configure.SecurityFixesForbidden, wantMethod: http.MethodPut},
		{name: "unexpected status errors", status: http.StatusInternalServerError, wantErr: true, wantMethod: http.MethodPut},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", "test-token")
			t.Setenv("GH_TOKEN", "")

			var gotPath, gotMethod, gotAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotMethod, gotAuth = r.URL.Path, r.Method, r.Header.Get("Authorization")
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			restore := configure.SetGitHubAPIForTest(server.URL, server.Client())
			defer restore()

			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}

			config := "[remote \"origin\"]\n\turl = https://github.com/larsartmann/dependabot-auto-configure.git\n"
			if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}

			outcome, err := configure.EnableSecurityFixes(context.Background(), root)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EnableSecurityFixes() error = nil, want one (outcome %q)", outcome)
				}

				return
			}

			if err != nil {
				t.Fatalf("EnableSecurityFixes() error = %v", err)
			}

			if outcome != tt.want {
				t.Errorf("EnableSecurityFixes() = %q, want %q", outcome, tt.want)
			}

			if wantPath := "/repos/larsartmann/dependabot-auto-configure/automated-security-fixes"; gotPath != wantPath {
				t.Errorf("request path = %q, want %q", gotPath, wantPath)
			}

			if gotMethod != tt.wantMethod {
				t.Errorf("request method = %q, want %q", gotMethod, tt.wantMethod)
			}

			if gotAuth != "Bearer test-token" {
				t.Errorf("authorization = %q, want Bearer test-token", gotAuth)
			}
		})
	}
}

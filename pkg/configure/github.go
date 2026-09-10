package configure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Security-fix enablement outcome values for Result.SecurityFixes.
const (
	SecurityFixesEnabled   = "enabled"
	SecurityFixesNoToken   = "skipped-no-token"
	SecurityFixesNoRemote  = "skipped-no-remote"
	SecurityFixesNotFound  = "skipped-repo-not-found"
	SecurityFixesForbidden = "skipped-forbidden"
)

// repoSlugFromGit extracts "owner/repo" from the origin remote of the git
// repository at root. Supports HTTPS and SSH GitHub URL shapes and returns
// "" when no origin remote exists or the URL is not a GitHub repository.
func repoSlugFromGit(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "[remote \"origin\"]") {
			continue
		}

		for _, cfg := range lines[i+1:] {
			trimmed := strings.TrimSpace(cfg)
			if strings.HasPrefix(trimmed, "[") {
				break
			}

			url, ok := strings.CutPrefix(trimmed, "url = ")
			if !ok {
				continue
			}

			return slugFromURL(url)
		}
	}

	return ""
}

// slugFromURL converts a GitHub remote URL to "owner/repo".
func slugFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")

	switch {
	case strings.HasPrefix(url, "git@github.com:"):
		return strings.TrimPrefix(url, "git@github.com:")
	case strings.HasPrefix(url, "ssh://git@github.com/"):
		return strings.TrimPrefix(url, "ssh://git@github.com/")
	case strings.HasPrefix(url, "https://github.com/"):
		return strings.TrimPrefix(url, "https://github.com/")
	default:
		return ""
	}
}

// githubToken reads the API token from the environment, preferring
// GITHUB_TOKEN over GH_TOKEN.
func githubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}

	return os.Getenv("GH_TOKEN")
}

// EnableSecurityFixes turns on automated security fixes for the repository
// at root via the GitHub API. The returned string is one of the
// SecurityFixes* outcome values; an error is returned only for unexpected
// transport failures.
func EnableSecurityFixes(ctx context.Context, root string) (string, error) {
	slug := repoSlugFromGit(root)
	if slug == "" {
		return SecurityFixesNoRemote, nil
	}

	token := githubToken()
	if token == "" {
		return SecurityFixesNoToken, nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/automated-security-fixes", slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call %s: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNoContent:
		return SecurityFixesEnabled, nil
	case resp.StatusCode == http.StatusNotFound:
		return SecurityFixesNotFound, nil
	case resp.StatusCode == http.StatusForbidden:
		return SecurityFixesForbidden, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

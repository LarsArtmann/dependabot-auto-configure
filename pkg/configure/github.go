package configure

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// githubAPIBase is the API root for security-fix enablement; a variable so
// tests can point the client at a stub server.
var githubAPIBase = "https://api.github.com"

// githubAPIClient bounds the security-fixes call so a stalled GitHub API
// cannot hang the CLI indefinitely; the request context still governs
// cancellation.
var githubAPIClient = &http.Client{Timeout: 15 * time.Second}

// EnableSecurityFixes turns on automated security fixes for the repository
// at root via the GitHub API. The returned outcome is one of the
// SecurityFixes* values; an error is returned only for unexpected
// transport failures, each a typed error from this package.
func EnableSecurityFixes(ctx context.Context, root string) (SecurityFixesOutcome, error) {
	slug := repoSlugFromGit(root)
	if slug == "" {
		return SecurityFixesNoRemote, nil
	}

	token := githubToken()
	if token == "" {
		return SecurityFixesNoToken, nil
	}

	url := fmt.Sprintf("%s/repos/%s/automated-security-fixes", githubAPIBase, slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return "", &GitHubRequestError{URL: url, Cause: err}
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := githubAPIClient.Do(req)
	if err != nil {
		return "", &APITransportError{Op: TransportOpCall, URL: url, Cause: err}
	}

	outcome, outcomeErr := securityFixesOutcome(resp, url)
	if closeErr := resp.Body.Close(); closeErr != nil && outcomeErr == nil {
		return "", &APITransportError{Op: TransportOpCloseResponse, URL: url, Cause: closeErr}
	}

	return outcome, outcomeErr
}

// securityFixesOutcome maps an API response to its enablement outcome.
// The modeled statuses are success and the two skip conditions; anything
// else is an UnexpectedStatusError carrying the (truncated) body.
func securityFixesOutcome(resp *http.Response, url string) (SecurityFixesOutcome, error) {
	switch resp.StatusCode {
	case http.StatusNoContent:
		return SecurityFixesEnabled, nil
	case http.StatusNotFound:
		return SecurityFixesNotFound, nil
	case http.StatusForbidden:
		return SecurityFixesForbidden, nil
	default:
		body, bodyErr := io.ReadAll(io.LimitReader(resp.Body, 512))

		return "", &UnexpectedStatusError{URL: url, StatusCode: resp.StatusCode, Body: string(body), BodyErr: bodyErr}
	}
}

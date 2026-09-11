package configure

import "net/http"

// Exported aliases for the external test binary: the git-config and URL
// parsers are pure functions worth testing without widening the package API.
var (
	SlugFromURL     = slugFromURL
	RepoSlugFromGit = repoSlugFromGit
)

// SetGitHubAPIForTest points the security-fixes client at a stub server and
// returns a restore function.
func SetGitHubAPIForTest(baseURL string, client *http.Client) (restore func()) {
	origBase, origClient := githubAPIBase, githubAPIClient
	githubAPIBase, githubAPIClient = baseURL, client

	return func() { githubAPIBase, githubAPIClient = origBase, origClient }
}

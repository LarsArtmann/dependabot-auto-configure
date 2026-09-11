package configure

// Exported aliases for the external test binary: the git-config and URL
// parsers are pure functions worth testing without widening the package API.
var (
	SlugFromURL     = slugFromURL
	RepoSlugFromGit = repoSlugFromGit
)

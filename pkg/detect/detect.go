// Package detect reads a repository's shape: the facts about ecosystems
// and modules that Dependabot configuration needs. Detection is read-only
// and never guesses — only file presence is reported.
package detect

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

// skippedSegments are directory names whose subtrees never contain
// meaningful module manifests: VCS internals, package managers' output,
// and Go's convention for fixtures.
var skippedSegments = map[string]bool{
	".git":         true,
	"node_modules": true,
	"testdata":     true,
	"vendor":       true,
}

// Detector detects the repository shape under Root.
type Detector struct {
	Root string
}

// NewDetector returns a Detector for the given repository root.
func NewDetector(root string) Detector {
	return Detector{Root: root}
}

// Shape walks the repository and reports detected ecosystems. go.mod files
// under skipped directories are ignored; npm detection covers only a root
// package.json (workspace member detection is future work); GitHub Actions
// detection covers .github/workflows/*.yml and *.yaml.
func (d Detector) Shape() (dependabot.RepoShape, error) {
	shape := dependabot.RepoShape{}

	err := filepath.WalkDir(d.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if d.skipDir(path, entry.Name()) {
				return fs.SkipDir
			}

			return nil
		}

		rel, relErr := filepath.Rel(d.Root, path)
		if relErr != nil {
			return relErr
		}

		classify(rel, &shape)

		return nil
	})
	if err != nil {
		return dependabot.RepoShape{}, err
	}

	return shape, nil
}

// skipDir reports whether the directory subtree can be pruned: it is
// outside the repository root and either a known no-manifest directory or
// a hidden directory other than .github.
func (d Detector) skipDir(path, name string) bool {
	if path == d.Root {
		return false
	}

	return skippedSegments[name] || (strings.HasPrefix(name, ".") && name != ".github")
}

// classify records one walked file in the repository shape.
func classify(rel string, shape *dependabot.RepoShape) {
	switch {
	case rel == "go.mod":
		shape.GoModuleDirs = append(shape.GoModuleDirs, "")
	case strings.HasSuffix(rel, "/go.mod"):
		shape.GoModuleDirs = append(shape.GoModuleDirs, filepath.Dir(rel))
	case strings.HasPrefix(rel, filepath.Join(".github", "workflows")+string(filepath.Separator)) &&
		(strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml")):
		shape.HasGitHubActions = true
	case rel == "package.json":
		shape.HasNPM = true
	}
}

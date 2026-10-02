// Package detect reads a repository's shape: the facts about ecosystems
// and modules that Dependabot configuration needs. Detection is read-only
// and never guesses — only file presence (plus package.json workspace
// declarations) is reported.
package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
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

// npmLockfileNames are lockfiles whose presence marks a nested package.json
// as a real package (not a stray manifest) even without a workspace
// declaration.
var npmLockfileNames = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"bun.lockb":         true,
	"bun.lock":          true,
}

// ecoKind names the per-directory ecosystems detected by manifest presence.
type ecoKind int

// The detected per-directory ecosystems, in generation order.
const (
	ecoPip ecoKind = iota
	ecoCargo
	ecoGradle
)

// manifestKinds maps a manifest file name to the ecosystem whose directory
// it marks. Go modules keep a dedicated go.mod branch; package.json is
// recorded with workspace knowledge by classifyPackageJSON.
var manifestKinds = map[string]ecoKind{
	"requirements.txt":    ecoPip,
	"Pipfile":             ecoPip,
	"pyproject.toml":      ecoPip,
	"Cargo.toml":          ecoCargo,
	"build.gradle":        ecoGradle,
	"build.gradle.kts":    ecoGradle,
	"settings.gradle":     ecoGradle,
	"settings.gradle.kts": ecoGradle,
}

// markManifest records the directory of one detected manifest.
func (w *walkResult) markManifest(kind ecoKind, dir string) {
	switch kind {
	case ecoPip:
		w.pipDirs = append(w.pipDirs, dir)
	case ecoCargo:
		w.cargoDirs = append(w.cargoDirs, dir)
	case ecoGradle:
		w.gradleDirs = append(w.gradleDirs, dir)
	}
}

// Detector detects the repository shape under Root.
type Detector struct {
	Root string
}

// NewDetector returns a Detector for the given repository root.
func NewDetector(root string) Detector {
	return Detector{Root: root}
}

// walkResult collects raw per-file facts during the walk. Directory lists
// hold slash-separated paths relative to the root ("" = root); the npm
// workspace resolution happens once, after the walk.
type walkResult struct {
	goModuleDirs     []string
	hasGitHubActions bool
	packageJSONDirs  []string
	lockfileDirs     map[string]bool
	workspacesFound  bool
	pipDirs          []string
	cargoDirs        []string
	gradleDirs       []string
}

// Shape walks the repository and reports detected ecosystems. Manifests
// under skipped directories are ignored; npm detection covers the root
// package.json plus nested ones that are workspace members (a package.json
// anywhere declares "workspaces") or carry their own lockfile; GitHub
// Actions detection covers .github/workflows/*.yml and *.yaml.
func (d Detector) Shape() (dependabot.RepoShape, error) {
	walked := &walkResult{lockfileDirs: map[string]bool{}}

	err := filepath.WalkDir(d.Root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if d.skipDir(p, entry.Name()) {
				return fs.SkipDir
			}

			return nil
		}

		rel, relErr := filepath.Rel(d.Root, p)
		if relErr != nil {
			return relErr
		}

		rel = filepath.ToSlash(rel)
		classify(rel, walked)

		if path.Base(rel) == "package.json" {
			if readErr := d.classifyPackageJSON(rel, walked); readErr != nil {
				return readErr
			}
		}

		return nil
	})
	if err != nil {
		return dependabot.RepoShape{}, err
	}

	return walked.finalize(), nil
}

// finalize resolves the collected facts into the repository shape: nested
// package.json directories count as npm entries when a workspace
// declaration exists anywhere or the directory has its own lockfile.
func (w *walkResult) finalize() dependabot.RepoShape {
	shape := dependabot.RepoShape{
		GoModuleDirs:     w.goModuleDirs,
		HasGitHubActions: w.hasGitHubActions,
		PipDirs:          w.pipDirs,
		CargoDirs:        w.cargoDirs,
		GradleDirs:       w.gradleDirs,
	}

	for _, dir := range w.packageJSONDirs {
		if dir == "" || w.workspacesFound || w.lockfileDirs[dir] {
			shape.NPMDirs = append(shape.NPMDirs, dir)
		}
	}

	sort.Slice(shape.NPMDirs, func(i, j int) bool {
		ci, cj := dependabot.CanonicalDir(shape.NPMDirs[i]), dependabot.CanonicalDir(shape.NPMDirs[j])
		if ci == "/" {
			return true
		}

		if cj == "/" {
			return false
		}

		return ci < cj
	})

	return shape
}

// classifyPackageJSON reads one package.json and records its directory,
// flagging a non-empty "workspaces" declaration. A missing file is not an
// error: detection never guesses, so an unreadable manifest simply marks
// the directory without workspace knowledge.
func (d Detector) classifyPackageJSON(rel string, walked *walkResult) error {
	walked.packageJSONDirs = append(walked.packageJSONDirs, manifestDir(rel))

	data, err := os.ReadFile(filepath.Join(d.Root, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return &UnreadableManifestError{Path: rel, Cause: err}
	}

	var pkg struct {
		Workspaces json.RawMessage `json:"workspaces"`
	}

	if err := json.Unmarshal(data, &pkg); err == nil {
		if trimmed := strings.TrimSpace(string(pkg.Workspaces)); trimmed != "" && trimmed != "null" {
			walked.workspacesFound = true
		}
	}

	// A malformed manifest contributes no workspace knowledge. The
	// directory stays recorded; membership then needs a lockfile, so a
	// broken manifest can only ever make detection MORE conservative.

	return nil
}

// skipDir reports whether the directory subtree can be pruned: it is
// outside the repository root and either a known no-manifest directory or
// a hidden directory other than .github.
func (d Detector) skipDir(p, name string) bool {
	if p == d.Root {
		return false
	}

	return skippedSegments[name] || (strings.HasPrefix(name, ".") && name != ".github")
}

// classify records one walked file in the raw walk result. rel is a
// slash-separated path relative to the repository root, so the checks are
// identical on every operating system.
func classify(rel string, w *walkResult) {
	if rel == "go.mod" {
		w.goModuleDirs = append(w.goModuleDirs, "")

		return
	}

	if strings.HasSuffix(rel, "/go.mod") {
		w.goModuleDirs = append(w.goModuleDirs, path.Dir(rel))

		return
	}

	if strings.HasPrefix(rel, ".github/workflows/") &&
		(strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml")) {
		w.hasGitHubActions = true

		return
	}

	base := path.Base(rel)

	if npmLockfileNames[base] {
		w.lockfileDirs[manifestDir(rel)] = true

		return
	}

	if kind, ok := manifestKinds[base]; ok {
		w.markManifest(kind, manifestDir(rel))
	}
}

// manifestDir returns the slash-separated directory of a root-relative
// file path, with "." normalized to "" (the repository root).
func manifestDir(rel string) string {
	dir := path.Dir(rel)
	if dir == "." {
		return ""
	}

	return dir
}

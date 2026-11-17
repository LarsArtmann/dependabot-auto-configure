// Package dependabot models GitHub Dependabot configuration files
// (.github/dependabot.yml) as typed Go values, so detection, generation,
// and repair all speak one vocabulary instead of shuffling raw YAML.
package dependabot

import (
	"bytes"
	"fmt"
	"path"
	"strconv"

	"github.com/go-faster/yaml"
)

// CurrentVersion is the only Dependabot configuration version GitHub supports.
const CurrentVersion Version = 2

// Version is the schema version of a Dependabot configuration file.
type Version int

// Ecosystem is a Dependabot package-ecosystem identifier. Only ecosystems
// this tool can detect are named; arbitrary values survive round-trips via
// the verbatim passthrough (see Update.Unmodeled) instead of the type system.
type Ecosystem string

// Supported ecosystems.
const (
	EcosystemGoModules     Ecosystem = "gomod"
	EcosystemGitHubActions Ecosystem = "github-actions"
	EcosystemNPM           Ecosystem = "npm"
	EcosystemPip           Ecosystem = "pip"
	EcosystemCargo         Ecosystem = "cargo"
	EcosystemGradle        Ecosystem = "gradle"
)

// Interval is a Dependabot schedule interval.
type Interval string

// IntervalWeekly is the canonical interval: often enough to stay fresh,
// rare enough to keep the PR queue quiet.
const IntervalWeekly Interval = "weekly"

// DefaultOpenPullRequestsLimit bounds how many update PRs Dependabot keeps
// open per ecosystem entry. Five matches the account-wide convention.
const DefaultOpenPullRequestsLimit = 5

const yamlIndentWidth = 2

// Canonical group names. Grouping collapses the minor/patch churn into one
// PR per entry; GitHub Actions use a pattern group because they have no
// semver update types.
const (
	GroupMinorAndPatch = "minor-and-patch"
	GroupActions       = "actions"
)

// UpdateType is a Dependabot version-update type for type-based groups.
type UpdateType string

// Supported update types.
const (
	UpdateTypeMajor UpdateType = "major"
	UpdateTypeMinor UpdateType = "minor"
	UpdateTypePatch UpdateType = "patch"
)

// mergeKeyTag identifies YAML merge keys ("<<"), the one scalar key a
// verbatim re-encode cannot reproduce faithfully.
const mergeKeyTag = "!!merge"

// Schedule controls when Dependabot runs an updates entry. Day, Time, and
// Timezone are user customizations: decoded, preserved verbatim by Reconcile,
// and never generated — the canonical form schedules a plain weekly interval.
// Keys outside the modeled four (cron, ...) ride in Unmodeled.
type Schedule struct {
	Interval Interval `yaml:"interval"`
	Day      string   `yaml:"day,omitempty"`
	Time     string   `yaml:"time,omitempty"`
	Timezone string   `yaml:"timezone,omitempty"`

	// Unmodeled carries every schedule key outside the modeled set, as raw
	// YAML nodes: decoded, preserved verbatim by Reconcile, re-emitted by
	// Encode, never generated, invisible to Diff. A rewrite drops nothing.
	Unmodeled map[string]yaml.Node `yaml:",inline"`
}

// TypeGroup groups updates by semver update type. Keys outside the modeled
// set (exclude-patterns, applies-to, ...) ride in Unmodeled.
type TypeGroup struct {
	UpdateTypes []UpdateType `yaml:"update-types"`

	// Unmodeled carries group keys outside the modeled set, verbatim (see
	// Schedule.Unmodeled).
	Unmodeled map[string]yaml.Node `yaml:",inline"`
}

// PatternGroup groups updates by dependency name pattern. Keys outside the
// modeled set ride in Unmodeled. Patterns is omitempty so a group carrying
// only unmodeled keys (e.g. actions with update-types) re-encodes without
// inventing an empty pattern list that would match nothing.
type PatternGroup struct {
	Patterns []string `yaml:"patterns,omitempty"`

	// Unmodeled carries group keys outside the modeled set, verbatim (see
	// Schedule.Unmodeled).
	Unmodeled map[string]yaml.Node `yaml:",inline"`
}

// Groups is the set of update groups on an entry. Pointer fields keep unset
// groups out of the YAML instead of emitting empty mappings. Group names
// other than the canonical two are not typed: they ride in Unmodeled,
// preserved verbatim, so a repair never drops a user's custom grouping.
type Groups struct {
	MinorAndPatch *TypeGroup    `yaml:"minor-and-patch,omitempty"`
	Actions       *PatternGroup `yaml:"actions,omitempty"`

	// Unmodeled carries group names outside the canonical two, as raw YAML
	// nodes keyed by group name: preserved verbatim, never generated.
	Unmodeled map[string]yaml.Node `yaml:",inline"`

	// NonMapping is an audit-only flag, set by UnmarshalYAML (never
	// serialized, never generated): the YAML carried a non-mapping
	// `groups:` value (e.g. the list form some hand-written configs use).
	// That shape has no typed representation a rewrite could preserve, so
	// Decode reports the document unsafe and repair stays suggest-only.
	NonMapping bool `yaml:"-"`
}

// MinorAndPatchGroups builds the canonical minor+patch group.
func MinorAndPatchGroups() *Groups {
	return &Groups{
		MinorAndPatch: &TypeGroup{
			UpdateTypes: []UpdateType{UpdateTypeMinor, UpdateTypePatch},
		},
	}
}

// ActionGroups builds the canonical GitHub Actions pattern group.
func ActionGroups() *Groups {
	return &Groups{
		Actions: &PatternGroup{Patterns: []string{"*"}},
	}
}

// Empty reports whether no modeled group is configured. Groups with names
// outside the canonical two are not typed, so such an entry still reports
// empty; Update.HasUnmodeledGroups distinguishes "no groups at all" from
// "groups this tool cannot see".
func (g *Groups) Empty() bool {
	return g == nil || (g.MinorAndPatch == nil && g.Actions == nil)
}

// UnmarshalYAML decodes canonical mapping-form groups and tolerates
// non-mapping shapes (e.g. the list form some hand-written configs use)
// without failing the whole parse: the NonMapping flag makes Decode report
// the entry unsafe, so repair never rewrites and the non-canonical shape
// survives.
func (g *Groups) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		g.NonMapping = true

		return nil
	}

	type plain Groups

	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}

	*g = Groups(p)

	return nil
}

// CommitMessage models the entry-level commit-message customization (prefix,
// include, separator). User customization: preserved by Reconcile, never
// generated, invisible to Diff — same contract as Labels. Keys outside the
// modeled three (prefix-development, ...) ride in Unmodeled.
type CommitMessage struct {
	Prefix    string  `yaml:"prefix,omitempty"`
	Include   string  `yaml:"include,omitempty"`
	Separator *string `yaml:"separator,omitempty"`

	// Unmodeled carries commit-message keys outside the modeled set,
	// verbatim (see Schedule.Unmodeled).
	Unmodeled map[string]yaml.Node `yaml:",inline"`
}

// Update is one entry under "updates:". A nil Schedule, a zero
// OpenPullRequestsLimit, or a nil Groups each mean "missing" and are
// reported as findings; there is no separate present-but-invalid state
// because this schema only knows the canonical values. Directory and
// Directories are alternatives in GitHub's schema; every entry needs one
// of them (see Config.Validate). Fields this tool does not type at all
// (ignore, reviewers, allow, target-branch, ...) ride in Unmodeled.
type Update struct {
	PackageEcosystem Ecosystem `yaml:"package-ecosystem"`
	// Directory is omitempty: entries using the glob `directories`
	// alternative carry no single directory, and an emitted empty
	// "" would be a schema violation.
	Directory             string    `yaml:"directory,omitempty"`
	Directories           []string  `yaml:"directories,omitempty"`
	Schedule              *Schedule `yaml:"schedule,omitempty"`
	OpenPullRequestsLimit int       `yaml:"open-pull-requests-limit,omitempty"`
	// Groups is the entry's update groups (see the Groups contract above).
	Groups *Groups `yaml:"groups,omitempty"`

	// Labels are PR labels carried on the entry. User customizations:
	// preserved by Reconcile, never generated, invisible to Diff.
	Labels []string `yaml:"labels,omitempty"`

	// CommitMessage is the PR commit-message customization. Same
	// customization contract as Labels.
	CommitMessage *CommitMessage `yaml:"commit-message,omitempty"`

	// Unmodeled carries every updates-entry key outside the typed set, as
	// raw YAML nodes: decoded, preserved verbatim by Reconcile, re-emitted
	// by Encode, never generated, invisible to Diff. This is what lets
	// repair fill missing canonical fields on configs using schema this
	// tool never modeled — the rewrite drops nothing.
	Unmodeled map[string]yaml.Node `yaml:",inline"`
}

// HasUnmodeledGroups reports whether the entry's groups mapping carries
// names outside the canonical model. Such an entry HAS groups (issue #3):
// the grouping-missing finding must not fire, and since v0.4.0 the names
// are carried verbatim through repair instead of blocking it.
func (u Update) HasUnmodeledGroups() bool {
	return u.Groups != nil && len(u.Groups.Unmodeled) > 0
}

// coversDir reports whether the entry configures the given canonical
// directory: via its own directory, or via one of its glob `directories`
// patterns matching it. An unparseable pattern counts as covering — a
// duplicate entry for the same ecosystem and directory would break
// Dependabot, a missing canonical fill would not.
func (u Update) coversDir(dir string) bool {
	if u.Directory == dir {
		return true
	}

	for _, pattern := range u.Directories {
		if pattern == dir {
			return true
		}

		if match, err := path.Match(pattern, dir); err != nil || match {
			return true
		}
	}

	return false
}

// Config is the whole .github/dependabot.yml document.
type Config struct {
	Version Version  `yaml:"version"`
	Updates []Update `yaml:"updates"`

	// Unmodeled carries every top-level key outside {version, updates}
	// (registries, enable-beta-ecosystems, ...), as raw YAML nodes:
	// preserved verbatim by Reconcile, re-emitted by Encode, never
	// generated, invisible to Diff.
	Unmodeled map[string]yaml.Node `yaml:",inline"`
}

// DecodeResult is the outcome of decoding an existing configuration.
// Unsafe is true only when the document carries a construct a canonical
// rewrite CANNOT reproduce verbatim; everything else — unknown keys,
// custom group names, unmodeled schema — is captured as raw nodes and
// survives Encode byte-for-value.
type DecodeResult struct {
	Config Config

	// UnsafeReasons names every construct a rewrite cannot carry verbatim,
	// one human-readable string each: a non-mapping `groups:` value, a
	// YAML alias, or a merge key inside a captured unmodeled subtree.
	// Any reason makes repair suggest-only, because a rewrite would
	// silently change or drop the user's shape.
	UnsafeReasons []string

	// Unsafe is true when UnsafeReasons is non-empty.
	Unsafe bool
}

// Decode parses YAML into a Config, capturing every construct outside the
// typed model as raw YAML nodes (the verbatim passthrough), and audits the
// result for the rare shapes that cannot round-trip. Decoding is
// deliberately lenient (unmodeled keys ride along instead of erroring) so
// existing configs are always readable and almost always repairable.
func Decode(data []byte) (DecodeResult, error) {
	res := DecodeResult{}

	if err := yaml.Unmarshal(data, &res.Config); err != nil {
		return res, &UnparseableConfigError{Stage: DecodeStageParse, Cause: err}
	}

	res.UnsafeReasons = auditUnrepresentable(res.Config)
	res.Unsafe = len(res.UnsafeReasons) > 0

	return res, nil
}

// auditUnrepresentable walks a decoded config for constructs a canonical
// rewrite cannot carry verbatim: a non-mapping `groups:` value (the list
// form some hand-written configs use) and YAML aliases or merge keys inside
// captured unmodeled subtrees. It returns one reason string per construct.
func auditUnrepresentable(cfg Config) []string {
	var reasons []string

	for key := range cfg.Unmodeled {
		reasons = scanUnmodeledNode(reasons, fmt.Sprintf("top-level key %q", key), cfg.Unmodeled[key])
	}

	for i := range cfg.Updates {
		update := &cfg.Updates[i]
		where := fmt.Sprintf("entry %d", i)

		for key := range update.Unmodeled {
			reasons = scanUnmodeledNode(reasons, fmt.Sprintf("%s key %q", where, key), update.Unmodeled[key])
		}

		if update.Schedule != nil {
			for key := range update.Schedule.Unmodeled {
				reasons = scanUnmodeledNode(
					reasons, fmt.Sprintf("%s schedule key %q", where, key), update.Schedule.Unmodeled[key],
				)
			}
		}

		if update.CommitMessage != nil {
			for key := range update.CommitMessage.Unmodeled {
				reasons = scanUnmodeledNode(
					reasons, fmt.Sprintf("%s commit-message key %q", where, key), update.CommitMessage.Unmodeled[key],
				)
			}
		}

		reasons = auditGroupsShape(reasons, update.Groups, where)
	}

	return reasons
}

// auditGroupsShape audits one entry's groups for unrepresentable shapes:
// the non-mapping form, plus aliases and merge keys inside unmodeled group
// names and unmodeled keys of canonical group values.
func auditGroupsShape(reasons []string, groups *Groups, where string) []string {
	if groups == nil {
		return reasons
	}

	if groups.NonMapping {
		reasons = append(reasons, fmt.Sprintf("%s groups must be a mapping, not a list or scalar", where))
	}

	for name := range groups.Unmodeled {
		reasons = scanUnmodeledNode(reasons, where+" group "+strconv.Quote(name), groups.Unmodeled[name])
	}

	if groups.MinorAndPatch != nil {
		for key := range groups.MinorAndPatch.Unmodeled {
			reasons = scanUnmodeledNode(
				reasons, fmt.Sprintf("%s group %q key %q", where, GroupMinorAndPatch, key),
				groups.MinorAndPatch.Unmodeled[key],
			)
		}
	}

	if groups.Actions != nil {
		for key := range groups.Actions.Unmodeled {
			reasons = scanUnmodeledNode(
				reasons, fmt.Sprintf("%s group %q key %q", where, GroupActions, key), groups.Actions.Unmodeled[key],
			)
		}
	}

	return reasons
}

// scanUnmodeledNode reports alias and merge-key nodes inside one captured
// unmodeled value. An alias re-emitted verbatim dangles whenever its anchor
// is defined inside a typed field (the typed re-encode strips anchors), and
// a merge key re-encodes with its tag spelled out — a shape GitHub rejects.
// Both are vanishingly rare in dependabot.yml; suggest-only is the safe call.
func scanUnmodeledNode(reasons []string, where string, node yaml.Node) []string {
	switch node.Kind {
	case yaml.AliasNode:
		return append(reasons, fmt.Sprintf("YAML alias inside unmodeled %s cannot be carried verbatim", where))
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			if keyNode.Tag == mergeKeyTag || keyNode.Value == "<<" {
				reasons = append(
					reasons, fmt.Sprintf("merge key inside unmodeled %s cannot be carried verbatim", where),
				)

				continue
			}

			reasons = scanUnmodeledNode(reasons, where, *node.Content[i+1])
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			reasons = scanUnmodeledNode(reasons, where, *child)
		}
	case yaml.DocumentNode, yaml.ScalarNode:
		// Leaf shapes carry no nested aliases or merge keys.
	}

	return reasons
}

// Encode renders the canonical YAML: two-space indent, stable field order
// (struct order, unmodeled keys sorted after the typed fields), trailing
// newline. Output is deterministic for equal Configs, which makes
// byte-comparison the idempotence check for repair.
func (c Config) Encode() ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndentWidth)

	if err := enc.Encode(c); err != nil {
		return nil, &EncodeError{Cause: err}
	}

	out := buf.Bytes()
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}

	return out, nil
}

// Find returns the index of the entry configuring the given ecosystem and
// directory, or -1. An entry whose `directories` globs cover the directory
// matches too (see Update.coversDir), so repair never appends an entry a
// user's glob-shaped config already covers.
func (c Config) Find(eco Ecosystem, dir string) int {
	for i, u := range c.Updates {
		if u.PackageEcosystem == eco && u.coversDir(dir) {
			return i
		}
	}

	return -1
}

// Validate reports entries missing their required fields (ecosystem; a
// directory, single or glob list). Repair never writes a config containing
// invalid entries.
func (c Config) Validate() error {
	for i, u := range c.Updates {
		var missing []RequiredField
		if u.PackageEcosystem == "" {
			missing = append(missing, FieldPackageEcosystem)
		}

		if u.Directory == "" && len(u.Directories) == 0 {
			missing = append(missing, FieldDirectory)
		}

		if len(missing) > 0 {
			return &InvalidEntryError{Index: i, Missing: missing}
		}
	}

	return nil
}

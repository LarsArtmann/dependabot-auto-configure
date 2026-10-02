// Package dependabot models GitHub Dependabot configuration files
// (.github/dependabot.yml) as typed Go values, so detection, generation,
// and repair all speak one vocabulary instead of shuffling raw YAML.
package dependabot

import (
	"bytes"

	"github.com/go-faster/yaml"
)

// CurrentVersion is the only Dependabot configuration version GitHub supports.
const CurrentVersion Version = 2

// Version is the schema version of a Dependabot configuration file.
type Version int

// Ecosystem is a Dependabot package-ecosystem identifier. Only ecosystems
// this tool can detect are named; arbitrary values survive round-trips via
// DecodeResult.Unknown* reporting instead of the type system.
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

// Schedule controls when Dependabot runs an updates entry. Day, Time, and
// Timezone are user customizations: decoded, preserved verbatim by Reconcile,
// and never generated — the canonical form schedules a plain weekly interval.
type Schedule struct {
	Interval Interval `yaml:"interval"`
	Day      string   `yaml:"day,omitempty"`
	Time     string   `yaml:"time,omitempty"`
	Timezone string   `yaml:"timezone,omitempty"`
}

// TypeGroup groups updates by semver update type.
type TypeGroup struct {
	UpdateTypes []UpdateType `yaml:"update-types"`
}

// PatternGroup groups updates by dependency name pattern.
type PatternGroup struct {
	Patterns []string `yaml:"patterns"`
}

// Groups is the set of update groups on an entry. Pointer fields keep unset
// groups out of the YAML instead of emitting empty mappings; unknown group
// names from existing configs are audited by Decode, never modeled.
type Groups struct {
	MinorAndPatch *TypeGroup    `yaml:"minor-and-patch,omitempty"`
	Actions       *PatternGroup `yaml:"actions,omitempty"`
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
// outside the canonical two are not modeled, so such an entry still reports
// empty; Update.HasUnmodeledGroups distinguishes "no groups at all" from
// "groups this tool cannot see".
func (g *Groups) Empty() bool {
	return g == nil || (g.MinorAndPatch == nil && g.Actions == nil)
}

// UnmarshalYAML decodes canonical mapping-form groups and tolerates
// non-mapping shapes (e.g. the list form some hand-written configs use)
// without failing the whole parse: Decode's audit then reports the entry
// unsafe, so repair never rewrites and the non-canonical shape survives.
func (g *Groups) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
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
// generated, invisible to Diff — same contract as Labels.
type CommitMessage struct {
	Prefix    string  `yaml:"prefix,omitempty"`
	Include   string  `yaml:"include,omitempty"`
	Separator *string `yaml:"separator,omitempty"`
}

// Update is one entry under "updates:". A nil Schedule, a zero
// OpenPullRequestsLimit, or a nil Groups each mean "missing" and are
// reported as findings; there is no separate present-but-invalid state
// because this schema only knows the canonical values.
type Update struct {
	PackageEcosystem      Ecosystem `yaml:"package-ecosystem"`
	Directory             string    `yaml:"directory"`
	Schedule              *Schedule `yaml:"schedule,omitempty"`
	OpenPullRequestsLimit int       `yaml:"open-pull-requests-limit,omitempty"`
	// Groups is the entry's update groups (see the Groups contract above).
	Groups *Groups `yaml:"groups,omitempty"`

	// HasUnmodeledGroups is an audit-only flag, set by Decode (never
	// serialized, never generated): the YAML carried a groups mapping with
	// names outside the canonical model. The entry HAS groups, so the
	// grouping-missing finding must not fire; the unknown names themselves
	// are reported separately via DecodeResult.UnknownGroupNames.
	HasUnmodeledGroups bool `yaml:"-"`

	// Labels are PR labels carried on the entry. User customizations:
	// preserved by Reconcile, never generated, invisible to Diff.
	Labels []string `yaml:"labels,omitempty"`

	// CommitMessage is the PR commit-message customization. Same
	// customization contract as Labels.
	CommitMessage *CommitMessage `yaml:"commit-message,omitempty"`
}

// Config is the whole .github/dependabot.yml document.
type Config struct {
	Version Version  `yaml:"version"`
	Updates []Update `yaml:"updates"`
}

// DecodeResult is the outcome of decoding an existing configuration.
// The Unknown* fields describe constructs outside this tool's schema; any
// non-empty Unknown value makes regeneration unsafe because a rewrite
// would silently drop the user's customizations.
type DecodeResult struct {
	Config Config

	// UnknownTopLevel lists top-level YAML keys outside {version, updates},
	// e.g. "registries".
	UnknownTopLevel []string

	// UnknownEntryFields reports entries carrying fields outside the known
	// schema (assignees, commit-message, ...) or schedule blocks with keys
	// outside {interval, day, time, timezone}.
	UnknownEntryFields bool

	// UnknownGroupNames lists group names other than the canonical two.
	UnknownGroupNames []string

	// Unsafe is true when any Unknown* signal is present.
	Unsafe bool
}

// knownEntryFields is every updates-entry field this tool understands.
var knownEntryFields = map[string]bool{
	"package-ecosystem":        true,
	"directory":                true,
	"schedule":                 true,
	"open-pull-requests-limit": true,
	"groups":                   true,
	"labels":                   true,
	"commit-message":           true,
}

// knownCommitMessageFields is every commit-message-block key this tool
// understands. Unknown keys inside the block are audited unsafe: a rewrite
// would silently drop them, exactly like unknown schedule keys.
var knownCommitMessageFields = map[string]bool{
	"prefix":    true,
	"include":   true,
	"separator": true,
}

// knownScheduleFields is every schedule-block key this tool understands.
// Unknown schedule keys are audited unsafe — modeling only the four standard
// keys keeps a rewrite from silently dropping a schedule field the schema
// does not carry.
var knownScheduleFields = map[string]bool{
	"interval": true,
	"day":      true,
	"time":     true,
	"timezone": true,
}

// knownTopLevelFields is every top-level field this tool understands.
var knownTopLevelFields = map[string]bool{
	"version": true,
	"updates": true,
}

// knownGroupNames is every group name this tool understands.
var knownGroupNames = map[string]bool{
	GroupMinorAndPatch: true,
	GroupActions:       true,
}

// Decode parses YAML into a Config and audits it for constructs this tool
// does not model. Decoding is deliberately lenient (unknown fields land in
// the audit, not in an error) so existing configs are always readable.
func Decode(data []byte) (DecodeResult, error) {
	res := DecodeResult{}

	if err := yaml.Unmarshal(data, &res.Config); err != nil {
		return res, &UnparseableConfigError{Stage: DecodeStageParse, Cause: err}
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return res, &UnparseableConfigError{Stage: DecodeStageInspect, Cause: err}
	}

	for key := range raw {
		if !knownTopLevelFields[key] {
			res.UnknownTopLevel = append(res.UnknownTopLevel, key)
		}
	}

	auditEntries(&res, raw["updates"])

	res.Unsafe = len(res.UnknownTopLevel) > 0 || res.UnknownEntryFields || len(res.UnknownGroupNames) > 0

	return res, nil
}

// auditEntries flags updates entries carrying constructs outside the known
// schema: unknown entry fields, unknown schedule keys, unknown group names.
// Raw entries align with res.Config.Updates by document order; the per-entry
// audit result is folded back into the typed entry.
func auditEntries(res *DecodeResult, updates any) {
	entries, _ := updates.([]any)

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		if auditEntry(res, entryMap) && i < len(res.Config.Updates) {
			res.Config.Updates[i].HasUnmodeledGroups = true
		}
	}
}

// auditEntry audits one updates entry for constructs outside the known
// schema: unknown entry fields, unknown schedule keys, non-canonical or
// unknown groups, unknown commit-message keys. It reports whether the
// entry's groups mapping carries names outside the canonical model.
func auditEntry(res *DecodeResult, entryMap map[string]any) bool {
	for key := range entryMap {
		if !knownEntryFields[key] {
			res.UnknownEntryFields = true
		}
	}

	auditBlock(res, entryMap["schedule"], knownScheduleFields)
	auditBlock(res, entryMap["commit-message"], knownCommitMessageFields)

	return auditGroups(res, entryMap["groups"])
}

// auditBlock flags unknown keys inside one known schema block (schedule,
// commit-message): a rewrite would silently drop them.
func auditBlock(res *DecodeResult, block any, known map[string]bool) {
	keys, ok := block.(map[string]any)
	if !ok {
		return
	}

	for key := range keys {
		if !known[key] {
			res.UnknownEntryFields = true
		}
	}
}

// auditGroups flags non-canonical group shapes and unknown group names,
// reporting whether the groups mapping carries names outside the canonical
// model. A non-mapping groups value (e.g. the list form) is decoded by
// Groups.UnmarshalYAML without failing the parse; auditing it unsafe here
// means a rewrite can never drop the non-canonical shape — but it is not a
// groups mapping at all, so it does not count as unmodeled group names.
func auditGroups(res *DecodeResult, groups any) bool {
	names, ok := groups.(map[string]any)
	if !ok {
		res.UnknownEntryFields = res.UnknownEntryFields || groups != nil

		return false
	}

	unmodeled := false
	for name := range names {
		if !knownGroupNames[name] {
			res.UnknownGroupNames = append(res.UnknownGroupNames, name)
			unmodeled = true
		}
	}

	return unmodeled
}

// Encode renders the canonical YAML: two-space indent, stable field order
// (struct order), trailing newline. Output is deterministic for equal
// Configs, which makes byte-comparison the idempotence check for repair.
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

// entryKey identifies an updates entry by its natural unique pair.
type entryKey struct {
	ecosystem Ecosystem
	directory string
}

func keyOf(u Update) entryKey {
	return entryKey{ecosystem: u.PackageEcosystem, directory: u.Directory}
}

// Find returns the index of the entry with the given ecosystem and
// directory, or -1.
func (c Config) Find(eco Ecosystem, dir string) int {
	for i, u := range c.Updates {
		if keyOf(u) == (entryKey{eco, dir}) {
			return i
		}
	}

	return -1
}

// Validate reports entries missing their required fields (ecosystem,
// directory). Repair never writes a config containing invalid entries.
func (c Config) Validate() error {
	for i, u := range c.Updates {
		var missing []RequiredField
		if u.PackageEcosystem == "" {
			missing = append(missing, FieldPackageEcosystem)
		}

		if u.Directory == "" {
			missing = append(missing, FieldDirectory)
		}

		if len(missing) > 0 {
			return &InvalidEntryError{Index: i, Missing: missing}
		}
	}

	return nil
}

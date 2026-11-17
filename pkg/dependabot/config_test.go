package dependabot_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

func TestDecodeCanonicalConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		wantGroups  bool
		wantUpdates int
	}{
		{
			name: "canonical grouped config decodes clean",
			yaml: `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      minor-and-patch:
        update-types:
          - minor
          - patch
`,
			wantUpdates: 1,
			wantGroups:  true,
		},
		{
			name:        "unknown top-level key rides along",
			yaml:        "version: 2\nregistries:\n  npm: {}\nupdates: []\n",
			wantUpdates: 0,
		},
		{
			name: "unknown entry field rides along",
			yaml: "version: 2\nupdates:\n  - package-ecosystem: npm\n    directory: /\n    assignees:\n      - someone\n",
			wantUpdates: 1,
		},
		{
			name: "labels and schedule day are modeled customizations, safe",
			yaml: `version: 2
updates:
  - package-ecosystem: npm
    directory: /
    schedule:
      interval: weekly
      day: monday
    labels:
      - dependencies
`,
			wantUpdates: 1,
		},
		{
			name: "unknown group name rides along",
			yaml: `version: 2
updates:
  - package-ecosystem: npm
    directory: /
    groups:
      everything:
        patterns:
          - "*"
`,
			wantUpdates: 1,
		},
		{
			name:        "minimal entry decodes with missing optional fields",
			yaml:        "version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n",
			wantUpdates: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dec, err := dependabot.Decode([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			if len(dec.Config.Updates) != tt.wantUpdates {
				t.Errorf("Decode() updates = %d, want %d", len(dec.Config.Updates), tt.wantUpdates)
			}

			if dec.Unsafe {
				t.Errorf("Decode() unsafe = true, want false (unmodeled constructs ride along verbatim)")
			}

			if tt.wantGroups && (len(dec.Config.Updates) == 0 || dec.Config.Updates[0].Groups.Empty()) {
				t.Error("Decode() groups empty, want populated")
			}

			if tt.wantUpdates > 0 && !tt.wantGroups && !strings.Contains(tt.yaml, "groups:") &&
				!dec.Config.Updates[0].Groups.Empty() {
				t.Error("Decode() groups populated, want empty")
			}
		})
	}
}

// TestDecodeCustomNamedGroupsFlagsEntry pins the false-positive fix behind
// the grouping-missing finding: GitHub's schema allows arbitrary group
// names, so an entry whose groups use names outside the canonical model
// must report "has groups this tool cannot see" — per entry, not per
// document — while the names ride in Groups.Unmodeled, safe to repair.
func TestDecodeCustomNamedGroupsFlagsEntry(t *testing.T) {
	t.Parallel()

	dec, err := dependabot.Decode([]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      gomod:
        patterns:
          - "*"
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if dec.Unsafe {
		t.Error("Decode() Unsafe = true for custom group names, want false (carried verbatim)")
	}

	if len(dec.Config.Updates) != 2 {
		t.Fatalf("Decode() updates = %d, want 2", len(dec.Config.Updates))
	}

	if !dec.Config.Updates[0].HasUnmodeledGroups() {
		t.Error("Decode() entry 0 HasUnmodeledGroups() = false, want true (groups mapping with unknown names)")
	}

	if _, carried := dec.Config.Updates[0].Groups.Unmodeled["gomod"]; !carried {
		t.Error("Decode() entry 0 Groups.Unmodeled has no \"gomod\", want the custom name carried")
	}

	if dec.Config.Updates[1].HasUnmodeledGroups() {
		t.Error("Decode() entry 1 HasUnmodeledGroups() = true, want false (no groups mapping at all)")
	}

	if !dec.Config.Updates[0].Groups.Empty() {
		t.Error("Decode() entry 0 Groups not Empty, want Empty (canonical fields are the only modeled ones)")
	}
}

// TestDecodeGroupShapesFlagMatrix pins which group shapes count as
// "unmodeled names" (suppressing the grouping-missing finding) versus "no
// groups mapping at all" (finding fires, repair fills on the safe path).
func TestDecodeGroupShapesFlagMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		groupsYAML string
		wantFlag   bool
	}{
		{
			name:       "custom-named mapping counts as unmodeled groups",
			groupsYAML: "    groups:\n      gomod:\n        patterns:\n          - \"*\"\n",
			wantFlag:   true,
		},
		{
			name:       "empty mapping is no groups at all",
			groupsYAML: "    groups: {}\n",
			wantFlag:   false,
		},
		{
			name:       "null-valued canonical key is no modeled group",
			groupsYAML: "    groups:\n      minor-and-patch: null\n",
			wantFlag:   false,
		},
		{
			name:       "list form is not a groups mapping",
			groupsYAML: "    groups:\n      - minor-and-patch\n",
			wantFlag:   false,
		},
		{
			name:       "unknown name with null value counts as unmodeled groups",
			groupsYAML: "    groups:\n      gomod: null\n",
			wantFlag:   true,
		},
		{
			name:       "canonical null value is no modeled group",
			groupsYAML: "    groups:\n      actions: null\n",
			wantFlag:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			yaml := strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				tt.groupsYAML,
				"",
			}, "\n")

			dec, err := dependabot.Decode([]byte(yaml))
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			if len(dec.Config.Updates) != 1 {
				t.Fatalf("Decode() updates = %d, want 1", len(dec.Config.Updates))
			}

			if got := dec.Config.Updates[0].HasUnmodeledGroups(); got != tt.wantFlag {
				t.Errorf("Decode() HasUnmodeledGroups() = %v, want %v", got, tt.wantFlag)
			}
		})
	}
}

// TestDecodeUnmodeledGroupValueKeysCarried pins the group-value half of the
// passthrough: canonical group names carry a typed key schema
// (minor-and-patch knows update-types, actions knows patterns), and keys
// inside them outside that schema (exclude-patterns, applies-to, ...) are
// valid GitHub schema this tool does not type. They ride in the group's
// Unmodeled map, so repair fills missing canonical fields without dropping
// them; the modeled names keep decoding and the entry keeps its groups, so
// the grouping-missing suppression is untouched.
func TestDecodeUnmodeledGroupValueKeysCarried(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		groupsYAML string
	}{
		{
			name: "unknown key inside minor-and-patch is carried",
			groupsYAML: `      minor-and-patch:
        update-types:
          - minor
          - patch
        exclude-patterns:
          - go.mod
`,
		},
		{
			name: "unknown key inside actions is carried",
			groupsYAML: `      actions:
        patterns:
          - "*"
        applies-to:
          - security-updates
`,
		},
		{
			name: "update-types inside actions is carried too",
			groupsYAML: `      actions:
        patterns:
          - "*"
        update-types:
          - minor
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			yaml := strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				"    groups:",
				tt.groupsYAML,
				"",
			}, "\n")

			dec, err := dependabot.Decode([]byte(yaml))
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}

			if dec.Unsafe {
				t.Errorf("Decode() unsafe = true, want false (group-value keys ride along verbatim)")
			}

			if len(dec.Config.Updates) != 1 {
				t.Fatalf("Decode() updates = %d, want 1", len(dec.Config.Updates))
			}

			if dec.Config.Updates[0].HasUnmodeledGroups() {
				t.Error("Decode() HasUnmodeledGroups() = true, want false (names are canonical)")
			}

			if dec.Config.Updates[0].Groups.Empty() {
				t.Error("Decode() groups empty, want the modeled canonical groups kept")
			}

			encoded, err := dec.Config.Encode()
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			redecoded, err := dependabot.Decode(encoded)
			if err != nil {
				t.Fatalf("re-Decode() error = %v", err)
			}

			if !dependabot.Equal(dec.Config, redecoded.Config) {
				t.Errorf("round-trip lost group-value keys:\n%s", encoded)
			}
		})
	}
}

// TestDecodeCanonicalGroupValueNonMappingIsUnparseable pins the corruption
// boundary under the group-value passthrough: a canonical name whose value
// is not a mapping (list, scalar) fails the typed parse, so repair is
// suggest-only via the unparseable path and the malformed shape can never
// be rewritten. A null value is the documented exception: it decodes to "no
// modeled group" and stays on the safe path.
func TestDecodeCanonicalGroupValueNonMappingIsUnparseable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		groupsYAML string
	}{
		{
			name:       "list value",
			groupsYAML: "      minor-and-patch:\n        - minor\n",
		},
		{
			name:       "scalar value",
			groupsYAML: "      actions: \"*\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			yaml := strings.Join([]string{
				"version: 2",
				"updates:",
				"  - package-ecosystem: gomod",
				"    directory: /",
				"    groups:",
				tt.groupsYAML,
				"",
			}, "\n")

			if _, err := dependabot.Decode([]byte(yaml)); err == nil {
				t.Error("Decode() error = nil for non-mapping canonical group value, want unparseable")
			}
		})
	}
}

// TestDecodeUpdatesShapes pins the updates-list parse boundary: null items
// are skipped by the typed decoder, and scalar or mapping forms of
// `updates:` fail the parse (the corruption path, suggest-only).
func TestDecodeUpdatesShapes(t *testing.T) {
	t.Parallel()

	t.Run("null entry does not disturb the typed decode", func(t *testing.T) {
		t.Parallel()

		dec, err := dependabot.Decode([]byte(`version: 2
updates:
  - null
  - package-ecosystem: gomod
    directory: /
    groups:
      custom:
        patterns:
          - "*"
`))
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if len(dec.Config.Updates) != 1 {
			t.Fatalf("Decode() updates = %d, want 1 (the null item is skipped)", len(dec.Config.Updates))
		}

		if !dec.Config.Updates[0].HasUnmodeledGroups() {
			t.Error("Decode() HasUnmodeledGroups() = false after a null entry, want true")
		}
	})

	t.Run("null-only updates list decodes empty", func(t *testing.T) {
		t.Parallel()

		dec, err := dependabot.Decode([]byte("version: 2\nupdates:\n  - null\n  - null\n"))
		if err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		if len(dec.Config.Updates) != 0 {
			t.Errorf("Decode() updates = %d, want 0", len(dec.Config.Updates))
		}

		if dec.Unsafe {
			t.Error("Decode() unsafe = true for null-only updates, want false")
		}
	})

	t.Run("scalar entry fails the parse", func(t *testing.T) {
		t.Parallel()

		_, err := dependabot.Decode([]byte("version: 2\nupdates:\n  - just-a-string\n"))
		if err == nil {
			t.Error("Decode() error = nil for a scalar updates entry, want unparseable")
		}
	})

	t.Run("mapping updates fail the parse", func(t *testing.T) {
		t.Parallel()

		_, err := dependabot.Decode([]byte("version: 2\nupdates:\n  package-ecosystem: gomod\n"))
		if err == nil {
			t.Error("Decode() error = nil for mapping-form updates, want unparseable")
		}
	})
}

func TestEncodeRoundTrip(t *testing.T) {
	t.Parallel()

	dec, err := dependabot.Decode([]byte(strings.Join([]string{
		"version: 2",
		"updates:",
		"  - package-ecosystem: gomod",
		"    directory: /",
		"    schedule:",
		"      interval: weekly",
		"    open-pull-requests-limit: 5",
		"    groups:",
		"      minor-and-patch:",
		"        update-types:",
		"          - minor",
		"          - patch",
		"",
	}, "\n")))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	first, err := dec.Config.Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	reDecoded, err := dependabot.Decode(first)
	if err != nil {
		t.Fatalf("re-Decode() error = %v", err)
	}

	second, err := reDecoded.Config.Encode()
	if err != nil {
		t.Fatalf("second Encode() error = %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("round-trip not stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	if string(first) != strings.Join([]string{
		"version: 2",
		"updates:",
		"  - package-ecosystem: gomod",
		"    directory: /",
		"    schedule:",
		"      interval: weekly",
		"    open-pull-requests-limit: 5",
		"    groups:",
		"      minor-and-patch:",
		"        update-types:",
		"          - minor",
		"          - patch",
		"",
	}, "\n") {
		t.Errorf("canonical output mismatch:\n%s", first)
	}
}

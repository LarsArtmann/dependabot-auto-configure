package dependabot_test

import (
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

func TestDecodeCanonicalConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		yaml           string
		wantUnsafe     bool
		wantUnknownTop []string
		wantGroups     bool
		wantUpdates    int
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
			name:           "unknown top-level key is audited unsafe",
			yaml:           "version: 2\nregistries:\n  npm: {}\nupdates: []\n",
			wantUnsafe:     true,
			wantUnknownTop: []string{"registries"},
		},
		{
			name:        "unknown entry field is audited unsafe",
			yaml:        "version: 2\nupdates:\n  - package-ecosystem: npm\n    directory: /\n    assignees:\n      - someone\n",
			wantUnsafe:  true,
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
			wantUnsafe:  false,
			wantUpdates: 1,
		},
		{
			name: "unknown group name is audited unsafe",
			yaml: `version: 2
updates:
  - package-ecosystem: npm
    directory: /
    groups:
      everything:
        patterns:
          - "*"
`,
			wantUnsafe:  true,
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

			if dec.Unsafe != tt.wantUnsafe {
				t.Errorf("Decode() unsafe = %v, want %v", dec.Unsafe, tt.wantUnsafe)
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
// must carry the audit flag distinguishing "has groups this tool cannot
// see" from "has no groups at all" — per entry, not per document.
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

	if !dec.Unsafe {
		t.Error("Decode() Unsafe = false for custom group names, want true (rewrite would drop them)")
	}

	if len(dec.UnknownGroupNames) != 1 || dec.UnknownGroupNames[0] != "gomod" {
		t.Errorf("Decode() UnknownGroupNames = %v, want [gomod]", dec.UnknownGroupNames)
	}

	if len(dec.Config.Updates) != 2 {
		t.Fatalf("Decode() updates = %d, want 2", len(dec.Config.Updates))
	}

	if !dec.Config.Updates[0].HasUnmodeledGroups {
		t.Error("Decode() entry 0 HasUnmodeledGroups = false, want true (groups mapping with unknown names)")
	}

	if dec.Config.Updates[1].HasUnmodeledGroups {
		t.Error("Decode() entry 1 HasUnmodeledGroups = true, want false (no groups mapping at all)")
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

			if got := dec.Config.Updates[0].HasUnmodeledGroups; got != tt.wantFlag {
				t.Errorf("Decode() HasUnmodeledGroups = %v, want %v", got, tt.wantFlag)
			}
		})
	}
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

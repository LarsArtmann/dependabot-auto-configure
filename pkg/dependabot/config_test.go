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

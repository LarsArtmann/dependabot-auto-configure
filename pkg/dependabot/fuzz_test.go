package dependabot_test

import (
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

// FuzzDecode pins the parse boundary: arbitrary bytes never panic Decode,
// and the Unsafe verdict always agrees with the individual audit signals.
func FuzzDecode(f *testing.F) {
	f.Add([]byte("version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n"))
	f.Add([]byte("version: 2\nupdates: []\n"))
	f.Add(
		[]byte(
			"version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n    registries-are-not-modeled: true\n",
		),
	)
	f.Add([]byte("version: 2\nregistries:\n  - type: npm-registry\n"))
	f.Add(
		[]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
      rebase-strategy: auto
`),
	)
	f.Add([]byte(""))
	f.Add([]byte("version: [2\nupdates: }}"))
	f.Add([]byte("\x00\x01\x02\xff"))
	f.Add([]byte("version: 2\nupdates: not-a-list\n"))
	f.Add([]byte(listFormGroupsYAML))
	f.Add([]byte(commitMessageYAML))
	f.Add([]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      gomod:
        patterns:
          - "*"
`))
	f.Add(
		[]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    commit-message:
      prefix: chore(deps)
      include: scope
      separator: ": "
    groups:
      - minor-and-patch
`),
	)
	f.Add(
		[]byte(`version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      minor-and-patch:
        update-types:
          - minor
        exclude-patterns:
          - go.mod
`),
	)
	f.Add(
		[]byte(`version: 2
updates:
  - null
  - package-ecosystem: gomod
    directory: /
    groups:
      custom:
        patterns:
          - "*"
`),
	)
	f.Add(
		[]byte(
			"version: 2\nx-common: &common\n  a: 1\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n    extra: *common\n",
		),
	)
	f.Add(
		[]byte(
			"version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n    sched2:\n      <<: {interval: weekly}\n",
		),
	)
	f.Add(
		[]byte(
			"version: 2\nupdates:\n  - package-ecosystem: github-actions\n    directories:\n      - \"/\"\n      - \".github/actions/*/*\"\n",
		),
	)

	f.Fuzz(func(t *testing.T, data []byte) {
		res, err := dependabot.Decode(data)
		if err != nil {
			return
		}

		// The Unsafe verdict must always agree with the individual reasons,
		// and a safe decode must round-trip verbatim through Encode.
		if res.Unsafe != (len(res.UnsafeReasons) > 0) {
			t.Fatalf("Unsafe = %v but reasons = %v for input %q", res.Unsafe, res.UnsafeReasons, data)
		}

		if res.Unsafe {
			return
		}

		encoded, encErr := res.Config.Encode()
		if encErr != nil {
			t.Fatalf("Encode() error = %v for input %q", encErr, data)
		}

		again, decErr := dependabot.Decode(encoded)
		if decErr != nil {
			t.Fatalf("re-Decode() error = %v for input %q, output %q", decErr, data, encoded)
		}

		if again.Unsafe {
			t.Fatalf(
				"safe config re-decoded unsafe, reasons = %v for input %q, output %q",
				again.UnsafeReasons,
				data,
				encoded,
			)
		}

		if !dependabot.Equal(res.Config, again.Config) {
			t.Fatalf("round-trip changed the config for input %q, output %q", data, encoded)
		}
	})
}

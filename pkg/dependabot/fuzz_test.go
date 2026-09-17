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
		[]byte(
			"version: 2\nupdates:\n  - package-ecosystem: gomod\n    directory: /\n    schedule:\n      interval: weekly\n      rebase-strategy: auto\n",
		),
	)
	f.Add([]byte(""))
	f.Add([]byte("version: [2\nupdates: }}"))
	f.Add([]byte("\x00\x01\x02\xff"))
	f.Add([]byte("version: 2\nupdates: not-a-list\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		res, err := dependabot.Decode(data)
		if err != nil {
			return
		}

		unsafeSignals := len(res.UnknownTopLevel) > 0 ||
			res.UnknownEntryFields ||
			len(res.UnknownGroupNames) > 0

		if res.Unsafe != unsafeSignals {
			t.Fatalf("Unsafe = %v but audit signals = %v for input %q", res.Unsafe, unsafeSignals, data)
		}
	})
}

package dependabot_test

import (
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

// listFormGroupsYAML uses the non-standard list form of `groups:` seen in
// real hand-written configs (e.g. Code-To-CV-Agent). Decode must succeed
// (so findings flow) but mark the document unsafe, because a rewrite would
// silently replace the list with the canonical mapping shape.
const listFormGroupsYAML = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    groups:
      - minor-and-patch
`

func TestDecodeListFormGroupsUnsafeNotError(t *testing.T) {
	t.Parallel()

	res, err := dependabot.Decode([]byte(listFormGroupsYAML))
	if err != nil {
		t.Fatalf("Decode() error = %v, want list-form groups to decode (audited unsafe instead)", err)
	}

	if !res.Unsafe {
		t.Error("Decode() Unsafe = false for list-form groups, want unsafe (rewrite would drop the shape)")
	}

	if len(res.Config.Updates) != 1 {
		t.Fatalf("Decode() updates = %d, want 1", len(res.Config.Updates))
	}
}

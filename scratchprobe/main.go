// Package main is a temporary observation probe; deleted after the session.
package main

import (
	"fmt"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
)

func probe(name, data string) {
	res, err := dependabot.Decode([]byte(data))
	fmt.Printf("=== %s ===\n", name)
	fmt.Printf("err=%v updates=%d unsafe=%v unknownEntryFields=%v unknownGroups=%v\n",
		err, len(res.Config.Updates), res.Unsafe, res.UnknownEntryFields, res.UnknownGroupNames)

	for i, u := range res.Config.Updates {
		fmt.Printf("  entry[%d] eco=%q dir=%q groups=%+v flagged=%v\n",
			i, u.PackageEcosystem, u.Directory, u.Groups, u.HasUnmodeledGroups)
	}
}

func main() {
	probe("null entry in updates list", `version: 2
updates:
  - null
  - package-ecosystem: gomod
    directory: /
    groups:
      custom:
        patterns:
          - "*"
`)

	probe("scalar entry in updates list", `version: 2
updates:
  - just-a-string
  - package-ecosystem: gomod
    directory: /
`)

	probe("updates is a mapping", `version: 2
updates:
  package-ecosystem: gomod
`)

	probe("canonical group value is a list", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      minor-and-patch:
        - minor
`)

	probe("canonical group value is a scalar", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      actions: "*"
`)

	probe("unknown key inside canonical minor-and-patch", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      minor-and-patch:
        update-types:
          - minor
          - patch
        exclude-patterns:
          - go.mod
`)

	probe("unknown key inside canonical actions", `version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    groups:
      actions:
        patterns:
          - "*"
        applies-to:
          - security-updates
`)

	probe("unknown group name with null value", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      gomod: null
`)

	probe("canonical group with null value", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      actions: null
`)

	probe("mixed canonical and custom names", `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    groups:
      minor-and-patch:
        update-types:
          - minor
      gomod:
        patterns:
          - "*"
`)
}

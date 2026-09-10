# TODO List

> Short-term, actionable, bounded work items, verified against the actual code.
> For long-term vision and unrefined ideas, use ROADMAP.md.
> Items are ranked by impact. Status is verified, not assumed.

## Status legend

| Status           | Meaning                                                     |
| ---------------- | ----------------------------------------------------------- |
| 🔴 `TODO`        | Not started. Needs doing.                                   |
| 🟡 `IN_PROGRESS` | Actively being worked on.                                   |
| 🔵 `BLOCKED`     | Cannot proceed, external dependency or decision needed.     |
| 🟢 `DONE`        | Completed. Remove from this list and log in `CHANGELOG.md`. |

## High Impact

| Task                                                                                                                                                   | Status    | Impact | Effort | Evidence                                                                   |
| ------------------------------------------------------------------------------------------------------------------------------------------------------ | --------- | ------ | ------ | -------------------------------------------------------------------------- |
| Make the public install path real: the flake depends on the private `BuildFlow` repo over SSH, so `nix run github:...` fails for anyone without access | 🔴 `TODO` | High   | 2-4h   | `flake.nix:38` (`git+ssh://` to private repo), `README.md` Install section |
| Add CI (build + test + lint) for the now-public GitHub repo                                                                                            | 🔴 `TODO` | High   | 1-2h   | No `.github/workflows/` exists in this repo                                |

## Medium Impact

| Task                                                                                | Status    | Impact | Effort | Evidence                                                                                    |
| ----------------------------------------------------------------------------------- | --------- | ------ | ------ | ------------------------------------------------------------------------------------------- |
| Add CLI-layer tests (exit codes, flag wiring, output lines)                         | 🔴 `TODO` | Med    | 1-2h   | `internal/cli` has no test files; exit codes 0/1/2 are untested (`internal/cli/root.go:16`) |
| Cover the BuildFlow provider with a test (Detect check-mode, Repair dry-run)        | 🔴 `TODO` | Med    | 1h     | `pkg/provider` coverage is 0% (`go test -cover ./pkg/...`)                                  |
| Drop the local `replace` and pin the SDK version now that it is on the module proxy | 🔴 `TODO` | Med    | 30min  | `go.mod:49` (`replace ... => ../linter-autoconfigure-sdk`); SDK resolves on the proxy       |

---

<!-- Guidance for the builder filling this in:
  - Source of truth is the CODE. Verify each item before adding, many
    documented TODOs are already done.
  - One task per row. If it takes more than ~2 hours, split it into smaller
    tasks.
  - Cite evidence (file:line) so the next person can verify without re-deriving.
  - DONE items should be REMOVED, not kept. Use CHANGELOG.md for history.
  - If a task is vague ("improve X"), refine it into concrete steps or move
    it to ROADMAP.md.
  - Deduplicate by semantic intent, not by text match.
  - For 80/20 impact prioritization, use the pareto-planning skill AFTER
    building the list here.
-->

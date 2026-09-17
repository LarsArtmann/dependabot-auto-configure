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

| Task                                                                                                                                           | Status       | Impact | Effort | Evidence                                                                                                                     |
| ---------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------ | ------ | ---------------------------------------------------------------------------------------------------------------------------- |
| Push `master` to origin and verify: CI green on real runners (incl. the new windows matrix entry), Dependabot PR rebase, README badges resolve | 🔵 `BLOCKED` | High   | 15m    | origin/master (`f85ca82`) is CI-RED (vendorHash + lint); local HEAD carries the fixes — pushing needs explicit authorization |

## Medium Impact

| Task                                                                                                                                                 | Status       | Impact | Effort | Evidence                                                                                                                                    |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Cut `linter-autoconfigure-sdk` v1.0.0: sweep its deps first (go-finding v1.10.0 → v1.11.0, go-atomic-write v0.5.1 → v0.5.2), run its gates, then tag | 🔵 `BLOCKED` | Med    | 1h     | `~/projects/linter-autoconfigure-sdk` go.mod is one release behind; 100% coverage, clean tree, tagging an external repo needs authorization |

## Low Impact

| Task                                                                                  | Status    | Impact | Effort | Evidence                                                                                                                 |
| ------------------------------------------------------------------------------------- | --------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------ |
| `--json`: decide whether the `--fail-on` policy outcome belongs in the wire shape     | 🔴 `TODO` | Low    | 30m    | `resultJSON` in `pkg/configure/configure.go`; policy verdict currently text-mode only; wire keys are contract-tested now |
| Upstream to go-nix-helpers: apps lack `meta.description` (`nix flake check` warnings) | 🔴 `TODO` | Low    | 30m    | Warning on every app in `nix flake check --all-systems`; module-owned, not per-repo                                      |

---

<!-- Guidance for the builder filling this in:
  - Source of truth is the CODE. Verify each item before adding, many
    documented TODOs are already done.
  - One task per row. If it takes more than ~2 hours, split it into smaller
    tasks.
  - Cite evidence (file:line) so the next person can verify without re-deriving.
  - DONE items should be REMOVED, not kept. Use CHANGELOG.md for history.
  - If a task is vague ("improve X"), refine it into concrete steps or move it
    to ROADMAP.md.
  - Deduplicate by semantic intent, not by text match.
  - For 80/20 impact prioritization, use the pareto-planning skill AFTER
    building the list here.
-->

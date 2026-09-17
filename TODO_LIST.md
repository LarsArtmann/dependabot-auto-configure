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

| Task                                                                                                                                | Status       | Impact | Effort | Evidence                                                                                      |
| ----------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------ | ------ | --------------------------------------------------------------------------------------------- |
| Push `master` to origin and verify: CI green on real runners, Dependabot picks up the `github-actions` entry, README badges resolve | 🔵 `BLOCKED` | High   | 15m    | ~10+ commits ahead of origin; pushing needs explicit authorization. CI has never run for real |

## Medium Impact

| Task                                                                                                                                                 | Status       | Impact | Effort | Evidence                                                                                                                                    |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Raise `pkg/configure` (69.6%) and `internal/cli` (65.1%) coverage to the 80% target (AGENTS.md §Testing)                                             | 🔴 `TODO`    | Med    | 2-4h   | `go test ./... -cover`; `github.go` request paths and `root.go` report rendering are the thin spots                                         |
| Add a Windows CI job — the slash-separator bug (v0.2.0-era) proved Windows matters and CI is Linux-only today                                        | 🔴 `TODO`    | Med    | 1h     | `.github/workflows/ci.yml` has no `windows-latest` matrix entry                                                                             |
| Cut `linter-autoconfigure-sdk` v1.0.0: sweep its deps first (go-finding v1.10.0 → v1.11.0, go-atomic-write v0.5.1 → v0.5.2), run its gates, then tag | 🔵 `BLOCKED` | Med    | 1h     | `~/projects/linter-autoconfigure-sdk` go.mod is one release behind; 100% coverage, clean tree, tagging an external repo needs authorization |

## Low Impact

| Task                                                                                  | Status    | Impact | Effort | Evidence                                                                              |
| ------------------------------------------------------------------------------------- | --------- | ------ | ------ | ------------------------------------------------------------------------------------- |
| Cache golangci-lint in CI (`golangci-lint-action` `cache: true` or `actions/cache`)   | 🔴 `TODO` | Low    | 15m    | Cold lint on real runners is minutes; `.github/workflows/ci.yml` lint job             |
| `--json`: decide whether the `--fail-on` policy outcome belongs in the wire shape     | 🔴 `TODO` | Low    | 30m    | `resultJSON` in `pkg/configure/configure.go`; policy verdict currently text-mode only |
| Upstream to go-nix-helpers: apps lack `meta.description` (`nix flake check` warnings) | 🔴 `TODO` | Low    | 30m    | Warning on every app in `nix flake check --all-systems`; module-owned, not per-repo   |

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

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

| Task                                                                                                                                    | Status    | Impact | Effort | Evidence                                                                     |
| --------------------------------------------------------------------------------------------------------------------------------------- | --------- | ------ | ------ | ---------------------------------------------------------------------------- |
| Add CI (`.github/workflows/`): `nix flake check`, `go test ./...`, golangci-lint, dprint check, coverage summary; add README badge after | 🔴 `TODO` | High   | 1-2h   | No workflows dir in repo; vendorHash rot already happened once without CI    |
| Create the GitHub Release for tag `v0.1.0` from CHANGELOG (tag exists, release missing)                                                 | 🔴 `TODO` | Med    | 15m    | `gh release list` → empty; tag `v0.1.0` at `f80557d`                         |

## Medium Impact

| Task                                                                        | Status    | Impact | Effort | Evidence                                                                                  |
| --------------------------------------------------------------------------- | --------- | ------ | ------ | ----------------------------------------------------------------------------------------- |
| Cover the BuildFlow provider with a test (Detect check-mode, Repair dry-run) | 🔴 `TODO` | Med    | 1h     | `pkg/provider` coverage is 0% (`go test -cover ./pkg/...`)                               |
| Pin a repo-owned `.golangci.yml` so lint results do not depend on machine config | 🔴 `TODO` | Med | 30m    | No `.golangci.yml` exists; `pkg/provider` import order drifted past gofmt (fixed by hand) |
| Add `--fail-on <severity>` flag for CI policy control                       | 🔴 `TODO` | Med    | 2-4h   | No such flag; `internal/cli/root.go` has only `--check` severity-blind exit                |
| Write `docs/adr/0001-suggest-only-unsafe-configs.md` (the core safety decision) | 🔴 `TODO` | Med  | 1h     | No `docs/adr/`; contract documented in AGENTS/FEATURES but not as an ADR                  |
| README: show a generated `dependabot.yml` example block                     | 🔴 `TODO` | Med    | 30m    | README `What it does` describes output but never shows it                                |

## Low Impact

| Task                                                                          | Status    | Impact | Effort | Evidence                                                            |
| ----------------------------------------------------------------------------- | --------- | ------ | ------ | ------------------------------------------------------------------- |
| Make `nix flake check` pass for `--all-systems` (aarch64/darwin, x86_64-darwin) | 🔴 `TODO` | Low    | 1h     | `nix flake check` warns: incompatible systems omitted             |
| Windows robustness: test `detect.Shape` path handling with Windows separators  | 🔴 `TODO` | Low    | 1h     | `pkg/detect/detect.go` uses `filepath` separators, untested on `\` |
| Fuzz `dependabot.Decode` with arbitrary YAML (it is the parse boundary)        | 🔴 `TODO` | Low    | 2-4h   | No fuzz targets in repo                                             |
| Property test: `Reconcile` is idempotent (`Reconcile(r, d) == r`)              | 🔴 `TODO` | Low    | 1h     | No property tests in repo                                           |
| Integration test: atomic-write failure path (read-only dir) in `planOrWrite`   | 🔴 `TODO` | Low    | 1h     | `planOrWrite` error branch uncovered                                |
| Decide exit-code semantics for unsafe configs (currently 0; maybe a distinct code) | 🔴 `TODO` | Low | 30m    | `internal/cli/root.go` exit codes 0/1/2; unsafe exits 0             |
| Add `.github/ISSUE_TEMPLATE` + PR template                                    | 🔴 `TODO` | Low    | 30m    | Public repo, no templates                                           |
| Set GitHub repo topics (`go`, `dependabot`, `devtools`, `nix`); description is set, topics are empty | 🔴 `TODO` | Low | 5m | `gh api repos/.../topics` → empty                     |
| Audit sibling dep pins (go-finding, go-atomic-write, go-error-family) for stale versions | 🔴 `TODO` | Low | 30m | `flake.nix` deps pinned at v1.10.0/v0.5.1/v0.10.0                |
| Error-message audit against the what/why/fix standard for all finding suggestions | 🔴 `TODO` | Low | 1h  | `pkg/dependabot/generate.go` finding texts never audited           |
| Add SECURITY.md (private disclosure contact)                                  | 🔴 `TODO` | Low    | 30m    | Public repo, no SECURITY.md                                         |
| Tag `linter-autoconfigure-sdk` `v1.0.0` (external repo) so consumers pin stable | 🔴 `TODO` | Low   | 30m    | SDK at v0.1.0 (`go.mod`); this repo is its first consumer          |

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

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

| Task                                                                                                                                                    | Status       | Impact | Effort | Evidence                                                                                                                                                                                      |
| ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Ship v0.4.0: `git tag v0.4.0`, push master + tags, `gh release create` reusing the CHANGELOG 0.4.0 section                                              | 🔵 `BLOCKED` | High   | 15m    | Tags stop at v0.3.0; local master ahead 5; origin CI red on the Lint job ONLY (run 44172371677 — the 3 wrapcheck findings, fixed locally in `.golangci.yml`); pushing needs explicit owner OK |
| (repo: BuildFlow) Repin the `dependabot-auto-configure` flake input to v0.4.0, refresh vendorHash (`nix run .#update-vendor-hash`), `nix build`         | 🔵 `BLOCKED` | High   | 15m    | BuildFlow `tools/providers/sdk_imports.go` blank-imports the provider; gate on the v0.4.0 tag (docs/status/2026-11-17_21-26 report §f item 2)                                                 |
| (repo: BuildFlow) Fleet rollout `buildflow -s dependabot-auto-configure --fix`, then re-run `scripts/sweep.sh` and record post-fix counts by the report | 🔵 `BLOCKED` | High   | 30m    | Sweep measured 34 unsafe → 1 pre-rollout; the rollout is where the warnings actually disappear (report §f items 3, 30)                                                                        |
| (repo: Code-To-CV-Agent) Decide fix-forward vs leave for the list-form `groups:` (the 1 remaining unsafe repo)                                          | 🔴 `TODO`    | Med    | 15m    | `DecodeResult.UnsafeReasons` reports "groups must be a mapping"; file is suggest-only forever until reshaped (report §f item 4)                                                               |

## Medium Impact

| Task                                                                                                                                                 | Status       | Impact | Effort | Evidence                                                                                                                                    |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ | ------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Surface `UnsafeReasons` in the suggest-only finding message: reasons exist in `DecodeResult` but users never see why repair held back                | 🔴 `TODO`    | Med    | 30m    | `pkg/configure/configure.go` `Run`: `dec.Unsafe` → `UnsafeRepair=true` with no reason text (report §f item 12)                              |
| Head-comment preservation for unmodeled keys (capture key nodes too); value-side line comments already survive                                       | 🔴 `TODO`    | Med    | 2h     | `pkg/dependabot/config.go` inline `Unmodeled` maps capture value nodes only; ADR 0002 documents the loss (report §f item 14)                |
| Cut `linter-autoconfigure-sdk` v1.0.0: sweep its deps first (go-finding v1.10.0 → v1.11.0, go-atomic-write v0.5.1 → v0.5.2), run its gates, then tag | 🔵 `BLOCKED` | Med    | 1h     | `~/projects/linter-autoconfigure-sdk` go.mod is one release behind; 100% coverage, clean tree, tagging an external repo needs authorization |

## Low Impact

| Task                                                                                                                                                 | Status    | Impact | Effort | Evidence                                                                                                     |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | --------- | ------ | ------ | ------------------------------------------------------------------------------------------------------------ |
| README: add a "your registries/ignore/reviewers ride along verbatim" example block with before/after                                                 | 🔴 `TODO` | Low    | 30m    | README bullets describe passthrough but show no example (report §f item 17)                                  |
| `scripts/sweep.sh`: per-rule CSV columns (rule,count per repo)                                                                                       | 🔴 `TODO` | Low    | 30m    | Current CSV has aggregate `findings` only (report §f item 18)                                                |
| Alias edge tests: anchor defined in unmodeled + alias used in a typed field (safe today — pin it); duplicate keys (last-wins) audit policy           | 🔴 `TODO` | Low    | 1h     | `pkg/dependabot/passthrough_test.go` covers alias-unsafe, not the safe anchor+typed-alias combo (§f item 20) |
| Naming pass on v0.4.0 identifiers (`HasUnmodeledGroups()` vs `HasCustomGroups()`, ...) via the naming-review approach                                | 🔴 `TODO` | Low    | 30m    | New API surface in `pkg/dependabot/config.go` unreviewed for names (§f item 21)                              |
| Collapse `normalizeStrings`/`normalizeUpdateTypes` into one generic helper                                                                           | 🔴 `TODO` | Low    | 15m    | `pkg/dependabot/generate.go` two near-identical normalizers (§f item 22)                                     |
| (repo: golangci-lint-auto-configure, repo: oxlint-auto-configure) Port the verbatim-passthrough audit if they still enumerate unknown keys as unsafe | 🔴 `TODO` | Low    | 2h     | Fleet noise win; siblings share the error-family/typed-error pattern (§f item 23)                            |
| Add a sanitized paperless-ngx-shaped regression fixture explicitly (configure tests pin it implicitly)                                               | 🔴 `TODO` | Low    | 30m    | `pkg/configure/configure_test.go` directories cases derive from it (§f item 16)                              |
| (repo: BuildFlow) E2E one fixture repo through BuildFlow to confirm the entry-invalid warning severity behaves under the findings gate               | 🔴 `TODO` | Low    | 1h     | Severity downgrade is pinned locally, not under the real gate (§f item 19)                                   |

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

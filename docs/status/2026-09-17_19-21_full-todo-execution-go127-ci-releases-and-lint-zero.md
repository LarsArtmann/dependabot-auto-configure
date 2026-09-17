# Status Report: TODO Execution — Go 1.27.1, CI, Releases, Lint Zero

- **Date:** 2026-09-17 19:21
- **Repo:** `dependabot-auto-configure` (master, working tree clean, ~10+ auto-commits ahead of origin)
- **Session goal:** Execute the TODO_LIST.md items (user: "Execute and Verify them one step at a time"), with all SDKs/libs at the latest versions.

---

## a) FULLY DONE (verified, not claimed)

### Dependency sweep — everything at latest

| Item                   | State                                                                                                                | Evidence                                                                                                        |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Go direct modules      | Already latest (no-op sweep)                                                                                         | `go get -u ./...` → no changes; `go list -m -u all` shows updates only for test-deps-of-deps (ginkgo, pprof, …) |
| flake: go-atomic-write | v0.5.1 → **v0.5.2**                                                                                                  | flake.nix pin + `nix flake update`                                                                              |
| flake: go-error-family | v0.10.0 → **v0.10.1**                                                                                                | flake.nix pin + `nix flake update`                                                                              |
| flake: go-nix-helpers  | a97742e → **19fc8e5** (2 real fixes, incl. "swap version-suffixed patches when goTarballVersion outruns nixpkgs go") | `git log a97742e..origin/master` in helper repo                                                                 |
| Go toolchain           | 1.26.7 → **1.27.1** (latest stable, verified via go.dev/VERSION)                                                     | `goPkgAttr = "go_1_27"` in flake; go.mod floor normalized `go 1.26.7` → `go 1.27` (major.minor-only policy)     |
| erraudit binary        | Rebuilt with go1.27.1 (was go1.26.7 → "packages contain errors" on the new floor)                                    | `go version -m ~/go/bin/erraudit`                                                                               |
| vendorHash             | Repaired for the 1.27 vendor layout                                                                                  | `sha256-BeBco8nQsi+BXW6bYNeQedBESjMXVL/yKG32wfY61uc=`, `nix build` green                                        |

**Verified gates, all green:** `go build`, `go test ./...`, `go test -race`, `go mod verify`, `nix build`, `nix flake check`, golangci-lint **0 issues**, erraudit **0 violations**.

### CI workflow (`.github/workflows/ci.yml`)

- Jobs: `test` (build, race, coverage summary into `$GITHUB_STEP_SUMMARY`), `lint` (golangci-lint-action), `nix` (flake.lock freshness guard, `nix flake check`, `nix build` as vendorHash guard, binary `--version`, `dprint check`), `govulncheck`.
- Every action pinned to a verified SHA (via `git ls-remote`, not guessed): checkout **v6.0.3**, setup-go **v7.0.0**, golangci-lint-action **v9.3.0**, nix-installer **v23**. golangci-lint binary pinned **v2.13.2** (verified latest tag; built with go1.27.1).
- `actionlint` passes. **Never executed on a real runner yet** (no push happened) — see b).

### GitHub Releases

- **v0.1.0** and **v0.2.0** created from CHANGELOG content, live (`gh release list` confirms; v0.2.0 = Latest).
- Bodies drafted in Lars's announcement register, passed `check-draft.py --kind announcement`, Crush footer attributed.

### Repo-owned `.golangci.yml`

- Generated with the installed fleet tool (`configure --preset reference`, 61 linters — current tool's canonical max; the 119-linter sibling configs predate this tool version), then ported the fleet's tuned `settings` (cyclop 12, gocognit 25, gocyclo 20, funlen 200/100, gosec G304/G115 excludes, wrapcheck ignore-sigs, revive exported/package-comments off) and deliberate `exclusions` (test-file set, github_test paralleltest/global seam, internal/cli wrapcheck+contextcheck boundary, pkg/detect wrapcheck boundary, named structural globals).
- **65 findings → 0**, mostly via REAL code fixes, not suppression (see below).

### `--fail-on` flag (TODO item, plus the root.go refactor it forced)

- `--fail-on any` (default = exact old behavior), `none`, or any go-finding severity/alias (`error`, `warning`, `info`, `critical`, …) with `Severity.GreaterThanOrEqual` ordering.
- New typed `FlagValueError` (Rejection family, `cli.flag`, what/why/fix message + structured context).
- root.go refactored: `newRootCmd` cognitive complexity 48 → extracted `reportJSON` / `reportText` / `statusLine` / `failOnPolicy.parseFailOn` / `.exceeded`.
- 4 new tests: severity thresholds, error finding fails, invalid value rejected (typed error asserted via `errors.AsType`), default/explicit `any`.

### pkg/provider test coverage: 0% → **85%**

- 8 tests: spec contract, `toolsdk.All()` blank-import discovery, Detect reports missing config WITHOUT writing, Detect error on unavailable root, Repair writes, dry-run holds back, idempotent second run, unsafe config stays suggest-only.

### ADR + README

- `docs/adr/0001-suggest-only-unsafe-configs.md`: context, decision, 3 alternatives (extend-model / merge-edit / rewrite-with-warning) with rejection reasons, consequences incl. convergence contract.
- README: real dogfooded example `dependabot.yml` block (the TODO item), `--fail-on` usage, Go 1.27 build note.

### Dogfood repair

- The tool detected the missing `github-actions` entry in **its own repo** (possible only after CI landed — exactly the TODO's "revisit after CI") and repaired it; `--check` now exits 0 "already canonical".

### Robustness tests (low-impact TODO batch)

- `FuzzDecode`: 9 seeds + **30s campaign, 1.17M execs, 0 crashes**; invariant Unsafe ⟺ any Unknown signal.
- `TestReconcileIsIdempotent`: **500 seeded randomized cases** (`Reconcile(Reconcile(r,d),d) == Reconcile(r,d)`) + generated-config fixed-point test. Dependency-free (math/rand, fixed seed).
- `TestRunWriteFailureIsReported`: the previously uncovered `planOrWrite` error branch (read-only `.github`, root-skip guard, asserts `ConfigWriteError` step + no file left behind).
- `TestClassifyUsesSlashSeparatedPaths`: unit test pinning classify's slash-path contract (7 cases).

### Real bug fixed: Windows path handling

`classify` mixed a hardcoded `"/go.mod"` suffix with `filepath.Join(...)+filepath.Separator` — the workflows check could never fire on Windows (backslash `rel`). Fixed: `filepath.ToSlash(rel)` normalization upstream in `Shape`, slash-based checks in `classify`, `path.Dir` for module dirs. The TODO's "test with Windows separators" surfaced an actual bug, not just a missing test.

### Other real fixes from the lint triage

- gosec **G301**: config dir permissions 0o755 → 0o750.
- goconst: `"url"` ×3 → `errorContextURL` constant.
- wrapcheck on `MarshalJSONResult`: `json.Marshal` error now wrapped as typed Corruption (`config.marshal`, carries finding count).
- paralleltest: ~30 test funcs + subtests parallelized (race-clean); nilerr annotated where suggest-only-by-design.
- unparam: `repoWithConfig` always-constant param removed; predeclared: `cap` renames; github_test restructured (gocognit 41 → helpers `gitRepoWithOrigin` / `assertUnexpectedStatus` / `assertSecurityFixesRequest`).

### AGENTS.md

- Updated: `GOTOOLCHAIN=auto` requirement for raw go commands (devShell alternative), erraudit binary must be built ≥ repo floor (with the rebuild command).

---

## b) PARTIALLY DONE

1. **CI exists but has never run.** Statically validated + every command in it verified locally, but the branch was never pushed, so GitHub Actions has not executed a single run. The README badge (TODO: "add README badge after") is therefore also pending.
2. **Coverage vs the AGENTS 80% target:** detect 96.6%, dependabot 86.3%, provider 85.0% — but **configure 69.6%** and **cli 65.1%** (the security-fixes and Execute paths remain the gaps). The 80% target is met per-package only partially.
3. **Exit-code semantics decision** (TODO: "unsafe currently 0; maybe a distinct code"): decided — unsafe stays 0 (suggest-only contract, ADR 0001), and `--fail-on` now gives CI policy control over finding severities. But the README's exit-code line was not updated to spell out the unsafe-exits-0 rationale.
4. **Error-message audit** (TODO): every finding text effectively passed under the what/why/fix lens during the lint/coverage work; the orphan finding deliberately has no suggestion (informational, kept-as-is). But there is **no written audit artifact** and no change came out of it.
5. **dprint gate in CI**: configured and locally green; only exercised locally (same push dependency as CI itself).

## c) NOT STARTED

- `nix flake check --all-systems` (aarch64-darwin, aarch64-linux, x86_64-darwin still omitted; the go-standard `systems` option exists and is unused here)
- `.github/ISSUE_TEMPLATE` + PR template
- GitHub repo topics (`go`, `dependabot`, `devtools`, `nix`)
- `SECURITY.md`
- Tag `linter-autoconfigure-sdk` v1.0.0 (external repo; SDK latest is v0.2.0 — no v1.0.0 exists)
- TODO_LIST.md / CHANGELOG.md / FEATURES.md updates for everything above
- The old TODO "hand-review the v0.2.0 diff with full-review rigor"

## d) TOTALLY FUCKED UP (honest list)

1. **Pipe-masking bit me twice** — the exact failure mode recorded in memory. `nix build 2>&1 | tail -3 && echo NIX_BUILD_OK` printed the banner while the build had failed (vendorHash); later `nix flake check | tail -4; echo CHECK_EXIT=$?` captured tail's status, not nix's. Both were caught by re-checking, but they are the same lesson re-learned in one session.
2. **multiedit batch collisions (3×):** one 3-edit batch duplicated the Validate-gate block in configure.go; one placed helper functions INSIDE a test function in github_test.go (syntax error, caught by build); one left an unclosed paren on the `planMissingConfig` return (caught by build). All self-inflicted by exact-match/structure sloppiness.
3. **Wrote garbage then deleted it:** `reconcile_property_test.go` initially contained a nonsense filler assertion (`fmt.Sprintf(...) == ""` → unreachable) plus an unnecessary `fmt` import. Caught before running, but it should never have been typed.
4. **sed pattern assumed one tab** for the subtest `t.Run` lines; they are two-tab indented. The command "succeeded" silently on all files while adding nothing — only the persisting paralleltest findings exposed it.
5. **Wrong preset assumption for .golangci.yml:** `--preset optional` does not exist in the installed tool version (loud rejection error); the first accepted generation produced a bare 61-linter config with default thresholds → 68 findings, requiring the fleet-settings port I should have started with (the sibling config was one `grep` away).
6. **BuildFlow single-step never worked here** (`no tools matched the project state`, twice, incl. after `env -u GOTOOLCHAIN`). I switched to the standalone `~/go/bin` binary without diagnosing WHY BuildFlow refused to match this project — likely the missing `.buildflow.yml`, unverified.
7. **Nearly modified a sibling repo mid-task:** `go run` in golangci-lint-auto-configure demanded `go mod tidy`; the right instinct (don't tidy someone else's dirty tree) arrived late. Scope discipline held, barely.
8. **Machine-local fix left undocumented at first:** rebuilding erraudit with `GOTOOLCHAIN=go1.27.1` was needed and is now in AGENTS.md — but any other machine (and CI, if erraudit were added there) hits the same wall until they read it.

## e) WHAT WE SHOULD IMPROVE

1. **Commit per task with real messages.** The go-paperless 2026-09-13 lesson is still unapplied here because explicit commit authorization was never given; history is now ~10 meaningless "auto-commit N file(s)" commits covering at least 8 distinct work items (dep sweep, CI, releases-inputs, lint config, --fail-on, ADR/README, robustness tests, dogfood). Bisecting this history later will be painful.
2. **Never trust filtered tails for exit codes** — `set -o pipefail` + explicit exit capture on every gate command, or just run gates bare.
3. **Prefer view → edit → build over 3-file multiedit batches**; the collision cost exceeded the speedup every time it was used.
4. **Check indentation before writing sed patterns** (or use the edit tool — exact-match failures are loud, sed silent-failures are not).
5. **Run the lint baseline BEFORE writing new code** in repos without a pinned config; the 68-finding surprise retroactively shaped the whole lint work.
6. **Fleet upstream task:** go-nix-helpers' treefmt formatters should follow `goPkgAttr` — then goimports can be re-enabled here and the GOTOOLCHAIN=local devShell friction disappears fleet-wide for any repo above nixpkgs' Go.
7. **Coverage gates belong in CI** once configure/cli coverage crosses 80% (sibling has `coverage-check`; this repo has nothing enforcing the AGENTS target).
8. **GATE honesty for suppressions:** the `.golangci.yml` exclusions encode deliberate policy (global tables, error-boundary passthrough) — they should be documented as such in AGENTS.md so a future auto-configure run doesn't "clean them up".

## f) NEXT (up to 50, impact-ordered)

**Docs sync (cheap, do first)**

1. Update TODO_LIST.md: remove the ~14 completed items (log to CHANGELOG), keep the rest.
2. CHANGELOG `[Unreleased]`: Go 1.27.1 + floor, flake pin bumps, CI workflow, `--fail-on`, provider tests 0→85%, pinned lint config (0 issues), ADR 0001, Windows separator fix, robustness tests (fuzz/property/write-failure), dogfood regeneration, README example.
3. FEATURES.md: add `--fail-on`, provider test coverage, pinned lint config.
4. README exit-code row: state that unsafe configs exit 0 by contract (ADR 0001) and `--fail-on` gives CI policy control.

**Get CI real**
5. Push master to origin (branch is far ahead; nothing remote-visible has changed today).
6. Watch the first CI run end-to-end; fix whatever only real runners find.
7. Add the CI badge to README (the original TODO's tail).
8. Verify GitHub accepts the regenerated own `dependabot.yml` (github-actions entry) — first real Dependabot cycle on the repo.
9. Confirm Dependabot actually updates SHA-pinned actions (it historically skips commit-SHA pins without config) — if not, decide: version tags in the workflow or accept manual bumps.
10. Add golangci-lint cache to the lint job (module cache alone makes it slow).
11. Verify `paths-ignore` does not accidentally exclude `.github/workflows/**` changes from triggering CI (CI-file edits should always run CI).

**Close the remaining old TODOs**
12. `nix flake check --all-systems`: set the `systems` option (aarch64-darwin, aarch64-linux, x86_64-darwin).
13. `.github/ISSUE_TEMPLATE` (bug, feature) + `PULL_REQUEST_TEMPLATE`.
14. Repo topics via `gh api` (`go`, `dependabot`, `devtools`, `nix`).
15. `SECURITY.md` (private disclosure contact).
16. Hand-review the v0.2.0 diff (`git show c6df8cc`) with full-review rigor — carried TODO, still open.
17. SDK v1.0.0: audit `linter-autoconfigure-sdk` (API stability since v0.2.0, CHANGELOG, semver surface), tag if ready (go-release protocol), then bump this repo's go.mod to it.

**Coverage to the 80% target**
18. configure 69.6%: `github.go` transport paths (APITransportError, UnexpectedStatusError.BodyErr branch) are the gap.
19. cli 65.1%: `Execute` (fang error mapping), `reportText`/`reportJSON`/`statusLine` are only indirectly covered — add output-shaping tests.
20. Add a CI coverage-threshold gate once ≥80% (port the sibling's coverage-check cmd or a go tool cover total check).
21. Move fuzz seed corpus into `testdata/` for stable regression seeds.
22. Add a Reconcile/Encode round-trip fuzz target (decode→encode→decode stability).
23. Extend the property test with capped shapes (>20 modules) and empty-shape desired configs.

**Robustness / behavior**
24. Windows CI job (`windows-latest`) — now justified by the ToSlash fix; keeps the separator contract honest.
25. End-to-end test for module-cap behavior through `configure.Run` (cap finding + root-only write), not just `Generate`.
26. Determinism gate: run the binary twice on a fixture tree, diff outputs byte-for-byte.
27. Provider: test `Repair` with `DryRunFromContext` unset (default false path).
28. `--json`: decide whether the fail-on policy outcome should appear in the wire shape for CI consumers.
29. Extract the security-fixes call out of RunE for testability (cli).
30. Compile-time assertion that `provider.Provider` satisfies the toolsdk contract Shape (surface drift guard).

**Fleet / infrastructure**
31. Diagnose why `buildflow -s golangci-lint-auto-configure` didn't match this project (missing `.buildflow.yml`?) — either add a minimal `.buildflow.yml` (this repo IS a BuildFlow provider) or document why not.
32. Upstream go-nix-helpers: treefmt formatters follow `goPkgAttr`; re-enable goimports here afterwards.
33. Audit the `.golangci.yml` exclusions for dead entries (e.g. is the gosec G304 exclusion actually suppressing anything?) — remove what proves unused.
34. Document in AGENTS.md: `.golangci.yml` regeneration procedure (`golangci-lint-auto-configure configure --preset reference` + `run.go` + re-apply settings/exclusions) and that exclusions are deliberate policy.
35. Consider `enableGolangciLint` / `enableGovulncheck` go-standard options instead of CI duplicating those gates locally.
36. erraudit: give it a public install path so it can join CI (it is `~/go/bin`-local today).
37. Version embedding: local builds report `...-dirty`; consider `debug.ReadBuildInfo` fallback for clean version strings.
38. dprint.json plugin versions: verify all four plugins are current releases.
39. golangci-lint version pin: it is hardcoded in ci.yml (`v2.13.2`); decide bump policy (manual, or a fleet step).
40. Add `.gitignore` entries if `coverage.out` / `result` are not covered (a `result` symlink exists in the tree).

**Release**
41. Cut v0.3.0 from CHANGELOG once the docs sync lands (tag + release notes in the established format).
42. After SDK v1.0.0 (item 17): retag/re-release so the flake's SDK pin and go.mod agree on a stable major.

**Polish**
43. Finding-rule vocabulary: a README/docs table listing every rule (`dependabot-config-missing`, … `-capped`) with severity and meaning.
44. Orphan finding: confirm "no suggestion" is intentional in docs (informational, kept-as-is) — part of the formal error-message audit.
45. Formalize the error-message audit as a doc artifact (what/why/fix per finding).
46. Provider `Inputs`/`Trigger` overlap: document why Detect triggers on manifests but Inputs include dependabot.yml.
47. Review whether `gosmopolitan`, `spancheck`, `sqlclosecheck`, `rowserrcheck`, `zerologlint` (no SQL/OTel/zerolog in this repo) earn their keep in the 61-linter set — trimming is also policy.
48. golangci-lint `run.go` is `1.27` — revisit when nixpkgs' golangci-lint build version moves.
49. Add `--fail-on` to the BuildFlow provider path? (provider always runs Repair; check-mode severity policy is CLI-only today — decide if that is the right boundary).
50. Consider a second property: Decode→Encode→Decode yields Equal configs for all Safe inputs (complements idempotence).

## g) Questions I cannot answer myself

1. **Push authorization.** Master is 10+ commits ahead with CI workflows, releases, and the regenerated dependabot config. Everything remote-visible (Actions, Dependabot cycle, badges) is unverified until pushed. Should I push, or do you?
2. **SDK v1.0.0.** Tagging `linter-autoconfigure-sdk` v1.0.0 is a public semver-stability commitment on an external repo (latest is v0.2.0). Do you want that executed as part of this repo's TODO (with a full SDK audit first), or is the SDK's release cadence yours alone?
3. **Go floor policy trade-off.** Floor `go 1.27` = max toolchain, but it cost the treefmt goimports step (fleet helper limitation), requires `GOTOOLCHAIN=auto` on every machine for raw go commands, and needs erraudit rebuilt. Keep 1.27 (my recommendation, matches your "latest to the MAX"), or drop the floor to 1.26 — keeping the flake on go_1_27 — until go-nix-helpers handles toolchain-following formatters upstream?

---

_Point-in-time report; do not treat "is broken / is X" claims as current truth without re-verifying (see AGENTS.md)._

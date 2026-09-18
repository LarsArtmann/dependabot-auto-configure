# Status Report — 2026-09-18 05:45 CEST

**Repo:** `LarsArtmann/dependabot-auto-configure` (branch `master`)
**Session arc:** continuation session executing the standing
"READ → UNDERSTAND → RESEARCH → REFLECT → Execute → Verify, repeat" protocol.
Inherited a **red CI on pushed master** and closed it, plus the remaining
TODO_LIST backlog. All quality gates verified green on the final tree.
**State at close:** ~10 commits ahead of origin (daemon committing), **NOT
pushed** — push still requires explicit authorization. Now waiting for
instructions.

---

## a) FULLY DONE

1. **Baseline re-verified.** `git fetch` + `gh run list` exposed the real
   remote state: origin/master = `f85ca82` and its CI is **red** (lint +
   nix jobs). The prior session-summary claim ("~12+ commits ahead, not
   pushed, all gates green") was false in both details — reality was 1
   commit ahead with red CI.
2. **CI lint failure root-caused:** a fleet-strict `.golangci.yml` (wsl_v5,
   mnd, lll, varnamelen, err113, iface, gocognit, tagliatelle, golines@120,
   gofumpt/gci/goimports formatters) landed via daemon commit **after** the
   last local gate run — 57 findings against the new standard.
3. **All 57 findings fixed → 0 issues.** Formatter autofixes
   (`golangci-lint fmt`, `run --fix`), then manual work: named consts for 6
   magic numbers (`githubAPITimeout`, `errorBodyLimit`,
   `serverFaultStatusFloor`, `configDirPerm`, `yamlIndentWidth`,
   `initialDiffCapacity`), 11 over-long test fixtures converted to backtick
   raw strings (more readable AND under 120 cols), `u` → `update` renames,
   one named return removed, one makezero rewrite, root.go flag-help
   strings split.
4. **Tagliatelle fleet pattern researched and applied.** Compared with
   `golangci-lint-auto-configure`'s config: global `json: pascal`,
   `toml: kebab`, `yaml: kebab` rules + per-file exclusions for
   wire-shape/intentional files. Added the same block here; excluded
   `pkg/configure/configure.go` (the released snake_case `--json` contract
   — renaming would break the sweep-script consumer). The 4 yaml findings
   were the Dependabot schema itself (kebab) — correctly matched by the
   rule, never renamed.
5. **vendorHash mystery resolved without touching code.** Local HEAD
   (`eb41be3`) already carries the fixed hash
   `sha256-V0nv...` (authored outside this session, unpushed); origin is
   the older `f85ca82` with the stale hash — exactly what CI built. Verified
   empirically: local `nix build` passes; building `github:...master`
   reproduces the CI failure bit-for-bit (same drv, same hashes).
6. **Adversarial preservation tests added** — the bug class the idempotence
   properties are structurally blind to (they hold _after_ a lossy first
   write):
   - `TestReconcilePreservesEveryModeledField` (seeded, 500 iterations,
     deep-copy pristine comparison): no modeled field may be dropped —
     identity, labels, schedule day/time/timezone, explicit intervals,
     non-zero limits, configured groups, orphan entries. Only empty fields
     may be filled.
   - `TestRunRepairWritesBackAllCustomizations` (end-to-end through
     Encode + disk): monthly/monday/03:00/Europe-Berlin schedule, custom
     limit, labels, and an orphan npm entry all survive the write.
   - The v0.2.0 schedule-fill bug (replacing the whole `Schedule` struct)
     would trip both; neither test existed before today.
7. **Typed-error contract tests** (`pkg/configure/errors_contract_test.go`,
   `internal/cli/errors_test.go`): every one of the 10 error types pinned
   for family, code, message content, structured context, and cause-chain
   survival (`errors.Is`), including both branches of
   `UnexpectedStatusError.ErrorFamily` (5xx transient / else rejection).
8. **`--json` wire-shape contract tests** at both boundaries
   (`TestMarshalJSONResultPinsWireShape`,
   `TestJSONOutputPinsWireShape`): snake_case keys (`planned_write`,
   `wrote`, `unsafe_repair`, findings' `rule/message/severity/file`) are now
   executable documentation; `security_fixes` asserted omitted-when-empty.
9. **Coverage raised past the 80% target on every package:**
   cli 65.1% → **83.5%**, configure 70.1% → **89.2%**
   (detect 96.6%, dependabot 86.7%, provider 85.0%).
   New tests also cover `Execute` (the 0%-covered entry point via
   `t.Chdir`), `reportJSON`, and the stdout-write-failure path
   (`failingWriter` → `OutputError` via `errors.AsType`).
10. **Windows CI job added** to `.github/workflows/ci.yml`
    (`strategy.matrix.os: [ubuntu-latest, windows-latest]`, coverage step
    gated to Linux since `/dev/null` piping is unix-only). YAML validated.
11. **Issue-template defect fixed:** `config.yml` pointed at the Discussions
    tab, which is **disabled** on this repo (`has_discussions: false` via
    API). Decision (One Alternative): repoint to Issues with the existing
    `question` label — dismissed enabling Discussions because **0 of
    LarsArtmann's repos use Discussions** (verified via API) and it would
    add a second triage surface. `bug`/`enhancement` labels confirmed to
    exist (prior session's open concern resolved).
12. **AGENTS.md hardened:** new canonical **Gates** section (the full green
    checklist, with the lesson that the auto-commit daemon can land changes
    after your last gate run), fleet lint-config notes (tag convention,
    raw-string fixture style), and the stale "provider is the coverage gap"
    note replaced with current per-package numbers.
13. **Docs synced:** CHANGELOG (Added: adversarial tests, contract tests,
    windows matrix; Fixed: CI-red rescue, Discussions link), TODO_LIST
    distilled to 4 honest rows, ROADMAP gained the rejected-idea record for
    the `--json` policy-verdict question (exit code is the machine
    contract; duplicating policy state in JSON invites drift).
14. **Full gate sweep green on the final tree:** `go build` ✓ ·
    `go test -race` 5/5 ✓ · golangci-lint **0 issues** ✓ · erraudit
    **0 violations** ✓ · `nix build` ✓ · `nix flake check --all-systems`
    **all checks passed** ✓ · `dprint check` ✓ · dogfood `--check` exit 0 ✓
    · all issue-form/workflow YAML parse ✓.

## b) PARTIALLY DONE

1. **Windows CI job** — merged into ci.yml and validated locally, but never
   executed on a real `windows-latest` runner (CI has not run on the fixed
   tree). Runner-specific quirks, if any, surface only after push.
2. **CI verification overall** — everything is verified **locally**; origin
   is still the red commit. The first real green run, README badges,
   Dependabot's first scheduled `gomod` update PR, and the
   `actions/checkout` 6.0.3 → 7.0.1 PR (currently red, branched from red
   master; Dependabot will rebase onto green master) are all pending the
   push.
3. **Push authorization** — ~10 commits ahead of origin. Explicitly blocked
   per repo rules; not done.
4. **SDK v1.0.0** — still blocked: `~/projects/linter-autoconfigure-sdk`
   needs its dep sweep (go-finding v1.11.0, go-atomic-write v0.5.2) + gates
   - tag, all in an external repo.
5. **go-nix-helpers `meta.description`** — the `nix flake check
   --all-systems` warnings persist; fix is module-owned upstream, not
   per-repo. Not started this session.

## c) NOT STARTED

1. `docs-health` **HARVEST** of this report's section (f) into
   TODO_LIST.md / ROADMAP.md (the report is written; the loop-closing step
   is pending).
2. **v0.3.0 release cut** — the schedule-fill fix, JSON contract tests,
   windows CI, and the lint-compliance work all sit in `[Unreleased]`.
3. Release automation (tag → nix build → GitHub Release workflow) and/or
   GoReleaser decision.
4. A CI coverage gate (fail below 80%) so the target is enforced, not
   aspirational.
5. Short-duration `go fuzz` runs in CI (FuzzDecode exists; CI never fuzzes).
6. Shell completions (cobra-native) and an install section
   (`nix run github:...`, `go install ...@latest`) with a smoke-tested
   install path.
7. SUPPORT.md / CODEOWNERS (community files are otherwise complete).
8. Next-ecosystem detection (pip/pyproject is the most-requested candidate;
   cargo/composer/maven behind it) — ROADMAP fuel, no code.
9. `--dry-run` unified-diff output (currently prints the planned content,
   not a diff against the existing file).
10. Continuous-fuzz/bench tracking for the Detect hot path on >20-module
    repos (the cap path).

## d) TOTALLY FUCKED UP

1. **"Green" was shipped while CI was red** — the durable failure this
   session inherited and named: the previous verification ran, then the
   auto-commit daemon landed a strict lint config + toolchain bump that
   nobody re-verified. 75 → 57 lint findings, a stale vendorHash, and two
   red CI jobs followed. Fix now codified in AGENTS.md §Gates, but the
   process hole (daemon commits are unverified by construction) remains
   structurally open.
2. **I nearly chased a phantom bug.** Misread the initial "ahead by 1
   commit" against the stale "~12+ commits NOT pushed" summary, and then
   misinterpreted the `github:...master` build failure as environment drift
   — it was simply building the actual red origin commit (f85ca82). Both
   misreads were caught by cross-checking (`git fetch` + `gh api`) before
   any destructive action; no damage, but the reflex to distrust stale
   summaries should have fired on message one, not after.
3. **Self-introduced lint churn:** my first-pass new tests violated the
   same fleet config I was fixing for (4× err113 dynamic sentinels, 2×
   iface anonymous interfaces, gocognit 46 on the monolithic property test,
   1× paralleltest, 1× lll). All caught by the gate and fixed (named
   sentinels, named `domainError` interface, helpers split out,
   justified nolint) — but a careful first pass would have skipped the
   extra cycle.
4. **Tooling friction (environmental, no code damage):** the LSP was dead
   all session (NixOS `GOTOOLCHAIN=local` vs go.mod floor), killing
   `lsp_replace_symbol` mid-edit (fell back to bash truncation + edit);
   dprint's formatter repeatedly raced my edits (mod-time conflicts, three
   re-reads); `PIPESTATUS` capture in the shell wrapper silently dropped
   two exit-code echoes (outputs were inspected directly instead — the
   pipeline-masking lesson applied).

## e) WHAT WE SHOULD IMPROVE

1. **Gate re-run after any daemon/external commit** — codified in
   AGENTS.md §Gates; next step is making it mechanical (see f-10/f-11).
2. **Distrust point-in-time claims** — old session summaries and stale git
   refs cost this session its first 15 minutes. Always `git fetch` +
   `gh run list` before planning.
3. **Read the fleet sibling configs before writing repo config** — the
   tagliatelle answer existed in `golangci-lint-auto-configure` the whole
   time; checking first would have saved a research detour.
4. **Write fleet-lint-clean tests on the first pass** — named sentinel
   errors, named interfaces, functions under gocognit 25,
   `t.Parallel()` everywhere except `t.Chdir`, raw strings for long
   fixtures. The defaults are now written down in AGENTS.md.
5. **`errors.AsType` as the default assertion** — used in new tests;
   older `errors.As` sites in the repo are already erraudit-clean, keep it
   that way.
6. **VendorHash canary in CI** — dep bumps without a hash refresh currently
   fail in the nix job with an opaque mismatch (see f-10).
7. **Daemon/CI contract** — consider a post-commit hook that runs the fast
   gates (build + lint) or a CI step that fails with an actionable message
   when the working tree at HEAD was never locally verified.
8. **Toolchain alignment** — go.mod `go 1.27.1` vs nixpkgs `go_1_27`
   (patch version drift) is the most likely future source of FOD hash
   surprise; document or pin deliberately.
9. **Upstream the two go-standard defaults** (dead x86_64-darwin in the
   default systems list; missing `meta.description` plumbing) so fleet
   flakes stop carrying local overrides.
10. **Keep status reports honest about tooling** — this session's LSP/dead
    diagnostics would have burned a fresh session without the AGENTS.md
    note; same treatment for any new environment quirk discovered.

## f) UP TO 50 THINGS WE SHOULD GET DONE NEXT

_(brainstorm, front-loaded by impact; 1-6 are the real queue, the rest is
ROADMAP fuel for docs-health HARVEST to triage)_

1. **Push `master`** (~10 commits) with explicit authorization — unblocks
   2-5, 46, 47.
2. Watch the first real CI run on the fixed tree; fix any
   windows-runner-only failures (keep the job required unless g-3 says
   otherwise).
3. Verify README badges resolve against real CI state.
4. Confirm Dependabot's scheduled `gomod` update PR arrives and the
   tool's own repair handles it (dogfood loop closed).
5. Rebase + merge the `actions/checkout` 7.0.1 Dependabot PR once master
   is green; audit remaining action pins.
6. Cut **v0.3.0** (schedule-fill fix is user-facing; CHANGELOG
   `[Unreleased]` is ready).
7. Sweep SDK deps (go-finding v1.11.0, go-atomic-write v0.5.2), run its
   gates, tag `linter-autoconfigure-sdk` **v1.0.0** (g-2).
8. Bump this repo to SDK v1.0.0 when tagged; drop the comment in flake.nix
   about the toolsdk tag provenance.
9. Upstream `meta.description` plumbing to go-nix-helpers (kills the
   `nix flake check` warnings fleet-wide).
10. Add a **vendorHash canary** CI step: run `nix flake lock && nix build`
    with a clear "run nix build locally and update vendorHash" message on
    mismatch.
11. Make the auto-commit daemon (or a post-commit hook) run the fast gates
    so daemon commits can no longer ship red.
12. `docs-health` HARVEST: route this section into TODO_LIST/ROADMAP
    (top items in, the rest ROADMAP), then prune.
13. Enforce the coverage floor in CI (`go tool cover -func` total < 80% →
    exit 1).
14. Add gitleaks/secret-scanning job (the tool writes CI config; supply-
    chain hygiene should be visible).
15. Nightly short `go fuzz` CI job for `FuzzDecode` (e.g. 60s per seed).
16. Shell completions (bash/zsh/fish via cobra) + document in README.
17. Install smoke test in CI: fresh container, `go install ...@latest`,
    `--version`, `--check` on a fixture repo.
18. Windows contributor docs: GOEXPERIMENT/GOTOOLCHAIN shell setup.
19. Provider ↔ BuildFlow integration test against a fixture repo (beyond
    the toolsdk spec-level suite).
20. Benchmark Detect on a >20-module fixture (cap path) and record numbers
    in the README or docs.
21. ADR 0002: the `--json` wire contract (keys, stability promise,
    tagliatelle exclusion rationale).
22. ADR 0003: ecosystem detection policy (read-only detectors, what
    triggers a new detector).
23. `docs/DOMAIN_LANGUAGE.md`: reconcile vs repair, desired/pristine,
    orphan, cap, modeled customization.
24. `--dry-run` unified diff output (diff existing → planned).
25. `--json`: include detected shape summary (ecosystems → directories).
26. Non-TTY output check: assert plain-text report shape in CI logs (no
    fang styling leaking into logs).
27. Exit-code documentation inside `--help` long text (0/1/2 semantics).
28. `--fail-on` examples in README (CI policy recipes).
29. Add `SUPPORT.md` and `CODEOWNERS`.
30. Release workflow: tag → `nix build` → GitHub Release with artifacts +
    provenance; wire `Version` ldflags verification into it.
31. Test that a nix-built binary reports a non-`dev` version
    (`--version` gate).
32. Property test: `Encode` determinism (equal Configs → byte-identical
    output, seeded).
33. Property test: Diff is silent on modeled customizations (randomized
    version of the existing table test).
34. Verify the module-cap finding end-to-end with a 21-module fixture
    (cap wording + root-only generation).
35. Provider: honor a config-path override from the toolsdk context, with
    a wiring test.
36. Error-message audit round 2: the newer finding texts (orphan
    suggestion, schedule-missing why-clause) against the
    what/why/fix standard.
37. Pip/pyproject detection spike (detector sketch + decision doc, no
    merge).
38. Cargo/composer/maven/gradle detection — ROADMAP ordering.
39. Document the single-config assumption (one `.github/dependabot.yml`
    per repo; nested configs are out of scope) in README.
40. Structured `--verbose` logging (slog) behind a flag; default stays
    silent-by-design.
41. Track CI duration; add `actions/cache` for go build/test caches if the
    matrix doubles runner time.
42. `nix flake check --all-systems` caching check (aarch64-darwin build
    time on CI; consider `--no-link` output caching).
43. Quarterly ROADMAP prune is due after HARVEST (remove landed items).
44. Re-run the error-message audit against the new contract-test messages
    (consistency of family names in user-facing text).
45. Add an example repository template showing the tool in CI (consumer
    documentation).
46. After push: watch Dependabot versioning strategy — confirm `gomod`
    grouped PRs behave (open-pull-requests-limit 5).
47. Consider `actions/checkout` v7 migration notes if the bump PR changes
    behavior (read its release notes before merging).
48. Add repo-level `.nixfmt`/treefmt config drift check (flake check
    already runs treefmt; confirm it covers the new test files).
49. SECURITY.md dry-run: exercise the private-vulnerability-report flow
    once (file a test advisory) so the process is proven before needed.
50. Write the next status report only after push + first green CI run —
    that is the next natural checkpoint.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Push authorization:** may I push `master` (~10 commits) to origin now?
   Everything downstream (first real green CI run incl. the Windows
   matrix, badge verification, Dependabot PR rebase/merge, then v0.3.0)
   is gated on this one action, and I will not push without your explicit
   go-ahead.
2. **SDK release authorization:** may I work in
   `~/projects/linter-autoconfigure-sdk` — sweep its deps
   (go-finding v1.10.0 → v1.11.0, go-atomic-write v0.5.1 → v0.5.2), run
   its gates, and tag **v1.0.0**? It is an external repo and tagging is a
   public, semi-irreversible act, so I need your call.
3. **Windows CI policy:** if the new `windows-latest` matrix leg fails on
   a runner-specific quirk after push, do you want it **required** (CI
   stays red until fixed — my recommendation, it matches your
   fail-closed philosophy) or `continue-on-error` while it stabilizes?

---

_Point-in-time snapshot; goes stale. After push + first CI run, re-verify
before trusting any claim above. Section (f) is HARVEST input for
TODO_LIST.md / ROADMAP.md._

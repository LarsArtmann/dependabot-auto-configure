# Status Report — Issue #3: custom-named update groups false positive

- **Date**: 2026-10-02 12:08 CEST
- **Session scope**: Fix [issue #3](https://github.com/LarsArtmann/dependabot-auto-configure/issues/3)
  (`dependabot-grouping-missing` fires for entries with custom-named groups),
  full gates, then this self-review + status report. Nothing else was
  researched or changed.
- **Commits (auto-commit daemon)**: `c354c26` (fix + tests),
  `0cb60d2` (docs/tests), `cf5f099` (wsl whitespace), `bc68ca2` (dprint fix
  of a pre-existing daemon-committed status report). All local — **not
  pushed**; origin/master (`f85ca82`) was already CI-RED before this session
  (pre-existing BLOCKED TODO item).
- **Point-in-time snapshot.** Stale the moment CI runs again. Section (f)
  feeds docs-health HARVEST; the top actionable items were already moved to
  `TODO_LIST.md` this session (3 rows).

## Executive summary

Issue #3 is fixed, tested at four layers, and verified end-to-end with the
built binary: the issue's exact config now stays silent for the custom-named
`gomod` group while a truly group-less `github-actions` entry still warns.
All eight gates from AGENTS.md are green on the final tree. The self-review
surfaced **one verified pre-existing bug adjacent to the code I touched**
(unknown keys inside canonical group blocks are silently dropped by repair
despite a SAFE verdict) — reported in §d and TODO_LIST, deliberately NOT
fixed (scope restraint; needs your call, see §g).

## a) FULLY DONE

1. **Root-cause analysis** — `Groups` models only the two canonical names,
   so a custom-named `groups:` mapping decoded as "empty" while the raw-map
   audit saw the names and discarded them; `entryIssues` trusted
   `Groups.Empty()` and fired the false finding.
2. **Design decision with documented rejection** — a presence flag inside
   `Groups` (the issue's literal suggestion) would participate in
   `reflect.DeepEqual` and break idempotence between decoded and constructed
   configs (`assertEntryPreserved` compares exactly those). Presence lives on
   `Update` instead.
3. **Fix** — `Update.HasUnmodeledGroups` (`yaml:"-"`, audit-only, never
   generated): the decode audit (`auditEntries` → `auditEntry` →
   `auditGroups`) folds "groups mapping carried non-canonical names" back
   into the typed entry, per entry (`pkg/dependabot/config.go:163,277-348`).
   The finding now fires only on `Groups.Empty() && !HasUnmodeledGroups`
   (`pkg/dependabot/generate.go:433`). Unknown names remain the separate
   unsafe-audit signal (suggest-only, never rewritten). `groups: {}`,
   null-valued canonical keys, and the list form still count as "no groups
   mapping at all", keeping check-mode findings consistent with Reconcile's
   safe-path fill.
4. **Tests, four layers** — decode flag + shape matrix
   (`config_test.go`: `TestDecodeCustomNamedGroupsFlagsEntry`,
   `TestDecodeGroupShapesFlagMatrix`), three new `TestDiff` table cases
   (`generate_test.go`), integration test encoding the issue's "To verify"
   scenario (`configure_test.go:329`
   `TestRunCustomNamedGroupsNoFalseGroupingFinding`), fuzz seed for the
   custom-name shape.
5. **Binary-level verification** — built via `nix build`, run against a
   fresh fixture with the issue's config: exactly one finding
   (`dependabot-grouping-missing` for `"github-actions"`), `wrote:false`,
   `unsafe_repair:true`, file byte-identical afterwards. Exit 1 in check
   mode (correct: a real finding remains).
6. **Docs** — CHANGELOG `Unreleased/Fixed` entry; AGENTS.md bullet
   documenting the semantics (incl. the empty-mapping/null/list boundary);
   AGENTS coverage number refreshed (dependabot 86.6% → 87.1%).
7. **All gates green on final tree** — `go test ./... -race`; golangci-lint
   run (0 issues); golangci-lint fmt (zero changes); erraudit with all five
   flags (0 violations); `nix build`; `nix flake check --all-systems`;
   dprint check (clean); dogfood `--check` (exit 0, re-run on final tree).
8. **On-sight maintenance** — fixed pre-existing dprint drift in
   `docs/status/2026-10-02_11-20_v0.3.0-release-and-ci-fix.md` (committed
   by the daemon in `a00a262`, bypassing dprint).
9. **Coverage measured** (gap from previous session closed): configure
   89.2%, dependabot 87.1%, detect 89.7%, provider 85.0%, cli 83.5%,
   version 96.2% — all above the 80% target.
10. **HARVEST executed** — 3 rows added to `TODO_LIST.md` (1 High: the §d
    bug; 2 Low: fuzz campaign, alignment test).

## b) PARTIALLY DONE

1. **Issue #3 lifecycle** — fix is done and verified, but the GitHub issue
   is neither closed nor commented (needs your call: now vs. after release,
   see §g).
2. **Release** — CHANGELOG has an `Unreleased` section; no v0.3.1 tag cut.
3. **Unpushed work** — all fix commits are local only; CI on real runners
   has not validated them (origin is CI-RED for pre-existing reasons;
   pushing is a BLOCKED TODO awaiting authorization).
4. **Fleet validation** — `scripts/sweep.sh` (CHECK mode) not re-run, so
   the real-world count of repos that just stopped false-positiving is
   unknown.
5. **README** — not audited for a finding-rules section that might need the
   suppression behavior documented (only grepped once; single marketing
   hit).

## c) NOT STARTED

1. Fix for the §d verified bug (auditing unknown keys inside canonical
   group blocks) — TODO_LIST High.
2. `dependabot-config-unsafe` visibility finding (unsafe configs emit no
   finding today; only the `UnsafeRepair` flag / `unsafe_repair` JSON
   field).
3. `Equal()` hardening against audit-only fields (documented hazard, §e).
4. Fuzz campaign execution (seed added, `-fuzz` never run).
5. `auditEntries` alignment invariant as a test (comment exists, test
   doesn't).
6. Untested matrix cells: `groups: {gomod: null}` (unknown name, null
   value), `groups: {actions: null}`, custom names on the actions entry in
   `TestDiff`.

## d) TOTALLY FUCKED UP

Nothing shipped broken this session, but full honesty on what went wrong:

1. **VERIFIED PRE-EXISTING BUG (found during this self-review, not fixed)**:
   unknown keys inside canonical group blocks are neither modeled nor
   audited — unlike schedule and commit-message blocks, group values get no
   `auditBlock` treatment. A config carrying
   `groups.minor-and-patch.exclude-patterns` (valid GitHub schema) decodes
   SAFE, and a real repair run **silently drops `exclude-patterns` from the
   rewritten file**. Verified live 2026-10-02 with the built binary:
   `unsafe_repair:false`, `wrote:true`, `exclude-patterns` gone. This
   violates the repo's #1 hard rule ("Never destroy user intent") and the
   ROADMAP Non-goals contract ("silence about user intent is the failure
   mode this tool exists to prevent"). It is exactly one audit-function away
   from the code I changed today.
2. **Masked exit code** — my first formatter-gate verification ran
   `git diff --exit-code --stat | tail`, and the pipe swallowed git's exit
   code; the stat output made it LOOK verified. Caught one step later and
   re-verified properly. Lesson: never gate on a piped exit code.
3. **Fumbled smoke test** — the first binary smoke test exited 1 with
   truncated output and I did not immediately recognize correct-by-design
   check-mode behavior; required a second, stepwise run with output captured
   to a file.
4. **Lint in the wrong order** — shipped three `wsl_v5` whitespace
   violations because I ran golangci-lint only after writing docs; linting
   right after implementing would have caught them in one pass.
5. **"All gates green" was batch-scoped** — my closing claim was true for
   the final batch (tests/lint/erraudit/flake/dprint), but `nix build` and
   dogfood were from a source-equivalent earlier tree (whitespace-only
   delta). Both re-verified post-hoc (green). Pedantic, but the AGENTS
   gates section exists precisely because "locally-green" claims erode.

## e) WHAT WE SHOULD IMPROVE (self-review)

- **What did I forget?** Executing a fuzz campaign (only added a seed);
  measuring coverage unprompted; the issue's lifecycle (close/comment);
  README audit; checking FEATURES.md for a finding-rules list during
  harvest.
- **What's stupid that we do anyway?** The auto-commit daemon commits files
  that bypass dprint (proven by the drift I fixed) — either the daemon runs
  formatters or CI must gate harder. Also: gates that depend on my remembering
  8 commands; a `just`-like flake app (`nix run .# gates`) would remove the
  mnemonic burden.
- **What could I have done better?** Lint immediately after each code
  change; capture CLI output to a file with a separate exit-code echo from
  the first attempt; write the `auditEntries` alignment invariant as a test
  instead of trusting a comment; run coverage as part of the standard gate
  list.
- **What can I still improve?** `Equal()` uses `reflect.DeepEqual`, which
  now spans an audit-only field — safe today (proven: unsafe configs never
  reach it; all call sites analyzed), but a future caller comparing
  decoded-with-unmodeled-groups against constructed values could silently
  mis-judge idempotence. Either mask audit flags in `Equal` or document the
  invariant on it. Also the groups vocabulary spans three signals
  (`Groups` canonical fields, `HasUnmodeledGroups`, document-level
  `UnknownGroupNames`) — coherent and documented, but it deserves one table
  in one place.
- **Did I lie?** No claim turned out false. One claim ("all gates green")
  was batch-scoped rather than tree-scoped; tightened post-hoc (§d.5).
- **Ghost systems?** None created — the flag is wired end-to-end (decode
  audit → typed entry → finding suppression → tests → docs → CHANGELOG).
- **Scope creep?** Held: found the §d bug and did NOT fix it (reported +
  TODO'd). Fixed the daemon's dprint drift per the on-sight policy.
- **Removed something useful?** No.
- **Split brains?** One deliberate, documented divergence: the issue
  suggested `GroupsPresent`; the implementation says `HasUnmodeledGroups`
  because the literal name would lie when canonical groups are present
  (flag false, groups present). AGENTS.md documents the semantics.
- **Tests?** Four layers added for the fix; gaps listed in §c.6 and
  TODO_LIST. The suite's safety contract (`TestRun*`) was extended, not
  bypassed.

## f) Up to 50 things to get done next

Ranked-ish, grouped; everything here is grounded in this session's run.

**P0 — the verified bug**

1. Extend the `auditBlock` pattern to group values: `minor-and-patch`
   accepts only `update-types`, `actions` only `patterns`; anything else →
   unsafe/suggest-only (TODO_LIST High).
2. Regression test for #1: `exclude-patterns` inside a canonical group must
   flip SAFE → UNSAFE and the file must never be rewritten.
3. Product decision on #1's boundary: which GitHub group keys
   (`exclude-patterns`, `applies-to`, `dependency-type`, `patterns` on type
   groups) become modeled vs. audited-unsafe. ROADMAP Non-goals says
   audited-unsafe is the default posture.

**Correctness hardening**

4. Harden `Equal()` against audit-only fields (mask `HasUnmodeledGroups`)
   or document the invariant next to it.
5. Pin the `auditEntries` raw↔typed alignment invariant with a test (null
   entries, non-map entries) (TODO_LIST Low).
6. Matrix cell: `groups: {gomod: null}` — unknown name with null value must
   set the flag (currently untested).
7. Matrix cell: `groups: {actions: null}` — canonical null must NOT set the
   flag.
8. `TestDiff` case: custom-named groups on the `github-actions` entry (only
   gomod is covered today).
9. Property test: any decoded entry whose raw groups mapping is non-empty
   never produces `dependabot-grouping-missing`.
10. Round-trip property including audit flags (encode → decode → flags
    stable).
11. Run `go test -fuzz=FuzzDecode -fuzztime=60s` (TODO_LIST Low); fold
    real-world shapes from sweep findings into the corpus.
12. Belt-and-suspenders: test asserting Reconcile never sees
    `HasUnmodeledGroups=true` entries (callers gate on Unsafe; prove it).
13. Evaluate (maybe reject) the architectural alternative: capture unknown
    group names inside `Groups.UnmarshalYAML` to collapse the audit/decode
    split. Risk: reintroduces the DeepEqual hazard that shaped the fix.

**Finding vocabulary / UX**

14. Add a `dependabot-config-unsafe` finding so unsafe configs have a
    visible finding (today: silent except `unsafe_repair` in JSON).
15. Decide severity for #14 (info vs. warning).
16. `dependabot-grouping-missing` suggestion text: acknowledge "or keep
    your existing custom-named group".
17. Decide whether `minor-and-patch: null` should stay silently filled on
    the safe path or become audited unsafe (today: filled, key dropped).
18. Consider exposing `has_unmodeled_groups` in `--json` for tooling (only
    if a consumer materializes).

**Docs**

19. README: document the finding vocabulary incl. the suppression behavior
    (if a rules section exists; create one if not).
20. One table documenting the three group signals (`Groups` fields /
    `HasUnmodeledGroups` / `UnknownGroupNames`).
21. Restructure AGENTS.md: the new groups bullet sits under "Ecosystem
    model (v0.3.0)" but is not ecosystem-specific — move to hard design
    rules.
22. Check FEATURES.md during next docs-health pass for a finding-rules
    inventory item.
23. Add `go test -cover` to the AGENTS gate checklist so coverage numbers
    stop drifting.

**Release / process**

24. Cut v0.3.1 (CHANGELOG Unreleased → version; go-release skill flow).
25. Push master + verify CI green on real runners (pre-existing BLOCKED
    TODO; needs your authorization).
26. Close issue #3 with a verification comment (github-voice).
27. Teach the auto-commit daemon to run `dprint fmt`/treefmt before
    committing, or add a CI dprint gate (drift proven this session).
28. Verify CI pins golangci-lint v2.13.2 (AGENTS claims it; cheap check
    next CI touch).

**Fleet / real-world**

29. Run `scripts/sweep.sh` (CHECK mode) across the fleet; count repos whose
    custom-group false positive disappeared.
30. Sweep CSV: consider an `unsafe_reason` column (e.g. unknown group
    names) so suggest-only causes are visible in aggregates.
31. Dogfood stays in CI (exit 0 today; keep).

**CLI / provider**

32. cli-level test: `--json` wire shape on the custom-group scenario.
33. cli exit-code test: unsafe+findings → 1 vs. unsafe+clean → 0 (pinned
    only indirectly today).
34. configure test: entry carrying BOTH canonical and custom names (mixed
    mapping) — suppression and audit both apply.
35. Provider-level check: custom-group scenario flows through BuildFlow
    check mode (covered via configure today; add if provider tests lag).
36. SDK: no change needed (`FindingFromIssue` semantics untouched) — keep
    pinned; revisit only when findings vocabulary grows.

**Housekeeping**

37. Remove the stale `--json` `findingJSON` duplication risk: keep
    `MarshalJSONResult` field names frozen (contract); add a comment if a
    second consumer appears.
38. Add the §d demo fixture as a permanent integration test the day #1 is
    fixed (reuse the YAML from this report).
39. AGENTS.md: document that `nix build` must re-run after ANY go.mod/
    flake change (vendorHash guard) — it cost a prior session; one sentence.
40. Consider a `nix run .#gates` flake app bundling the 8-command checklist.
41. During the next docs-health pass: ANNOTATE this report when §d is fixed
    (do not rewrite it).

**Raw ideas (ROADMAP fuel, not tasks)**

42. Model the full GitHub groups schema in a v2 schema layer (contradicts
    current Non-goals; would need an explicit product reversal).
43. A `dependabot-explain` mode printing the three group signals per entry
    (debuggability for suggest-only verdicts).
44. Structured audit output (`--audit-json`) listing every unsafe construct
    with kind + path.
45. `dependabot-grouping-missing` could suggest the exact canonical YAML
    snippet for the entry's ecosystem.
46. Telemetry-free sweep histogram: findings frequency by rule across the
    fleet (local CSV only).
47. Support `groups` on `registries`-scoped configs (needs registries
    modeling — far future).
48. Consider `unsafe_repair` finding dedup when multiple unsafe entries
    exist (one doc-level finding vs. N).
49. Explore merging `auditBlock`/`auditGroups` into one generic
    `auditMapping(value any, known map[string]bool) bool` helper (three
    near-clones today; my change added a return value to one of them).
50. Rename `HasUnmodeledGroups`-adjacent vocabulary in one pass if a better
    domain term emerges (flag name is truthful today; revisit only with
    evidence of confusion).

## g) Questions I cannot figure out myself

1. **List-form `groups:` semantics** — the issue's fix spec says
   grouping-missing should fire "only when there is no groups mapping at
   all", which I followed literally: the list form still fires (and is
   separately audited unsafe). But the entry _does_ have groups in some
   form, so the finding's "has no update groups" text is arguably still
   false there. Suppress for list form too, or keep firing? Product call.
2. **The §d bug (`exclude-patterns` silent drop)** — fix now in this
   session (it extends the unsafe boundary per ROADMAP Non-goals), or leave
   it scheduled in TODO_LIST? It changes what counts as "safe", so it may
   deserve its own CHANGELOG entry and tests rather than a drive-by.
3. **Issue #3 closure** — close now with a verification comment, or hold
   until v0.3.1 is tagged and master is pushed (origin is CI-RED +
   unpushed; the push is a BLOCKED TODO awaiting your authorization)?

_Self-review skill note: this report is `.md` at the user's explicit path
demand; the skill's HTML default was overridden. Section (f) items 1-12
were routed with HARVEST rigor (top actionable ones live in TODO_LIST.md;
42-50 are ROADMAP fuel)._

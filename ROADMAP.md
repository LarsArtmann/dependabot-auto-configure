# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.

## Themes

### 1. Ecosystem breadth

The tool watches six ecosystems (gomod, github-actions, npm, pip, cargo,
gradle). Dependabot supports many more (docker, terraform, composer, ...).
Each new ecosystem means: a detection signal, canonical group/schedule
choices, and tests. Growth here should stay boring and incremental — one
ecosystem at a time, always behind the same safety contract.

Raw ideas:

- Detect Docker (Dockerfile / compose files), Terraform, composer
- Per-ecosystem canonical policies (e.g. docker pinning guidance)

### 2. Monorepo completeness

Detection is manifest-presence-based: any nested `go.mod` counts, npm counts
at the root plus nested package.json that declare `workspaces` or ship a
lockfile. A deeper shape model would understand even more membership
structure, producing tighter configs for large monorepos.

Raw ideas:

- Detect ecosystems from directory contents rather than single files
- Smarter cap behavior: summarize or tier entries instead of root-only —
  e.g. generate a `directories` glob entry for >20-module repos (the field
  is modeled since v0.4.0; the cap still generates root-only)
- `--root` defaulting to the git toplevel instead of the CWD

### 3. Frictionless distribution

The tool should be installable by strangers with zero private dependencies.
Both `go install ...@latest` and `nix run github:...` already work (all
dependency repos are public); what remains is breadth of channels.

Raw ideas:

- Homebrew / nixpkgs packaging
- GitHub Action wrapper (`uses: larsartmann/dependabot-auto-configure@v1`)
- Release workflow producing binaries on tag (nix-based or GoReleaser)
- Man page or a documented `--help`-as-canonical stance (fang already
  ships a `completion` subcommand for shells)
- Social preview image for the repo

## Open questions

Decisions only the owner can make; designs parked here until answered:

- **Pruning orphan entries** (43 info findings, the largest remaining
  bucket after v0.4.0): add an opt-in `--prune-orphans` (CLI flag +
  BuildFlow repair knob), or keep orphan entries human-decisions
  forever? Safe semantics if built: prune ONLY entries in ecosystems
  this tool models (gomod, github-actions, npm, pip, cargo, gradle)
  whose detection found nothing — never entries in undetected
  ecosystems (docker, terraform, ...), where "orphan" usually means
  "this tool is blind", and default OFF so the never-destroy-intent
  contract is unchanged.

## Non-goals

Things we are deliberately NOT pursuing and why:

- **Normalizing or rewriting unknown config constructs** (registries,
  custom groups, further schedule keys): since v0.4.0 unknown constructs
  ride along VERBATIM as raw nodes and repair fills around them — but
  they are never interpreted, validated, or regenerated. Turning
  passthrough into typed modeling remains a non-goal unless a finding
  needs it: each typed field is a new maintenance surface, and the
  passthrough already preserves everything. Suggest-only now fires only
  for shapes a rewrite cannot reproduce (list-form `groups:`, aliases,
  merge keys, nulls) — that narrowed contract is the product
  (ADR 0002).
- **Creating commits or pull requests**: this tool writes (at most) one
  config file; lifecycle automation belongs to Dependabot itself.
- **Network calls beyond the opt-in security-fixes flag**: file-level
  correctness needs no network. The one exception is
  `--enable-security-fixes`, an explicit opt-in that talks to the GitHub
  API; everything else stays offline.
- **Per-repo policy config files** (an `.autorc`-style override layer):
  convention-over-configuration is why the tool needs no setup today.
  Revisit only if the defaults prove too rigid in practice.
- **Policy verdicts inside the `--json` wire shape**: the process exit
  code is already the machine-readable verdict for `--check` +
  `--fail-on`; duplicating policy evaluation in JSON creates two sources
  of truth that can drift. Findings are present in the payload, so
  consumers with custom policies can threshold themselves.

---

<!-- Guidance for the builder:
  - NO bounded actionable tasks here. If it has a clear scope and effort
    estimate, it belongs in TODO_LIST.md.
  - NO status indicators on individual items. This is vision, not inventory.
  - Ideas should be raw and unrefined by design.
  - Non-goals are as important as goals: they prevent scope creep.
  - Revisit quarterly to prune stale directions.
-->

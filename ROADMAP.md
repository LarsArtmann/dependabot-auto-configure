# Roadmap

> Long-term direction and raw ideas. Items here are NOT actionable tasks.
> When an idea is refined into bounded work, it moves to TODO_LIST.md.

## Themes

### 1. Ecosystem breadth

The tool watches three ecosystems (gomod, github-actions, npm). Dependabot
supports many more (cargo, pip, docker, terraform, composer, ...). Each new
ecosystem means: a detection signal, canonical group/schedule choices, and
tests. Growth here should stay boring and incremental — one ecosystem at a
time, always behind the same safety contract.

Raw ideas:

- Detect Rust (Cargo.toml), Python (requirements.txt / pyproject.toml),
  Docker (Dockerfile / compose files), Terraform
- Per-ecosystem canonical policies (e.g. docker pinning guidance)

### 2. Monorepo completeness

Detection today is manifest-presence-based: any nested `go.mod` counts, npm
only at the root. A deeper shape model would understand workspaces and
module membership, producing tighter configs for large monorepos.

Raw ideas:

- npm workspace member detection (`workspaces` in package.json)
- Detect ecosystems from directory contents rather than single files
- Smarter cap behavior: summarize or tier entries instead of root-only
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

## Non-goals

Things we are deliberately NOT pursuing and why:

- **Modeling or rewriting unknown config constructs** (registries, custom
  groups, further schedule keys): the suggest-only safety contract is the
  product. Silence about user intent is the failure mode this tool exists
  to prevent. Modeled customizations (`labels`, schedule
  `day`/`time`/`timezone`) are the deliberate exception — each one is
  decoded, preserved verbatim, and covered by round-trip tests.
- **Creating commits or pull requests**: this tool writes (at most) one
  config file; lifecycle automation belongs to Dependabot itself.
- **Network calls beyond the opt-in security-fixes flag**: file-level
  correctness needs no network. The one exception is
  `--enable-security-fixes`, an explicit opt-in that talks to the GitHub
  API; everything else stays offline.
- **Per-repo policy config files** (an `.autorc`-style override layer):
  convention-over-configuration is why the tool needs no setup today.
  Revisit only if the defaults prove too rigid in practice.

---

<!-- Guidance for the builder:
  - NO bounded actionable tasks here. If it has a clear scope and effort
    estimate, it belongs in TODO_LIST.md.
  - NO status indicators on individual items. This is vision, not inventory.
  - Ideas should be raw and unrefined by design.
  - Non-goals are as important as goals: they prevent scope creep.
  - Revisit quarterly to prune stale directions.
-->

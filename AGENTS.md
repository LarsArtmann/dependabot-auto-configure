# AGENTS.md — dependabot-auto-configure

Non-obvious context for AI sessions. Read before changing anything.

## What this is

Standalone CLI + BuildFlow provider that auto-configures `.github/dependabot.yml`
from the detected repository shape (Go modules, GitHub Actions workflows, npm).
Sibling to `golangci-lint-auto-configure` and `oxlint-auto-configure`.

## Layout

- `cmd/dependabot-auto-configure` — binary entry point
- `internal/cli` — cobra/fang command wiring, exit codes 0/1/2, flags:
  `--root`, `--config-path`, `--check`, `--dry-run`, `--json`,
  `--enable-security-fixes`
- `pkg/detect` — repository shape detection (read-only)
- `pkg/dependabot` — typed config model: Generate, Reconcile, Diff, Encode
- `pkg/configure` — orchestrator shared by CLI and provider (`Run`);
  `github.go` is the opt-in GitHub API adapter behind
  `--enable-security-fixes` (reads `GITHUB_TOKEN`/`GH_TOKEN`)
- `pkg/provider` — BuildFlow `toolsdk.Spec` registration

## Hard design rules

- **Never destroy user intent.** Existing non-canonical choices (monthly
  schedules, custom limits, orphan entries for ecosystems we did not detect)
  are preserved byte-for-value. Only _missing_ fields are filled.
- **Unsafe = cannot round-trip, nothing else.** Since v0.4.0 every
  construct outside the typed model is captured as raw `yaml.Node`s
  (the `Unmodeled` inline maps — see below) and carried verbatim through
  Reconcile/Encode, so a rewrite drops NOTHING and repair converges on
  configs full of unmodeled schema. `DecodeResult.Unsafe` fires only for
  shapes no rewrite can reproduce: list-form `groups:`, YAML aliases or
  merge keys inside captured unmodeled subtrees (an alias dangles when
  its anchor sits in a typed field), and null scalars inside captured
  subtrees (the encoder renders a null as `''`, which re-decodes as a
  string — found by fuzzing, complex mapping keys included).
  `UnsafeReasons` names each. Unsafe
  repair is suggest-only, tested in `pkg/configure/configure_test.go`
  and `pkg/dependabot/passthrough_test.go`.
- **Semantic idempotence, not byte idempotence.** A hand-written config that
  parses to the same `Config` is a no-op (`dependabot.Equal` on the reconcile
  result); quoting, indentation, node position/comment metadata, and
  empty-vs-absent lists never trigger rewrites. Byte comparison is only
  the secondary fast path. `Equal` normalizes via `semanticConfig`/
  `semanticNode` — extend those when adding node-carrying fields, or
  cross-decode comparisons silently break.
- **Invalid entries are suggest-only too.** `Config.Validate` gates repair:
  an existing entry missing `package-ecosystem` or both `directory` and
  `directories` gets a `dependabot-entry-invalid` finding (WARNING
  severity — unfixable-by-design must sit below the fail-on threshold,
  BuildFlow gotcha #156) and the file is never rewritten.
  `Diff` owns the issue vocabulary; findings always flow through it.
- **Skip directories with a `testdata` / `vendor` / `node_modules` / `.git`
  segment** during detection (see `pkg/detect/detect.go`).
- **Module cap = 20** (`dependabot.MaxModuleEntries`). Beyond that, generate
  root-only and emit `dependabot-modules-capped`. Do not raise silently.

## Config model (v0.4.0)

- `RepoShape` carries per-ecosystem directory lists (`GoModuleDirs`,
  `NPMDirs`, `PipDirs`, `CargoDirs`, `GradleDirs`; "" = root) plus the
  `HasGitHubActions` flag. There is no `HasNPM` boolean — npm is a
  directory list like every other ecosystem.
- npm membership: root always counts; a nested `package.json` counts when
  any package.json declares `workspaces` OR its directory has a lockfile
  (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock[b]`).
  A malformed package.json contributes no workspace knowledge — detection
  only ever gets MORE conservative from a broken manifest.
- **Verbatim passthrough is the core mechanism.** Every struct that maps a
  YAML mapping carries `Unmodeled map[string]yaml.Node` tagged
  `yaml:",inline"` (Config, Update, Schedule, CommitMessage, TypeGroup,
  PatternGroup, Groups). go-faster/yaml's inline-map decode captures
  unknown keys as nodes and its encoder re-emits them sorted and
  deterministically (byte-stable across decode→encode — verified by fuzz).
  Contract: decoded, preserved verbatim by Reconcile, re-emitted by Encode,
  never generated, invisible to Diff. This is what made 34/300 fleet repos
  (ignore/reviewers/assignees/target-branch/registries/custom groups/...)
  repairable instead of suggest-only-forever. Do NOT add a typed field just
  to "support" a new schema key — the passthrough already preserves it.
  Value-side line comments on unmodeled keys survive; head comments on keys
  do not (same as typed fields — rewrites never preserved comments).
- List-form `groups:` decodes (Groups.UnmarshalYAML tolerates non-mapping
  shapes and sets `Groups.NonMapping`) and is audited unsafe, so repair
  never rewrites it. Do not "fix" this by generating a mapping — that
  would destroy intent.
- Custom-named groups ride in `Groups.Unmodeled`; `Update.HasUnmodeledGroups()`
  (derived, not a stored flag anymore) suppresses
  `dependabot-grouping-missing` — the entry HAS groups (issue #3) — and
  Reconcile must NOT graft canonical groups onto such an entry (the user
  grouped on purpose). Empty mappings (`groups: {}`), null-valued canonical
  keys, and the list form count as "no groups mapping at all" and keep the
  finding.
- Canonical group values (`minor-and-patch`, `actions`) are typed
  (`update-types`, `patterns` — both omitempty so a group carrying only
  unmodeled keys never fabricates an empty list); other keys inside them
  (`exclude-patterns`, `applies-to`, `actions.update-types`, ...) ride in
  the group's Unmodeled. Non-mapping canonical values (list, scalar) fail
  the typed parse — the corruption path, also suggest-only; only null
  stays on the safe path.
- `directories` (glob-list alternative to `directory`) is a typed,
  preserved-verbatim field. `Update.coversDir`/`Config.Find` treat an entry
  as covering any directory its patterns match (an UNPARSEABLE pattern
  counts as covering — a duplicate entry breaks Dependabot, a missed fill
  is only info); orphan detection (`entryDetected`) glob-matches against
  the detected shape the same way. `Validate` accepts either spelling.
- `DecodeStageInspect` is gone: Decode is a single typed pass plus
  `auditUnrepresentable`, which walks only the captured Unmodeled maps
  (alias/merge-key scan) and the Groups.NonMapping flag. There is no
  raw↔typed fold anymore — the old null-entry alignment hazard is
  structurally gone.
- `flake.nix` pins `goPkgAttr = "go_1_27"`: the go-standard auto-pick
  resolved to go_1_26, which fails the encoding/json/v2 floor.
- `scripts/sweep.sh <projects-dir> [out.csv]` sweeps all local repos in
  CHECK mode into CSV. NEVER sweep without `--check` — plain runs repair
  every repo they touch.

## Build

The SDK imports `encoding/json/v2`; the go.mod floor is `go 1.27` and the
flake builds with nixpkgs `go_1_27`. NixOS exports `GOTOOLCHAIN=local`, so
raw go commands with the system 1.26 toolchain fail the floor — prefix
`GOTOOLCHAIN=auto` (downloads go1.27.1 once) or run inside the devShell
(`nix develop -c ...`, which has go_1_27 and golangci-lint):

```sh
GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go build ./...
GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go test ./...
```

Use the flake (`nix build`, `nix flake check`) for the canonical gate.
dprint formats markdown/json/yaml (`dprint check`); CHANGELOG.md is excluded
from markdown formatting.

## Gates (the full green checklist)

A "green" claim is only true when ALL of these ran clean AFTER the last file
change — the auto-commit daemon lands changes (dep bumps, fleet config syncs)
after your last run, so locally-green history can still ship CI red:

```sh
GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 go test ./... -race
nix develop -c golangci-lint fmt   # then confirm: git diff --exit-code
nix develop -c golangci-lint run   # CI pins the same v2.13.2 binary
GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 erraudit ./... --type-aware \
  --enforce-go-error-family --no-suppress --enforce-samber-oops --enforce-generic-return
nix build                          # vendorHash guard; daemon go.mod bumps invalidate it
nix flake check --all-systems      # includes treefmt (gofmt)
nix shell nixpkgs#dprint -c dprint check
go run ./cmd/dependabot-auto-configure --root . --check   # dogfood, exit 0
```

Lint config is the fleet-strict `.golangci.yml` (same family as the sibling
`*-auto-configure` repos): formatters gofumpt/gci/goimports/golines@120,
`wsl_v5`, `mnd`, `lll`, `varnamelen`, etc. Struct-tag convention is
`tagliatelle` with `json: pascal` (new tags; the snake_case `--json` wire
shape in `pkg/configure/configure.go` is excluded — a released contract) and
`yaml: kebab` (the Dependabot schema itself). Long YAML test fixtures are
backtick raw strings, not escaped one-liners.

## Typed error system (DDD)

Errors are domain objects, not strings. Each bounded context owns its
vocabulary in an `errors.go`: `pkg/dependabot/errors.go` (the config
document), `pkg/configure/errors.go` (orchestration + GitHub adapter),
`internal/cli/errors.go` (reporting). Every type carries typed fields
(index, path, status code), an `ErrorFamily()` (Rejection = bad user input,
Transient = retryable transport, Corruption = unparseable/unrepresentable
document, Infrastructure = environment), a machine-readable `ErrorCode()`
(`config.*`, `shape.detect`, `findings.convert`, `github.*`, `cli.output`,
`provider.*`), and `ErrorContext()` structured data. Causes chain via
`Unwrap`, so `errors.Is`/`errors.AsType` work through the whole stack.

The gate is `erraudit` with zero violations expected:

```sh
GOTOOLCHAIN=auto GOEXPERIMENT=jsonv2 erraudit ./... --type-aware \
  --enforce-go-error-family --no-suppress --enforce-samber-oops \
  --enforce-generic-return
```

erraudit's binary must itself be built with a Go >= the repo floor (its
embedded go/types loads the packages); rebuild it from `~/projects/erraudit`
with `GOTOOLCHAIN=go1.27.1` when a floor bump makes it report package errors.

Non-obvious rules the flags enforce (empirically verified 2026-09-11):

- **No stdlib error constructors** (`--enforce-samber-oops`): never write
  `fmt.Errorf`/`errors.New`/`errors.Join` — not even for sentinels. Build a
  typed error value, or wrap with `errorfamily.Wrap*` constructors.
- **Concrete values must flow to return sites** (`--enforce-generic-return`):
  function signatures stay `error`, but each failure return returns a
  concrete typed value. Do NOT switch signatures to concrete pointer types —
  that invites the typed-nil-in-interface trap (sibling
  `golangci-lint-auto-configure` shipped two bugs from it).
- **No lost context** (`--enforce-go-error-family`): every in-scope variable
  on an error path must appear in the message or structured context. Human
  messages use user-facing values (e.g. relative `configPath`);
  `WithContext("config_path", absConfig)` carries the precise one — that key
  is also what error-family's FilesystemRule matches for diagnostics.
- **No blank-identifier discards** (`--no-suppress` surfaces them): check
  every write/close/read; a best-effort read failure becomes a field
  (`UnexpectedStatusError.BodyErr`), a stdout failure becomes
  `*cli.OutputError`, a success-path body-close failure becomes a transport
  error. Conditional discards need an explicit `&& err == nil` guard.

## Ecosystem wiring

- Findings are emitted via `linter-autoconfigure-sdk.FindingsFromIssues`
  (SDK pinned to the module proxy — no local replace; use `go mod edit
  -replace=...=../linter-autoconfigure-sdk` temporarily when iterating on
  the SDK locally). This repo is the SDK's
  first consumer — if `FindingFromIssue` semantics need to change, change the
  SDK, not this call site.
- The repo and every dependency (flake inputs and Go modules) are public,
  so `nix run github:...` and `go install ...@latest` work for anyone.
- BuildFlow integration is the toolsdk contract: `pkg/provider` registers a
  `toolsdk.Spec`; BuildFlow blank-imports it in
  `tools/providers/sdk_imports.go`. Detect runs check-mode; Repair honors
  `toolsdk.DryRunFromContext`.

## Testing

Table-driven tests for pure functions; integration tests with `t.TempDir`
fixtures in `pkg/configure`. Coverage target: 80%+ on every package —
currently (v0.4.0, after the passthrough work) dependabot 90.8%,
version 95.5%, detect 86.7%, configure 86.4%, provider 82.4%, cli 80.6%.
Seeded randomized properties pin Reconcile (idempotence, preservation of
modeled fields AND Unmodeled passthrough, mutation-tested) and
`FuzzDecode` pins the parse boundary; its corpus in
`pkg/dependabot/testdata/fuzz/FuzzDecode/` runs on plain `go test` —
extend it with real-world shapes (it found the null round-trip break).
The
`TestRun*` suite encodes the safety contract (check/dry-run/unsafe/no-op) —
extend it when adding behavior.

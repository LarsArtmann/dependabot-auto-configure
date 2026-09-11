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
- **Unsafe configs are suggest-only.** When `Decode` reports unknown top-level
  keys, unknown entry fields, or unknown group names (`DecodeResult.Unsafe`),
  repair MUST NOT write. A rewrite would silently drop the user's
  customizations (registries, custom groups). This is tested in
  `pkg/configure/configure_test.go`. Modeled customizations — entry `labels`
  and schedule `day`/`time`/`timezone` — are the exception: since v0.2.0 they
  are decoded, preserved verbatim by Reconcile, never generated, and invisible
  to Diff, so a config carrying them decodes Safe and repair converges
  (`pkg/dependabot/customization_test.go`). Unknown schedule keys are audited
  unsafe for the same reason: a rewrite would drop them.
- **Semantic idempotence, not byte idempotence.** A hand-written config that
  parses to the same `Config` is a no-op (`dependabot.Equal` on the reconcile
  result); quoting or indentation differences never trigger rewrites. Byte
  comparison is only the secondary fast path.
- **Invalid entries are suggest-only too.** `Config.Validate` gates repair:
  an existing entry missing `package-ecosystem` or `directory` gets a
  `dependabot-entry-invalid` finding and the file is never rewritten.
  `Diff` owns the issue vocabulary; findings always flow through it.
- **Skip directories with a `testdata` / `vendor` / `node_modules` / `.git`
  segment** during detection (see `pkg/detect/detect.go`).
- **Module cap = 20** (`dependabot.MaxModuleEntries`). Beyond that, generate
  root-only and emit `dependabot-modules-capped`. Do not raise silently.

## Build

The SDK imports `encoding/json/v2`; Go 1.26 toolchains build it without any
flags (verified), while the flake still exports `GOEXPERIMENT=jsonv2`. Raw go
commands work either way:

```sh
GOEXPERIMENT=jsonv2 go build ./...
GOEXPERIMENT=jsonv2 go test ./...
```

Use the flake (`nix build`, `nix flake check`) for the canonical gate.
dprint formats markdown/json/yaml (`dprint check`); CHANGELOG.md is excluded
from markdown formatting.

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
fixtures in `pkg/configure`. Coverage target: 80%+ on pkg/* (the
`pkg/provider` BuildFlow wiring is the known gap). The
`TestRun*` suite encodes the safety contract (check/dry-run/unsafe/no-op) —
extend it when adding behavior.

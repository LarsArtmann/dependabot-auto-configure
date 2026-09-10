# AGENTS.md — dependabot-auto-configure

Non-obvious context for AI sessions. Read before changing anything.

## What this is

Standalone CLI + BuildFlow provider that auto-configures `.github/dependabot.yml`
from the detected repository shape (Go modules, GitHub Actions workflows, npm).
Sibling to `golangci-lint-auto-configure` and `oxlint-auto-configure`.

## Layout

- `cmd/dependabot-auto-configure` — binary entry point
- `internal/cli` — cobra/fang command wiring, exit codes 0/1/2
- `pkg/detect` — repository shape detection (read-only)
- `pkg/dependabot` — typed config model: Generate, Reconcile, Diff, Encode
- `pkg/configure` — orchestrator shared by CLI and provider (`Run`)
- `pkg/provider` — BuildFlow `toolsdk.Spec` registration

## Hard design rules

- **Never destroy user intent.** Existing non-canonical choices (monthly
  schedules, custom limits, orphan entries for ecosystems we did not detect)
  are preserved byte-for-value. Only _missing_ fields are filled.
- **Unsafe configs are suggest-only.** When `Decode` reports unknown top-level
  keys, unknown entry fields, or unknown group names (`DecodeResult.Unsafe`),
  repair MUST NOT write. A rewrite would silently drop the user's
  customizations (labels, registries, custom groups). This is tested in
  `pkg/configure/configure_test.go`.
- **Semantic idempotence, not byte idempotence.** A hand-written config that
  parses to the same `Config` is a no-op (`dependabot.Equal`); quoting or
  indentation differences never trigger rewrites. Byte comparison is only the
  secondary fast path.
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
  (local replace in go.mod for local SDK iteration; the SDK is public and
  on the module proxy as of 2026-09). This repo is the SDK's
  first consumer — if `FindingFromIssue` semantics need to change, change the
  SDK, not this call site.
- The repo is public on GitHub, but the flake fetches its BuildFlow input
  over SSH from that private repo, so `nix run github:...` only works for
  people with access. A fully public install path is tracked in
  `TODO_LIST.md`.
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

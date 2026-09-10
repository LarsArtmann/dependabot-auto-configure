# AGENTS.md — dependabot-auto-configure

Non-obvious context for AI sessions. Read before changing anything.

## What this is

Standalone CLI + BuildFlow provider that auto-configures `.github/dependabot.yml`
from the detected repository shape (Go modules, GitHub Actions workflows, npm).
Sibling to `golangci-lint-auto-configure` and `oxlint-auto-configure`.

## Hard design rules

- **Never destroy user intent.** Existing non-canonical choices (monthly
  schedules, custom limits, orphan entries for ecosystems we did not detect)
  are preserved byte-for-value. Only *missing* fields are filled.
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

`GOEXPERIMENT=jsonv2` is REQUIRED (linter-autoconfigure-sdk uses
encoding/json/v2). The flake sets it; for raw go commands:

```sh
GOEXPERIMENT=jsonv2 go build ./...
GOEXPERIMENT=jsonv2 go test ./...
```

Use the flake (`nix build`, `nix flake check`) for the canonical gate.

## Ecosystem wiring

- Findings are emitted via `linter-autoconfigure-sdk.FindingsFromIssues`
  (local replace in go.mod; the SDK is unpublished). This repo is the SDK's
  first consumer — if `FindingFromIssue` semantics need to change, change the
  SDK, not this call site.
- BuildFlow integration is the toolsdk contract: `pkg/provider` registers a
  `toolsdk.Spec`; BuildFlow blank-imports it in
  `tools/providers/sdk_imports.go`. Detect runs check-mode; Repair honors
  `toolsdk.DryRunFromContext`.

## Testing

Table-driven tests for pure functions; integration tests with `t.TempDir`
fixtures in `pkg/configure`. Coverage target: 80%+ on pkg/*. The
`TestRun*` suite encodes the safety contract (check/dry-run/unsafe/no-op) —
extend it when adding behavior.

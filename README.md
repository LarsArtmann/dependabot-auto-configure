# dependabot-auto-configure

Auto-configure `.github/dependabot.yml` from the detected repository shape —
the Dependabot sibling of `golangci-lint-auto-configure` and
`oxlint-auto-configure`.

## Why

Dependabot defaults flood PR queues: one PR per dependency, no grouping, no
visible limit. The result, on any account with 100+ repos, is a graveyard of
stale dependency PRs nobody trusts. This tool fixes the config at the root:
it detects what a repo actually contains and writes a tight, grouped,
bounded configuration.

## What it does

- Detects **Go modules** (root + every `go.mod`, skipping `testdata`,
  `vendor`, `node_modules`, `.git`, and hidden dirs except `.github`),
  **GitHub Actions** workflows, and root-level **npm**
- Generates weekly grouped updates (minor+patch in one PR, actions by
  pattern) with an explicit `open-pull-requests-limit: 5`
- Repairs existing configs by filling only what is missing — a monthly
  schedule you chose stays monthly
- Refuses to rewrite configs containing constructs it does not model
  (registries, labels, custom groups): those get findings, not silent drops

## Usage

```sh
# generate or repair in place
dependabot-auto-configure

# CI gate: exit 1 when changes are needed, never writes
dependabot-auto-configure --check

# show the planned file without writing
dependabot-auto-configure --dry-run

# operate on another checkout
dependabot-auto-configure --root /path/to/repo
```

Exit codes: `0` clean or repaired, `1` changes needed (under `--check`),
`2` operational error.

## BuildFlow

`pkg/provider` registers a `buildflow/tool-sdk` Spec (Detect + Repair,
dry-run aware). BuildFlow consumes it via a blank import:

```go
import _ "github.com/larsartmann/dependabot-auto-configure/pkg/provider"
```

## Install

Via Nix:

```sh
nix run github:LarsArtmann/dependabot-auto-configure
```

> Note: the flake currently depends on the private `BuildFlow` repository
> over SSH, so the command above only works with access to it. See
> `TODO_LIST.md` for the plan to ship a fully public install path.

Or build directly (Go 1.26+; the source uses `encoding/json/v2` via the
auto-configure SDK):

```sh
go build ./cmd/dependabot-auto-configure
```

## Safety contract

| Situation                            | Behavior                                    |
| ------------------------------------ | ------------------------------------------- |
| No config, repo has ecosystems       | Generates grouped weekly config             |
| No config, repo has nothing to watch | No-op                                       |
| Config missing fields                | Fills missing fields, preserves your intent |
| Config semantically canonical        | No-op (formatting is never rewritten)       |
| Config with unknown constructs       | Findings only — never rewritten             |

## License

Proprietary — see [LICENSE](LICENSE).
All rights reserved.

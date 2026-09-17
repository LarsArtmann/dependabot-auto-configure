# ADR 0001: Unsafe configs are suggest-only

- Status: accepted
- Date: 2026-09-11 (decision implemented in v0.1.0; recorded as ADR 2026-09-17)
- Deciders: Lars Artmann

## Context

This tool rewrites `.github/dependabot.yml`. A rewrite means regenerating
the whole YAML document from the model this tool holds in memory.

The model is deliberately small: it understands `version`, `updates`
entries with a fixed field set (`package-ecosystem`, `directory`,
`schedule` with `interval`/`day`/`time`/`timezone`,
`open-pull-requests-limit`, `groups` with two canonical names, `labels`),
and nothing else. Dependabot's real schema is much larger: `registries`,
`assignees`, `commit-message`, `milestone`, custom group names, and more.

A repository can therefore contain a config this tool cannot fully
represent. When repair runs on such a config, regeneration would produce
a valid-but-different document: every field the model does not carry is
silently gone. Dependabot accepts the result, the commit looks innocent,
and the user's `registries` block — say, a private npm registry — stops
working at the next Dependabot run. Silent data loss in a config file, by
a tool whose job was to help.

## Decision

When decoding finds any construct outside the modeled schema
(`DecodeResult.Unsafe`: unknown top-level keys, unknown entry fields,
unknown schedule keys, unknown group names) or an unparseable document,
repair MUST NOT write. The run becomes suggest-only: findings describe
what was found, exit code stays 0, the file is untouched.

The same gate covers entries that fail `Config.Validate` (missing
`package-ecosystem` or `directory`): round-tripping a broken entry into a
config GitHub would reject is its own form of damage.

The full safety contract lives in the README table and is pinned by the
`TestRun*` suite in `pkg/configure/configure_test.go`.

## Alternatives considered

1. **Extend the model until everything is covered.** Dependabot's schema
   is a moving target owned by GitHub. Chasing it means every schema
   release risks a new silent-drop class. Rejected: unbounded cost,
   bounded safety.
2. **Merge: regenerate only the fields we model, keep the rest as-is.**
   Requires surgical YAML tree edits with comment/anchor/key-order
   preservation. go-faster/yaml node editing makes this possible but the
   merge logic becomes the new correctness surface (which unknown field
   belongs to which modeled subtree?), and idempotence — the property
   that repair converges — becomes much harder to guarantee. Rejected for
   now; revisit only if suggest-only proves too limiting in practice.
3. **Rewrite with a loud warning.** The damage is already done by the
   time anyone reads the warning; config files get committed by scripts.
   Rejected: silent drops must be impossible, not loudly apologized for.

## Consequences

- Repos with exotic configs never break by running this tool — they also
  never get repaired automatically. The finding tells the user exactly
  what is unmodeled and suggests aligning with the schema or extending
  the tool.
- Convergence is the contract only for modeled configs: repair is
  idempotent on anything that decodes Safe. This is asserted by
  `TestRunCanonicalConfigIsNoOp`, `TestRunUnsafeConfigIsSuggestOnly`, and
  the customization suite (modeled customizations — `labels`, schedule
  `day`/`time`/`timezone` — decode Safe since v0.2.0, so repair converges
  on configs carrying them).
- Every newly modeled construct (the v0.2.0 customization work) shrinks
  the unsafe class and must ship with decode/reconcile/diff tests proving
  the suggest-only contract still holds for everything left unmodeled.

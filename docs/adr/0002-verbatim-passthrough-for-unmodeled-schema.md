# ADR 0002: Verbatim passthrough for unmodeled schema

- Status: accepted
- Date: 2026-11-17 (shipped in v0.4.0)
- Deciders: Lars Artmann
- Supersedes: the scope of [ADR 0001](0001-suggest-only-unsafe-configs.md)
  (its rationale — "regeneration silently drops unmodeled fields" — no
  longer holds; its remaining validity is recorded below)

## Context

ADR 0001 made every construct outside the typed model block repair
(suggest-only forever). That was correct when a rewrite meant
regenerating from a model that could not carry the construct. But the
enumerated-model escape hatch (v0.2.0 `labels` + schedule keys, v0.3.0
`commit-message`) only fixed the keys someone noticed, one release at a
time — each GitHub schema addition re-created the trap.

A fleet sweep (2026-11-17, 300 local repos) measured the damage: 34
repos suggest-only-forever on valid GitHub schema — `ignore` (14),
`reviewers` (8), `assignees` (5), `commit-message.prefix-development`
(5), `target-branch` (4), `auto-merge`, `allow`, `versioning-strategy`,
`cooldown`, top-level `registries`/`enable-beta-ecosystems`, ~90
custom-named groups, and unknown keys inside canonical groups. Every one
of them carried permanent warnings in BuildFlow with an autofix that
could never fire — the tool's own findings gate became noise.

## Decision

1. **Carry every unmodeled construct as raw YAML nodes.** Each struct
   that maps a YAML mapping gets an inline `Unmodeled
   map[string]yaml.Node` (Config, Update, Schedule, CommitMessage,
   TypeGroup, PatternGroup, Groups). Contract: decoded, preserved
   verbatim by Reconcile, re-emitted by Encode, never generated,
   invisible to Diff. go-faster/yaml's inline-map support makes this
   enumeration-free — future GitHub schema needs no code here to stay
   safe.
2. **Unsafe now means "a rewrite cannot reproduce this verbatim"**, and
   only that: the list form of `groups:` (no typed representation of
   the shape), YAML aliases or merge keys inside captured unmodeled
   subtrees (an alias re-emitted verbatim dangles when its anchor is
   defined in a typed field; a merge key re-encodes with its tag spelled
   out), and null scalars anywhere inside captured subtrees, complex
   mapping keys included (the encoder renders a null as `''`, which
   re-decodes as a string; found by fuzzing after the 1.1M-exec clean
   run — corpus seeding with real-world shapes still pays).
   `DecodeResult.UnsafeReasons` names each.
3. **`directories` (the glob alternative to `directory`) is typed.** It
   needs semantics (validation, entry matching, orphan detection), not
   just preservation: entries using it must validate, must count as
   covering the directories their patterns match (never duplicated),
   and orphan detection glob-matches against detection. An unparseable
   pattern counts as covering: a duplicate entry breaks Dependabot, a
   missed canonical fill is only info.
4. **Unfixable-by-design findings sit at warning severity** (the
   `dependabot-entry-invalid` downgrade): repair refuses to write them
   by contract, so error severity is a permanent findings-gate failure
   (BuildFlow gotcha #156).

## Consequences

- Encode must stay deterministic: the yaml encoder emits inline maps
  sorted, so decode→encode is byte-stable (pinned by fuzz: 1.1M+
  executions of the round-trip invariant, zero failures).
- `Equal` normalizes before comparing (`semanticConfig`/`semanticNode`):
  node position/style/comment metadata and empty-vs-absent lists do not
  count. Any future node-carrying field must extend the normalizer.
- Encode never invents values: `directory`, `patterns`, `update-types`
  are omitempty, so a `directories`-entry or a group carrying only
  unmodeled keys re-encodes without fabricated empty lists (an empty
  pattern list could make a group match nothing).
- Reconcile does not graft canonical groups onto entries with
  custom-named groups: the user grouped on purpose.
- Value-side line comments on unmodeled keys survive; head comments on
  keys do not — identical to typed fields, whose comments rewrites never
  preserved. Comment loss is now the only rewrite-visible change beyond
  formatting.
- ADR 0001's gate survives for the narrowed unsafe set (list-form
  groups, aliases, merge keys, nulls in captured subtrees) and for
  invalid entries; its "unknown
  keys" rationale is retired by the passthrough.

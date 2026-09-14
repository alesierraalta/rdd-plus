---
name: no-excess-tests
description: "Trigger: writing tests, adding a test file, test cleanup, 'too many tests', reviewing a test diff before push. Keep only behavioral tests; route local-only tests to gitignored testLocales/; cut low-value tests."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
---

## Activation Contract

Apply when writing tests or deciding what test code gets pushed. Author-time companion to `pr-review` §3c (review-time detection). Also invoked by `branch-pr`'s Pre-PR Test Review Gate — a subagent runs this skill against the branch diff before any PR is opened. A test must earn its place by catching a real future regression. Applies to tests being promoted into the repo. Exploration probes produced by `exploit-testing` stay in scratchpad or `testLocales/` and are out of scope until promotion.

## Hard Rules

- For each kept test, name the distinct production behavior, contract, or regression it protects and the falsifiable oracle. Do not require a specific production line; no unique behavior or oracle → CUT candidate.
- Coverage of BEHAVIOR, not count: one representative case per equivalence class (each branch, each error path, each boundary). Extra cases in the same class are redundant.
- Markered live-infra/real-LLM tests only behind a marker (`llm_eval`/`memory_eval`); unmarkered live tests do not belong in the suite.
- Comments in promoted tests follow `~/.claude/skills/test-strategy/references/comments.md`: one-line intent only, the characterization label, and the ledger id; restating or stale comments are cut at promotion.

## Default Action: route, don't delete

The goal is **not to delete tests — it is to keep them OUT of the committed repo**,
where they rot and can't be maintained. For any CUT candidate the DEFAULT action is
**move to `testLocales/`** (gitignored, local-only). Borrar SOLO when it is so
redundant that even a local copy adds nothing — exact duplicate or pure tautology.
Consolidations that produce a KEPT behavioral test (parametrize, fold-negatives) stay
in the repo as the single covering test; only the leftover copies get routed/cut.

## Decision Gates

| Test type (BLOCK) | Action |
|-------------------|--------|
| Tautological (`assert X == X`, const vs itself) | Borrar only the exact tautology after previewing the candidate; never broaden deletion to neighbouring tests |
| Structure-pinning (`inspect.signature`, source-text grep, import presence) | → `testLocales/`; borrar si duplicado exacto. Trust types/behavior in the committed suite |
| File-exists / substring-in-generated-artifact, layout asserts | → `testLocales/` (validate by one real run); borrar si no aporta ni local |
| Coupled to LLM phrasing / fragile NL regex | Make behavior deterministic; test THAT in repo, route the regex version → `testLocales/` |
| Negative-mirror (N `doesn't_X` inverting one positive) | Fold into the positive assertion (kept); leftover negatives → `testLocales/` if they have local signal, else borrar |
| N near-identical cases differing by one input | One `pytest.mark.parametrize` (kept); extra copies dropped |
| Asserts-on-mocks (only echoes `return_value`) | → `testLocales/`; borrar si solo refleja `return_value` (sin señal ni local) |

## Execution Steps

1. For each new test, state the distinct prod bug it catches. Names one → keep in repo. Names none → it is a CUT candidate (default: route to `testLocales/`, not delete); preview the exact path and ownership before any move.
2. Exploratory / run-introspection / CUT-candidate tests → `testLocales/`, which MUST be gitignored. Use bounded, explicit paths; borrar only an exact duplicate or pure tautology, and report skipped ambiguous items.
3. Verify `.gitignore` lists `testLocales/`. If absent, stop and report the safety limitation; do not move or delete tests unconditionally.
4. Consolidate copies (parametrize), fold negatives into positives, trim docstring bloat — the consolidated behavioral test stays in repo; the leftover copies route/cut per step 2.

## Output Contract

The minimal covering set (every real behavior keeps exactly one test). Report target paths, ownership, preview, retention exceptions, and skipped ambiguities. List cuts by `path:line`; for whole-file noise say "borrar archivo".

## References

- `~/.claude/skills/pr-review/SKILL.md` — §3c test quality + `testLocales/` convention.
- `~/.claude/skills/right-size/SKILL.md` — scope-level overengineering.

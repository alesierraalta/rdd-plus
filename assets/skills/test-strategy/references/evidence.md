# Evidence contract — no claim without execution

A testing skill reports what it RAN and SAW. Reading code produces hypotheses, never
findings. This contract applies to every testing skill routed by `test-strategy`.

## Rules

1. **Two labels, one privilege.** Every conclusion is `observado` (executed and seen) or
   `razonado` (inferred). Only `observado` conclusions may be reported as a defect or as
   "works". `razonado` items are hypotheses: list them separately, each with the probe that
   would settle it.
2. **One record per observado conclusion**, with exactly these fields:
   - **Claim** — falsifiable, one sentence: "if <invariant> breaks, this goes red".
   - **Executed** — the command, test id, or driver path that ran.
   - **Inputs and parameters** — exact values, not descriptions.
   - **Observed** — verbatim excerpt, at most 10 lines; the full output saved at a path in the
     scratchpad or gitignored `testLocales/`.
   - **Mutation or negative control → result** — what was changed where, and whether the probe
     went red or stayed green (for example `>=` → `>` at `path:line` → probe went red).
   - **Reproduction** — the smallest input or sequence that shows it.
   - **Label** — the literal word `observado`. No synonyms (`observed`, `observed-state`, `verified`); a record with any other label is treated as `razonado`.
3. **"It works" needs the same record.** Green is evidence only when the probe was shown to
   go red under mutation or a negative control. A green that cannot be made red is a
   decorative-suite finding, not a pass.
4. **Never paraphrase output; quote it.** Trim, do not rewrite.
5. **Never report a defect from reading alone.** If it was not executed, it is `razonado`.

## Record template

| Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label |
|---|---|---|---|---|---|---|

## Example

| Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label |
|---|---|---|---|---|---|---|
| If a value exactly at the limit is rejected, the boundary probe goes red | `pytest tests/test_limits.py::test_at_limit -q` | `limit=100`, `value=100` | `1 passed in 0.02s` | `>=` → `>` at `src/limits.py:42` → `1 failed: assert accepted is True` | `accept(100)` with `limit=100` | observado |

The mutation column is what separates a test from a decoration: the same probe, one flipped
operator, one red run.

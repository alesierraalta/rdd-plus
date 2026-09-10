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
3. **Name the test's direction in the field's terms.** A reddening test is FAIL_TO_PASS: it
   fails on the defective version and passes once the fix lands (the SWE-bench and SWT-bench
   definition). The existing suite that must stay green through the change is PASS_TO_PASS.
   Record both per finding: the FAIL_TO_PASS test that proves the defect, and the PASS_TO_PASS
   set that proves the fix broke nothing else.
4. **"It works" needs the same record.** Green is evidence only when the probe was shown to
   go red under mutation or a negative control. A green that cannot be made red is a
   decorative-suite finding, not a pass.
5. **Never paraphrase output; quote it.** Trim, do not rewrite.
6. **Never report a defect from reading alone.** If it was not executed, it is `razonado`.
7. **The probe must survive as a test.** A probe that lived only in the scratchpad is gone when
   the session ends: nobody can re-run it, and the next change cannot break it visibly. Every
   `observado` defect names the promoted test (suite path and test name) whose outcome flips
   with the fix, and the evidence row records both runs. A test green with and without the fix
   pins nothing; it is a decoration with a defect's name on it.

## Record template

| Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label |
|---|---|---|---|---|---|---|

## Example

| Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label |
|---|---|---|---|---|---|---|
| If a value exactly at the limit is rejected, the boundary probe goes red | `pytest tests/test_limits.py::test_at_limit -q` | `limit=100`, `value=100` | `1 passed in 0.02s` | `>=` → `>` at `src/limits.py:42` → `1 failed: assert accepted is True` | `accept(100)` with `limit=100` | observado |

The mutation column is what separates a test from a decoration: the same probe, one flipped
operator, one red run.

# Learning loop — iteration log

One entry per measured iteration: the proposal, the readings, the decision rule fixed **before** the
numbers, and the decision with its rollback. Nothing here changes the method on its own: an iteration
is adopted only when the criterion below passes, and a rejection is a recorded result, not a failure.

Readings come from the benchmark (`bench run`), and every number below is a rollup of the per-case
`result.json` files. The rollup was validated against the baseline's own `aggregate.json`: cost, found,
caught, false positives, out-of-scope rows, pending adjudication, controls, light runs and the three
unique-defect counts all matched exactly, so the same arithmetic was applied to both sides.

---

## 2026-09-17 — trim the light-scope ceremony (rejected)

**Proposal, from recorded feedback.** The delegated `test-strategy` run of #96 recorded its own
retrospective (skill 0.3.9) and criticised the plan it had to produce: six `n/a` layer rows plus the
full template preamble are more file than a single-target bounded run needs.

**Change measured.** `assets/skills/test-strategy/SKILL.md` 0.3.9 → 0.3.10 (a light run may collapse
skipped layers into one aggregated `n/a` row) and `assets/test-plan-template.md` 136 → 110 lines
(column mechanics deferred to `references/evidence.md`). Commit `9ce3603`.

**Decision rule, fixed before the numbers.** Adopt only if adjudicated recall and precision do not drop
materially **and** cost and turns do not rise. A recall drop inside run-to-run variability with a clear
cost win is a WARN, not an adoption.

**Protocol.** Same clone, same corpus (`sha256:61d78d07b4779295`), same scorer, same model
(`openai-codex/gpt-5.6-luna`), `--runs 3 --concurrency 6`. Only the skill version differs.

| Reading | Revision | Units | Cost | Turns | Found | Caught | FP | Pending | No plan | Light runs |
|---|---|---|---|---|---|---|---|---|---|---|
| Baseline (0.3.9) | `d3fe552` | 60 | $2.149 | 1731 | 86/93 | 81/93 | 0 | 88 | 0 | 10 |
| After (0.3.10) | `9ce3603` | 59 | $2.206 | 1791 | 78/91 | 65/91 | 0 | 79 | 2 | 10 |

The after side is one unit short (`n10-order-state/2`): its agent run stalled past 3 minutes with no
disk activity, the run was killed, and the aggregate was rolled up from the 59 completed units. That
removes two defect-runs from the denominators above.

**Where the change actually bites** — the ten light runs:

| Light runs only | Units | Cost | Turns | Found | Caught | Rows | No plan |
|---|---|---|---|---|---|---|---|
| 0.3.9 | 10 | $0.429 | 355 | 13/15 | 13/15 | 13 | 0 |
| 0.3.10 | 10 | $0.389 | 336 | 11/15 | 7/15 | 11 | 0 |

**Unique defects per case** are unchanged (only `n04-slug-normalize` moved, 1 → 2 found), so the
proposal did not make any defect unfindable in this corpus; the differences live in repeat-level
consistency.

**Decision: NOT ADOPTED.** The intended win appears where it was aimed (light cost −9%, light turns
−5%) but it did not survive the criterion: cost and turns rose overall, the caught rate fell
(81/93 → 65/91 overall; 13/15 → 7/15 in the light subset), and two runs produced **no plan at all**
against zero in the baseline — a categorical new failure mode, not a small one. Precision could not be
part of the decision either way: no finding is adjudicated on either side (88 and 79 pending).

The light-subset drop is inside the variability the benchmark documents (five cases are unstable on
their own), so this is a rejection on the fixed rule, not a proof that the change is harmful.

**Rollback.** The skill stays at 0.3.9 and the template at 136 lines; `9ce3603` is dropped and nothing
from it reaches `master`.

**Follow-ups, in order.**
1. Explain the two no-plan runs before retrying any plan-shape change: `n01-csv-rfc4180/2` (15 turns,
   0 caught) and `g01-inventory-reserve/2` (39 turns, 2 caught). Nothing else in either reading failed
   to produce a plan.
2. If this proposal is retried, scope the reading to the light subset with at least five repetitions
   per case and a threshold declared before the run, instead of diluting ten light units inside
   fifty noisy ones.
3. Do not re-propose the same shape change until (1) is answered.

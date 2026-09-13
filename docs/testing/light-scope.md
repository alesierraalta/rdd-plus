# Proposal: a scoped run for a bounded change

Status: proposal, not implemented. Written 2026-09-10 against skill 0.3.6.

**Measured, 2026-09-12, and the falsification below is why this note is here.** Eighteen cases at three
runs a case, both sides on the same instrument, same model, same machine. The scoped mode declared
itself **seven times on each side**: the trigger wording was never what kept it from firing — a prompt
that named no bounded unit of work was — so the mode's reachability came from the reading, not from
the rule. Detection was unchanged (77 versus 78 caught of 93) and the runs cost **8.5% more turns and
6% more tokens**. The mode is kept for what it records, not for what it saves: a `Light:` run states
its blast radius, the classes it touched, and a reason for every layer it skipped, and that record is
what a plan which swept everything never leaves behind. The boundary list is not narrowed on this
evidence, and no reading should claim a saving from it.

## Problem, with the field evidence

A session ran the discipline on a change confined to one scope and paid for a whole-app campaign:
zero defects found in exchange for 8 evidence rows, 3 mutations, 2 local-server probes and a plan
over 100 lines, with half the value coming from two moments (a mutation that did not redden and a
route pin). Its own verdict was `partly`, and its closing line was the point of this document: for a
change that size it would skip the method next time unless there is an explicit lighter mode.

The skill is not silent about scope. Its state table already says a diff means its blast radius
first, and it already lets a scope keep its own plan beside another scope's. The cost comes from one
row of that same table:

| State found | Action |
|---|---|
| No `docs/testing/test-plan.md` | **PLAN the whole app**, then EXECUTE the first rows, same run |

So a repository with no plan — or a new scope beside an existing plan, which is the same state at a
different path — pays a full surface inventory plus a layer sweep that invokes up to eight sibling
owners in PLAN mode, however small the change is. That is the row to change, and nothing else needs
to move with it.

## What the corpus already proves about the size of the problem

Every case in `bench/cases/` is a small change: the fix overlay has a median of 32 lines, and the
smallest is 6. The last reading of the 18-case corpus cost $17.88 and 25 to 55 turns per case, and
those runs are precisely the shape a scoped run is for. There is no new corpus to build and no new
measure to invent: the instrument that would validate this proposal is the one already in use.

## The proposal

**A scoped run is the same discipline with a smaller campaign, never a smaller EXECUTE.**

Replace the "PLAN the whole app" row with a declared scope decision:

- A first run in a repository, or a first run for a scope, may PLAN the blast radius of the change
  instead of the whole app when the change is bounded: one target, a diff confined to one or two
  files, and no layer touched that the boundary list below forbids.
- The plan must say so in its header, in one line: `Light: <blast radius> · touches <classes>`.
- Every layer the run did not touch gets its row with status `n/a` and **a reason in its `Scope`
  cell** — the cell that already exists and already carries the layer's description. A reason is a
  sentence, not a keyword: `no persistence in the touched diff`, not `n/a`.
- The plan names what a full run would have added, in the "Not testing, on purpose" table: the layers
  left untouched and the journeys not driven.

What the scoped run keeps, unchanged:

- the plan is persisted and `plan check` closes it (rule 12);
- one target climbs to its target rung through its sibling, and the sibling is read, not approximated
  (rule 7);
- every confirmed finding leaves a pinning test asserting the contract, RED then GREEN (rule 13);
- every claim carries an evidence row, and `razonado` stays a hypothesis (rule 8);
- the regression gate still runs (rule 9).

What it drops: the whole-app surface inventory, the layer sweep beyond the touch class, the ranking
of more than one target, and the "Real-run recipes" table when no critical journey is touched.

## The boundary list: when a scoped run is not allowed

The mode is chosen by consequence, never by diff size — a three-line change to an authorization
check is not a small change. A scoped run is refused when the diff touches any of:

- authentication, authorization, or anything that decides who may do what;
- secrets, credentials, or PII handling;
- persistence: schema, migrations, or a query whose shape changed;
- money: rounding, totals, invoicing;
- an existing public behaviour that other code depends on, unless the diff is additive.

It is also refused when the diff is structural (renames across files, a moved package, a changed
interface), or when a plan already covers the same scope — in that case the run resumes that plan.

## What makes this checkable rather than a promise

`plan gaps` already counts a row as swept only when its status reads `done`, `fixed` or `closed`, and
already drops `n/a`, `na`, `none` and `skipped` from the denominator. That is exactly the mechanism a
scoped plan needs, and it is the reason this proposal does not invent a ledger or a measure.

One rule is missing on top of it, and it is bounded: **an `n/a` row in a plan that declares `Light:`
must carry a reason in its `Scope` cell.** That belongs in `plan check`, so the cheap half of the
decision (the declaration) is machine-checked while the judgement half (is this really bounded?)
stays with the operator and the plan's own reader. Without the rule, `Light:` is a label anyone can
type; with it, a scoped plan that skipped a layer without saying why fails the same gate every other
plan fails.

## How it would be falsified

One comparison, on the existing instrument: the 18-case corpus, the same scorer, the same model, two
skill contents — the current one and the one with the scoped run. The bar is the reading of
2026-09-10T17:36: 29 of 31 defects reported, 29 caught, no false positives, no failed case.

- If `caught` holds at 29/31 or better while turns and cost drop materially, the mode is validated:
  the ceremony was not buying detection on bounded changes.
- If `caught` falls, the mode over-trimmed, and the boundary list is the first thing to tighten — not
  the discipline.
- The corpus can only prove the first half of the claim: that a scoped run does not lose depth on a
  small library change. It contains no authorization or migration case, so it cannot prove the mode is
  safe on a risky diff. That boundary is enforced by the declaration and the reason rule, and it stays
  a human decision.

Cost of the experiment: one reading per content, about $18 each at the measured rate.

## Non-goals

- No second ledger, no new measure, no new score: the corpus, the scorer and the history carry it.
- No automatic inference of the mode from the number of changed lines. Lines are a proxy for effort,
  never for consequence, and rule 1 already says risk is the unit of decision.
- No reduction of `plan check`, and no exemption from rule 13 or the evidence ledger.
- No change to the Stop gate: it already reports breadth owed from the plan's own cells.

## Risks

- **The escape hatch.** If the mode is available for anything an author calls small, the blind spot
  the layer sweep exists to cover comes back. Mitigation: the declaration is in the plan, the reason
  rule is checked, and the gate's "assigned and never invoked" line still fires on a layer the plan
  assigned and nobody swept.
- **The mode becomes the default.** Mitigation is measurement: the corpus reading shows what the mode
  costs and what it keeps, per case, with the two misses named.
- **The skeleton tempts a scoped plan to stay scoped.** A plan created as `Light:` is a plan for that
  scope; the next run in the same repository either resumes it or plans the scope it is asked for.

## Review workload

Skill content plus one `plan check` rule plus its tests: under 200 lines, one work unit, no chained
PR.

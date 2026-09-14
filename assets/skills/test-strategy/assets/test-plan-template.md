# Test plan — <app / module>

Created: <date> · Last updated: <date> · Plan path: `docs/testing/test-plan.md` · Sandbox: <worktree / container / scratchpad> · Findings precision: <confirmed+fixed> / <rows with a verdict>
Baseline: `<git rev>` · untracked files: <count> · fingerprint: `<assets/fingerprint.sh output>` (a change is anything that differs from this fingerprint, not raw `git status`)

## Inventory

| Surface | Entry points | Owner module | Notes |
|---|---|---|---|

## Ranked targets

Rows are never removed by budget; budget changes order and status only. Rows contributed by a
sibling in the layer sweep name that sibling in "Sibling skill".

| Target | Blast radius | Churn / past fixes | Consequence class | Existing evidence | Altitude | Target rung | Sibling skill | Verdict | Status |
|---|---|---|---|---|---|---|---|---|---|

Verdicts: probe · pin · none. Statuses: pending · in progress · done · blocked · n/a.
A `none` verdict is created with status `n/a`; the execution ratio excludes `n/a` rows.

## Real-run recipes

How each critical journey is driven for real (`real-run-validation`), so EXECUTE never rediscovers it.

| Journey | Start command | Data setup | Sample requests | Expected observable |
|---|---|---|---|---|

## Layer matrix

| Layer | Skill | Scope | Status |
|---|---|---|---|
| Security | `appsec-adversarial-auditor` | auth boundaries, untrusted input, secrets | pending |
| Runtime and faults | `runtime-reliability-testing` | load, latency, fault injection, smoke | pending |
| Persistence and migrations | `database-persistence-testing` | migration naming/order/idempotency, isolation, N+1 | pending |
| Architecture conformance | `clean-architecture-audit` | layer purity, targeted mutation | pending |
| Critical e2e journeys | `real-run-validation` | <journey 1>, <journey 2>, <journey 3> | pending |
| Sandbox | `docker-test-containers` | ephemeral DB/cache, environment proof | pending |

Statuses: pending · in progress · done · blocked · n/a. `plan gaps` counts a row as swept when its
status cell reads `done`, `fixed` or `closed`, and drops `n/a`, `na`, `none` and `skipped` from the
denominator entirely; the `Skill` cell only labels the rows still owed. In a plan that declares
`Light:`, every `n/a` row also states its reason in the `Scope` cell.

## Not testing, on purpose

| Target | Reason |
|---|---|

## Characterization (legacy)

| Test | Behavior pinned | Believed correct? | Promote or delete after the change |
|---|---|---|---|

## Execution log

| Date | Target | Rung reached | Findings (path:line) | Promoted tests | Evidence (ledger id) | Notes |
|---|---|---|---|---|---|---|

## Findings

A rejected or wontfix finding is a known non-issue: it is never re-proposed unless the fingerprint
of its cited files changed; when a run skips it, it cites the row. Severity is the consequence class
(`references/prioritization.md`); always state whether data is safe. A `confirmed` or `fixed`
finding names the promoted test that asserts the promised behaviour, so it is red on the current
code and green once fixed (rule 13); a test written the other way round is a characterization
test and says so in its name. A finding that never got a test stays `open`, reason `not pinned`.

| Id | Finding (path:line, one line) | Severity (consequence class) | Data safe? | Evidence id | Pinning test (suite path :: test name) | Status | Verdict by / date | Reason | Cited-files fingerprint at verdict |
|---|---|---|---|---|---|---|---|---|---|

Statuses: open · confirmed · fixed · rejected · wontfix.

## Evidence ledger

One row per `observado` conclusion (`references/evidence.md`). `razonado` items go under
"Hypotheses" below, never here.

| Id | Claim | Executed | Admit | Inputs and parameters | Observed | Digest | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |
|---|---|---|---|---|---|---|---|---|---|

`Admit` holds ONE bare shell command, with no backticks and no placeholders, because `Executed` is
prose a human reads and `Admit` is the command the binary runs. `Digest` holds the `sha256:` digest of
the normalized output; `plan admit` prints the digest it observed and writes nothing, so this cell is
filled by copying what it printed. `plan check` requires neither column; `plan admit` refuses a row
whose `Admit` is absent or whose `Digest` is unpinned.

### Hypotheses (razonado)

| Hypothesis | Probe that would settle it |
|---|---|

## Calibration history

One row per calibration run (`references/calibration.md`); never overwritten.

| Date | Skill version | K | Found | Recall | Misses (file:line operator, why) | False positives |
|---|---|---|---|---|---|---|

## Blocked by testability

| Target | Rung | Why | Minimal change that opens it |
|---|---|---|---|

## Remaining, in order

1. <target — target rung — sibling>

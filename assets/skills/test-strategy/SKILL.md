---
name: test-strategy
description: "Trigger: haz el testing, testea esto, prueba esto, test this, test the app, test my change, run the testing, calibra el testing, calibrate the testing, test strategy, what to test, where to start testing, test planning, testing priorities, test altitude, low coverage, legacy code testing, test plan for the app, testing roadmap, plan de testing, roadmap de pruebas. Organic entry point for testing: infers scope and mode from repository state, builds or resumes a persisted test plan, and executes it through specialized testing skills."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "3.1"
  scope: [common]
  auto_invoke: "Any request to test something: infer scope and mode from repo state, build or resume the persisted plan, execute it through specialized testing skills"
---

## Activation Contract

Load on ANY request to test something ("haz el testing", "test this") and on the strategic
questions. The phrase is the whole instruction: read repository state, infer mode and scope
from the state table, act. Never ask which mode; one question only if the target is ambiguous.

- **PLAN**: inventory → layer sweep → rank → assign → persist; then EXECUTE unless asked for
  the plan only. **EXECUTE**: next pending target → sandbox → sibling → update the plan.

This skill decides WHICH targets and routes; siblings do the work.

## Hard Rules

1. **Risk, not the file, is the unit of decision.** Rank qualitatively
   ([references/prioritization.md](references/prioritization.md)); no numeric scores or targets.
2. **Falsifiable evidence per selected risk**: mutation, contract, invariant, negative control,
   differential, metamorphic, or observed state. Coverage percentage is not evidence.
3. **Contract boundary that survives a refactor** ([references/altitude.md](references/altitude.md));
   doubles only at process boundaries.
4. **Legacy: characterization tests first**, labeled. **Bug: the reddening test first.**
5. **THE PLAN NEVER SHRINKS.** Every ranked target keeps its row and target rung; budget decides
   order and how far today, never what is dropped.
6. **DEFAULT BUDGET is everything.** No user cap: every selected target runs to its target rung.
   Stop only at a testability defect or a user cap; report the remainder.
7. **Routing is an instruction.** INVOKE the sibling (Skill tool, or read
   `~/.claude/skills/<name>/SKILL.md`); never approximate it.
8. **No claim without execution.** Every finding and every "works" carries an evidence record
   per [references/evidence.md](references/evidence.md); `razonado` items are hypotheses.
9. **Regression gate.** A diff is validated only when the existing suite is green, its blast-radius
   rows ran to target rung, and `real-run-validation` drove the affected journey.
10. **RDD receipt.** Derived from an evidence ledger row, only when a provider-issued lineage id
    exists (`_shared/test-receipt-contract.md`); otherwise the ledger row is the record. Evidence
    only, never authority.
11. **Findings persist with verdicts** (template "Findings"). Severity is the consequence class;
    state whether data is safe. Never re-propose a `rejected` or `wontfix` finding unless its
    cited-files fingerprint changed; cite the row when skipping.
12. **Persist before you report.** The final message is written only after
    `docs/testing/test-plan.md` is on disk with every finding row, its evidence row, and the row
    statuses of this run. A finding that exists only in chat does not exist: the next run cannot
    honor its verdict, and nobody can re-score it. Asking the user whether to fix something never
    replaces writing the finding first.

## Decision Gates

**State → mode and scope** (read state first, always)

| State found | Action |
|---|---|
| No `docs/testing/test-plan.md` | PLAN the whole app, then EXECUTE the first rows, same run |
| Plan exists but predates the template (missing sections or `Baseline:`) | Migrate it: add the missing sections empty, record the baseline (`assets/fingerprint.sh`), treat the tree as changed, then EXECUTE |
| Plan exists, fingerprint unchanged | EXECUTE from the first `pending` row |
| Plan exists, files differ from the plan baseline | Refresh rows in that diff's blast radius, rank first, EXECUTE |
| Plan exists, baseline diff is large or structural | PLAN refresh (keep statuses), then EXECUTE |
| User says "plan only" / "solo el plan" | PLAN, stop |
| User says "calibra el testing" | CALIBRATE ([references/calibration.md](references/calibration.md)) |
| Monorepo with several apps, none named | Ask one question: which app |

Scope: a diff means its blast radius first, closed by the regression gate (rule 9); a clean
tree on main means the whole app.

**Verdict per target**: probe · pin · none (`n/a`), profiles in
[references/prioritization.md](references/prioritization.md). **Routing** by need: full table in
[references/ordering.md](references/ordering.md); `exploit-testing` owns adversarial probing.

## Execution Steps

**Always first**: read `docs/testing/test-plan.md` if present, `git status`, the diff against
main, the test runner. Pick mode and scope from the state table; state the inference in one line.

**PLAN**

1. Inventory surfaces and entry points with CodeGraph when `rdd-plus doctor` reports it
   available, otherwise with `git ls-files` plus grep: routes, CLIs, jobs, migrations,
   ports/adapters, critical journeys.
2. Layer sweep: invoke each layer owner in PLAN mode and take the target rows it returns
   (its own checks): security `appsec-adversarial-auditor` · runtime and faults
   `runtime-reliability-testing` · persistence and migrations `database-persistence-testing` ·
   architecture `clean-architecture-audit` · e2e journeys `real-run-validation`; when the
   surface exists: `iac-safe-auditor`, `rag-audit-evaluator`, `ai-evals-auditor`. The router
   ranks what siblings contribute, never invents it.
3. Rank ([references/prioritization.md](references/prioritization.md)).
4. Per target: altitude, target rung L1–L5, sibling skill, verdict.
5. Layer matrix (one row per layer plus sandbox); name anti-priorities with reasons.
6. Record the baseline (`assets/fingerprint.sh`: repo fingerprint; per-finding cited-files fingerprint) and persist to
   `docs/testing/test-plan.md` (or the path the user names) using
   [assets/test-plan-template.md](assets/test-plan-template.md); rows start `pending`,
   `none` rows start `n/a`.

**EXECUTE**

1. Pick the first `pending` row, or the one the user names.
2. Sandbox: throwaway worktree or ephemeral container; environment proof; probes in scratchpad
   or gitignored `testLocales/`.
3. Invoke the assigned sibling; it climbs to the target rung without reduction.
4. Promote probes through `no-excess-tests`.
5. Update the plan: row status, rung, evidence ledger, findings, testability blocks with the
   minimal change that opens them. Write the file, then report the delta (rule 12).

## Output Contract

Return: inferred mode and scope (one line); plan path and delta; ranked table; layer matrix;
not-testing list; routing ledger (per sibling: `contributed rows` / `invoked` / `skipped` +
reason; assigned-but-never-invoked is an open gap); evidence ledger; findings with verdicts and
precision; execution ratio (done / rows excluding `n/a`) and the ordered remainder.

## References

- [references/prioritization.md](references/prioritization.md) · [references/altitude.md](references/altitude.md) ·
  [references/ordering.md](references/ordering.md) (stop rules, full routing) · [references/evidence.md](references/evidence.md) ·
  [references/calibration.md](references/calibration.md) (`assets/seed-mutants.py`) · [references/comments.md](references/comments.md) ·
  [assets/test-plan-template.md](assets/test-plan-template.md).

---
name: breakcheck
description: "Trigger: breakcheck, romper esto, intenta romperlo, testing adversarial, pruebas adversariales, casos limite, edge cases, regresiones, robustez, seguridad, supuestos, before RDD. Bounded adversarial campaign on one candidate: select the few risks worth attacking, run falsifiable probes under a stated budget, and report evidence plus an explicit readiness disposition."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "0.1.1"
  requires_tpp: "0.3.8"
  scope: [common]
  auto_invoke: "Explicit invocation to break a change before RDD: bounded adversarial probes and a readiness disposition"
---

# breakcheck

Break the candidate, not confirm it. The existing Verifier answers "does it work"; breakcheck answers "what invalidates its assumptions". Run a bounded adversarial campaign that turns selected risks into reproducible evidence and a readiness recommendation.

## Activation Contract

Load only on explicit invocation ("breakcheck", "romper esto", or "testing adversarial") for one named candidate. Do not load for whole-repository audits, routine unit tests, PR review, releases, or coverage targets.

## Probe Selection

- Start with assumptions whose failure would block readiness or change the project authority's decision.
- Prefer boundary, malformed, repeated, concurrent, interrupted, and unauthorized inputs when they match the candidate's contract.
- For regressions, compare the smallest safe behavior against the base; do not treat base behavior as correct without a contract.
- For integration risks, exercise the real seam that the candidate claims to use.
- For security risks, use synthetic data and non-destructive inputs; stop before shared or remote state without authorization.
- Define the invariant, input, oracle, and evidence location before spending a probe.
- Prefer one causal change per probe so a failure has a smallest reproducer.
- Bound randomness, retries, and concurrency inside the stated budget.
- Prefer deterministic inputs and preserve enough output to reproduce a failure.
- Drop probes that cannot change a disposition, and name each omission in the report.

## Hard Rules

1. Identify the candidate before probing: project/root and a human-readable execution label, plus base and HEAD and the relevant staged, unstaged, and untracked content; HEAD alone is not what runs.
2. Risk, not coverage: enumerate the few assumptions worth attacking across correctness, normal and edge cases, regressions, integration, robustness, and security; explicitly drop the rest.
3. State a finite budget (number of probes and wall-clock time) before the first probe; stop at the limit.
4. Every probe is falsifiable: "if <invariant> breaks, this goes red"; record expected and observed state plus an evidence reference.
5. Isolate proportionately to the risk; a worktree is not a sandbox, and shared, production, or remote resources need explicit authorization.
6. Use no global score: use PASS, WARN, FAIL, INCONCLUSIVE, or N/A per applicable check.
7. Missing required evidence or any open blocker means not ready; WARN needs an explicit accepted disposition by the project authority, and accepted risk stays visible with its reason.
8. An honest negative is valid: "no defect observed within scope X under budget Y" never claims the code is free of defects.
9. Readiness is a recommendation only: never claim RDD approval, review authority, delivery authorization, or a substitute for the Verifier.
10. Report a finding only with a reproducible path (input, command, or sequence) and the smallest reproducer; unsupported suspicion is a hypothesis, not a finding.

## Execution Steps

1. Read the candidate, its contracts, and any Verifier evidence.
2. List assumptions and choose the few probes worth running; record omitted risks and why.
3. State the probe and time budget before the first probe.
4. Run probes in isolation and stop when the budget is exhausted.
5. Record expected and observed state, evidence references, environment, and disposition.
6. Use exit status, returned values, files, logs, and observable state as evidence; a plan, transcript, or claimed invocation is not execution proof.
7. Re-run the smallest reproducer for a suspected finding before reporting it.
8. If a probe cannot run, mark INCONCLUSIVE or N/A with the reason; never convert inability into PASS.
9. Write the report from `assets/readiness-report-template.md` at a caller-chosen path under `docs/testing/`.
10. Close with the beta feedback obligation below, including blocked or partial runs.

## Disposition Guide

- Mark PASS only when required probes and evidence support the stated outcome with no unresolved blocker.
- Mark WARN for a bounded concern or omission only when the project authority explicitly accepts its disposition.
- Mark FAIL for a wrong required behavior or a blocker that prevents the stated outcome.
- Mark INCONCLUSIVE when the finite evidence cannot distinguish the relevant outcomes.
- Mark N/A only when the probe is outside scope and the reason is recorded.
- Cite the exact scope and budget whenever reporting a negative result.
- Separate missing required evidence from a probe that actually failed.
- State whether each warning was accepted, by whom, and for what reason.
- Do not hide an unresolved blocker inside a nominally successful check.
- Keep accepted risks, missing evidence, and residual risk visible in the report.

## Beta feedback obligation

After finishing, including blocked or partial runs, record the retrospective in the existing ledger and keep it out of chat. Generate the existing template, fill only `ts`, `repo`, `plan`, `skill`, `build`, `paid`, `cost`, `reason`, and `verdict` (`paid`, `partly`, or `ceremony`), with optional `guess` and `freeform`; the parser rejects unknown keys. Set `skill` to what actually ran as `<name>` or `<name> <version>`; a breakcheck run records `breakcheck <its version>`, not the template's embedded test-strategy default. Acknowledge briefly only after submit succeeds; disclose storage failure and do not claim feedback was recorded.

```sh
feedback_file="$(mktemp)"   # scratch path, outside the repository
tpp feedback --template --plan "<plan-path>" > "$feedback_file"
# Fill the generated fields; add no fields.
tpp feedback --file "$feedback_file"
```

The skeleton is a scratch file outside the repository, never inside the worktree; the ledger is the record, and the report's Feedback record section names it.

## Output Contract

Reply with the candidate identity, budget, each probe and its disposition, findings, omissions and residual risk, readiness disposition, report path, and whether feedback was recorded or failed.

## Non-goals

Do not add a framework, command, schema, or parser. Do not require a full ladder or whole-repository sweep. Do not fix autonomously. Do not add host or Pi integration.

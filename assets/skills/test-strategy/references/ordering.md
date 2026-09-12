# Order of Work, Stopping, and Routing

## Characterization tests — before touching legacy you do not fully understand

The purpose is not to prove the code is right. It is to make any change in behavior
VISIBLE in a diff instead of discovered in production (Feathers, *Working Effectively with
Legacy Code*: a characterization test documents what the code does, not what it should do).

1. Pick the seam and drive the existing code with representative inputs.
2. Record what it ACTUALLY does — including the results you believe are wrong.
3. Assert exactly that, and label it: `test_characterization_*`, with a comment saying
   this pins current behavior and is not a specification. Unlabeled, someone will later
   read a bug as a requirement and defend it.
4. Now make the change. Every characterization test that reddens is a behavior change:
   intended ones get their assertion updated in the same commit with the reason;
   unintended ones are the bugs you just avoided shipping.
5. When the change lands, promote the ones that encode real requirements into normal
   tests and delete the rest.

This is also the honest answer to "there are no tests and I cannot add them all": you do
not need them all. You need the ones covering the blast radius of THIS change.

---

## Order of work per situation

**New behavior** — write the contract first (inputs, guarantees, errors, invariants),
then one test per equivalence class, test-first where repo policy requires it. The
strategy layer decides WHICH tests exist; it never overrides a test-first policy.

**A bug** — reproduce in a red test BEFORE the fix: a FAIL_TO_PASS test in SWE-bench terms,
red on the defective version, green after the fix, while the existing suite stays PASS_TO_PASS.
Two reasons: it proves you found the cause rather than a symptom, and it is the only test in
the codebase guaranteed to catch this exact regression. Then ask the systemic question — do sibling call sites share the
defect? — and decide whether the class deserves one test or several.

**A refactor with no behavior change** — the tests must not change. Any test you have to
edit was at the wrong altitude; note it as a finding rather than quietly rewriting it.

**Raising confidence in an untested module** — never "cover the module". Take the top of
the ranked list, write one behavioral test per real behavior, mutate to confirm each one
bites, stop when the budget ends, and report where it ended.

---

## Stop rules

Stop when any of these is true:

- The next test's failure would not change what anyone does.
- You cannot state a falsifiable oracle for it (mutation, contract, invariant, negative
  control, differential, metamorphic, or observed state).
- It would only break on a rename.
- The behavior is already pinned by a higher-altitude test that would catch the same
  regression (keep the higher-value one).
- The remaining targets are anti-priorities.

Then state the stopping point explicitly. "We stopped after the top six; the rest of the
list is in the plan, untested, in this order" is a professional result. Silence is not.

---

## Budget framing

Every test is code maintained forever plus CI time on every run for years. It has to earn
that. Treat the effort as a fixed budget spent top-down on the ranked list, and be willing
to say out loud that something low on the list is not worth it — that is a decision, not
negligence, and writing it down is what distinguishes the two.

---

## Plan persistence

The plan lives at `docs/testing/test-plan.md` (or the path the user names), built from
`assets/test-plan-template.md`. Rows are never deleted by budget. The stopping point is
recorded as row status (`pending`, `in progress`, `done`, `blocked`), not as prose.
EXECUTE mode resumes from the first `pending` row; a refreshed PLAN keeps existing statuses.

---

## Scoped runs: what a bounded trigger buys

A bounded trigger — the operator names one area, file or module, or the diff is confined to one or two
files — buys a smaller campaign, never a shallower one. The plan stops inventorying surfaces the
trigger cannot reach. The target it does reach climbs to its rung through its sibling, leaves a pinning
test and an evidence row, and the run closes on `rdd-plus plan check` like any other. A bounded request
and a bounded diff are the same decision reached from two sides: one names the surface, the other shows
it.

- Eligible: "test the reservation math in `src/inventory.js`"; a two-file fix to a pure function whose
documented contract does not change.
- Not eligible: "test the app"; a rename across packages; a change to what a public function promises,
however few lines it takes; anything in authentication, secrets, persistence or money.

`Light:` is a declaration, and what `plan check` verifies about it is deliberately narrow: the declared
shape, a target the plan itself ranks or cites as `path:line`, and a reason for every layer the plan
left out. It cannot tell whether the trigger was really bounded, whether the named classes are true, or
whether the consequence classes were judged correctly; those stay with the operator and the plan's
reader. That is the point of writing the declaration down — one half is machine-checked, and the half
that cannot be is named where the next reader will find it.

---

## Central Routing to Sibling Skills

Routing means **INVOKING the skill** — calling the Skill tool with that name, or reading
`~/.claude/skills/<name>/SKILL.md` — not recalling roughly what it does. Each sibling
carries specialized catalogs, harnesses, and procedures that are not reproduced here;
approximating them from memory degrades the method into generic advice.

| Once target and risk are chosen | Invoke Sibling Skill | Purpose |
|---|---|---|
| High risk, needs real adversarial probing | `exploit-testing` | Layered adversarial tests (L1–L5) to actively break the implementation. |
| Property-based, stateful model-based, differential, metamorphic | `exploit-testing` | Technique catalog and runnable shapes (`Hypothesis`, shrinking, back-to-back). |
| Operational load, latency p95/p99, soak leaks | `runtime-reliability-testing` | Open-model arrival-rate load testing (`k6`) without coordinated omission. |
| Network faults, circuit breakers, retry jitter | `runtime-reliability-testing` | Deterministic socket fault injection with `Toxiproxy`. |
| API boundary fuzzing, smoke journeys | `runtime-reliability-testing` | OpenAPI fuzzing with `Schemathesis`, sub-30s smoke tests with `Playwright`. |
| Business logic authorization, BOLA/IDOR | `appsec-adversarial-auditor` | Dual-persona authorization verification (`403/404` red-under-mutation). |
| Parser safety with untrusted input | `appsec-adversarial-auditor` | Coverage-guided fuzzing (`testing.F`, libFuzzer-style) of parsers and decoders. |
| Secret leaks in diffs, taint tracking | `appsec-adversarial-auditor` | AST taint analysis (`Semgrep`) and high-entropy secret detection (`Gitleaks`). |
| Layer boundaries, clean architecture purity | `clean-architecture-audit` | Automated architecture conformance tests (`pytest-archon`, `dependency-cruiser`). |
| Genuine test resistance, killed mutants on PR diff | `clean-architecture-audit` | Incremental mutation testing (`Stryker`, `Mutmut`) on the PR diff; classify survivors, no numeric target. |
| Code readability, cognitive load ceiling | `clean-architecture-audit` | Enforce Cognitive Complexity at or below 15 and eliminate empty interface theater. |
| Database migrations, up/down idempotency, locks | `database-persistence-testing` | Reversible migration verification, DDL lock inspection, and expand/contract patterns. |
| DB deadlocks, isolation, N+1 query budget | `database-persistence-testing` | Concurrency stress, pessimistic locks (`SKIP LOCKED`), and query count assertions. |
| Infrastructure as Code, Terraform, Cloud SAST | `iac-safe-auditor` | Static linting (`tflint`), misconfiguration scans (`checkov`), and OPA Rego guardrails. |
| Cloud unit tests with mocks, FinOps budget | `iac-safe-auditor` | Zero-risk `terraform test` with `mock_provider` and Infracost cost delta gates. |
| RAG retrieval accuracy (Hit Rate, MRR, NDCG) | `rag-audit-evaluator` | Decoupled retrieval evaluation on annotated chunk golden sets. |
| RAG hallucinations, Faithfulness (NLI) | `rag-audit-evaluator` | Atomic claim decomposition, NLI entailment, and RAG triad scoring. |
| LLM judge calibration, Cohen's Kappa, G-Eval | `ai-evals-auditor` | Symmetric pairwise positional swapping, G-Eval logprob continuous scoring, $\kappa \ge 0.70$. |
| Autonomous agent tool accuracy & loops | `ai-evals-auditor` | Tool precision/recall, action hashing loop detection, and strict schema validation. |
| Adversarial Red-Teaming (Promptfoo/Garak) | `ai-evals-auditor` | Direct/indirect prompt injection, crescendo attacks, and CI budget circuit breakers. |
| No oracle, or quality is unmeasured | `silent-degradation` | Hunt silent data loss and unmeasured degradations. |
| Might never execute in production | `implementation-theater` | Verify code is referenced and alive before spending test budget. |
| A new third-party package appears in the plan | `dependency-legitimacy` | Audit supply chain and hallucinated packages before installation. |
| The test diff is already written and too bloated | `no-excess-tests` | Prune fragile, low-value tests and keep behavioral ones. |
| Final end-to-end reality check of completed change | `real-run-validation` | Exercise real code with real inputs, not just passing mocks. |
| Database / cache ephemeral containers | `docker-test-containers` | Spin up zero-leak ephemeral Postgres/Redis/Mongo containers. |
| Python-specific test patterns and fixtures | `python-testing-patterns` | Idiomatic Pytest, TDD, and test structure. |
| Go-specific teatest, golden files, coverage | `go-testing` | Idiomatic Go test conventions. |

Each of these also activates on its own triggers without passing through here. This table
is the path when starting from the strategic question: *"What should I test, and how?"*

---
name: clean-architecture-audit
description: "Trigger: clean architecture, architecture audit, layer conformance, mutation testing, cognitive complexity, code health, AST linting. Enforce architectural boundaries and code maintainability."
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Load when evaluating code design, architectural changes, PR diffs, or refactoring plans. Objective: prove that code respects layer boundaries, exhibits genuine behavioral test resilience (mutation-verified), and stays cognitively readable without "Clean Architecture Theater" (unearned indirection, 1:1 empty interfaces, mock-mirroring tests).

NOT for: strategic test allocation (`test-strategy`), pruning excessive tests (`no-excess-tests`), or hunting dead unreferenced code (`implementation-theater`).

## Plan Contribution

When invoked by `test-strategy` in PLAN mode: do not execute. Return target rows for the plan:
target (package, module, or symbol) · check (layer boundary, cycle, domain isolation, unearned
interface, survived mutants on high-risk logic, complexity hotspot, swallowed error) · target
depth · consequence class · why. Cover every check this skill would run on this codebase,
including cheap static ones (dependency-cruiser / depguard / archon run, complexity scan).

## Hard Rules

1. **Architecture is executable code, not diagrams**: layer boundaries are verified by automated conformance tests (`pytest-archon`, `ArchUnit`, `dependency-cruiser`, `arch-go`).
2. **Domain isolation is non-negotiable**: the domain core never imports web frameworks, ORMs, or network clients. Infrastructure depends on domain, never the reverse.
3. **No unearned interfaces**: one production implementation and no process boundary → delete the interface, depend on the concrete type.
4. **Behavioral testing over line coverage**: for selected high-risk domain logic use incremental mutation when meaningful; classify survivors (equivalent, unexecuted, flaky, timeout, tooling-invalid) instead of treating each as a gap.
5. **Cognitive complexity signal**: interpret each function against a target-owned/configured threshold with recorded provenance and tolerance; without a target threshold, report a hotspot, not a blocker ([references/conformance-and-complexity.md](references/conformance-and-complexity.md)).
6. **No silent error swallowing**: empty `catch`, `except: pass`, uninspected `?? []`, ignored Go errors are defects.
7. **Evidence**: every finding carries an executed evidence record per `~/.claude/skills/test-strategy/references/evidence.md` (the conformance run, mutation run, or analyzer output); no finding from reading alone.
8. **Stale comment audit**: a comment contradicting the code it annotates is a finding (`~/.claude/skills/test-strategy/references/comments.md`).

## Decision Gates

| Finding | Classification | Mandatory Agent Action |
| :--- | :--- | :--- |
| Domain imports Infra or ORM | **BLOCKER** | Invert: port in application/domain, implementation in infra. |
| Circular dependency ($|SCC| > 1$) | **BLOCKER** | Break the cycle: shared value object, domain events, or inverted dependency. |
| Interface with one implementation (non-I/O) | **THEATER** | Remove the interface; inject the concrete class. |
| High-risk domain logic with survived mutants | **WARNING** | Add the behavioral assertion that kills the exact survivor. |
| Complexity above a configured threshold | **WARNING** | Guard clauses or extracted sub-functions; report threshold provenance. No threshold → hotspot only. |
| Complexity above a configured critical limit | **BLOCKER** | Redesign before merge; same provenance report. |
| Mock asserting only on its own return values | **THEATER** | Real in-memory collaborator, or drop to integration test. |
| Empty catch or swallowed exception | **BLOCKER** | Log, handle explicitly, or rethrow a domain exception. |

## Execution Steps

1. **Static conformance**: run layer boundary checks (`pytest tests/test_architecture.py`, `npx depcruise`, `golangci-lint` with `depguard`); verify zero framework imports in the domain and an acyclic graph.
2. **Cognitive complexity audit** on the symbols in the diff; flag against the configured threshold; remove nesting with early returns.
3. **Targeted incremental mutation** when risk-appropriate: `stryker --since origin/main` or `mutmut run --paths-to-mutate <changed_domain_files>`; classify each survivor; use contract/invariant, negative-control, differential, metamorphic, or observed-state evidence when mutation is not meaningful.
4. **Theater audit**: use cases orchestrate and enforce invariants rather than pass through; remove 1:1 DTOs that add no filtering or transformation.

## Output Contract

Report: architectural violations (file, line, rule) · mutation evidence (candidate gaps and classified survivors, with follow-up evidence) · complexity hotspots (symbol, score, threshold or explicit no-threshold, provenance, nesting breakdown, refactoring diff) · theater and accidental indirection to prune · the evidence record per finding. RDD receipt when required: [references/rdd-receipt.md](references/rdd-receipt.md).

## References

- [references/conformance-and-complexity.md](references/conformance-and-complexity.md) — layer rules, Tarjan SCC cycle math, cognitive vs McCabe complexity.
- [references/rdd-receipt.md](references/rdd-receipt.md) — receipt contract for `lens:architecture`.

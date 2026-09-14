# Architecture Conformance and Cognitive Complexity

## 1. The Architectural Drift Problem

Architecture diagrams decay into aspiration without automated tests. 

### Core Layer Invariants (Clean / Hexagonal):
1. **Domain Isolation**: Domain cannot import `infrastructure`, `adapters`, `ui`, or external frameworks.
2. **Ports Ownership**: The consumer owns the port interface (`domain/ports/` or `application/ports/`). Concrete adapters in infrastructure implement that port.
3. **Cycle Elimination**: The module dependency graph $G = (V, E)$ must be an Directed Acyclic Graph (DAG). Any Strongly Connected Component with $|SCC| > 1$ (Tarjan's algorithm) represents a fatal architectural cycle.

---

## 2. Cognitive Complexity vs Cyclomatic Complexity

### Why McCabe Fails at Maintainability:
McCabe Cyclomatic Complexity counts linearly independent paths. It is completely **blind to nesting**:
- A flat 5-case `switch` statement has $CC = 5$.
- A 5-level nested `if (a) { if (b) { if (c) ... } }` also has $CC = 5$.

### Cognitive Complexity (Campbell, SonarSource):
Quantifies human mental effort. It penalizes nesting levels geometrically:
- Level 0 `if`: +1
- Level 1 `if` (nested inside `for`): +2 (1 base + 1 nesting penalty)
- Level 2 `if` (nested inside `if` inside `for`): +3 (1 base + 2 nesting penalty)

### Conditional Interpretation:
- Compare the score against the target's configured threshold or band, appropriate to the language, analyzer, and version; when the target configures none, compare it against the package default of 15.
- Report the threshold in use and its provenance (target-configured, or the package default of 15), with the sample, measurement conditions, tolerance, rationale, and owner, alongside the score and nesting breakdown.
- Three outcomes, one per score: above the threshold in use → WARNING; above a configured critical limit → BLOCKER; at or below the threshold in use → no classification, reported with the score and its nesting breakdown. A target that configures no critical limit has no BLOCKER outcome.

---

## 3. High-Impact Incremental Mutation Testing

Mutation testing proves assertions are meaningful. Count only comparable, valid mutant outcomes in the score:
$$\text{MSI} = \frac{\text{Confirmed Killed}}{\text{Confirmed Killed} + \text{Survived} + \text{Unexecuted} + \text{Flaky} + \text{Timed Out}} \times 100$$

Equivalent and tooling-invalid mutants are excluded from the denominator, but every category is reported with counts and examples. Timed-out mutants are not killed; preserve their timeout evidence and classify them separately.

### Fast Execution Protocol:
1. **Scope to Diff**: Run only against modified files (`--since origin/main`).
2. **Coverage Matrix**: Execute only the tests that hit the mutated line, and record unexecuted mutants rather than treating them as killed.
3. **Outcome Classification**: Confirm killed and survived outcomes; rerun and classify flaky, timeout, equivalent, and tooling-invalid outcomes explicitly. Do not silently coerce an unresolved or timed-out result into killed.
4. **Mutant Schemata**: Mutate in-memory with boolean flags instead of writing files to disk.
5. **Risk Filter**: Mutate domain algorithms and state transitions; ignore DTOs and configs. Do not require mutation per test or production line.

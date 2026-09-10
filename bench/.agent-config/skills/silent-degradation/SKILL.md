---
name: silent-degradation
description: "Trigger: funciona pero está mal, la precisión es baja y nadie se entera, el sistema dice que todo OK, degradación silenciosa, se pierde data sin error, un filtro o permiso que recorta de más, calidad sin métrica."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
  scope: [common]
  auto_invoke: "Hunting silent quality loss in a system that reports success"
---

## Activation Contract

Load when the system REPORTS SUCCESS and the outcome is still wrong or degraded: RAG
precision quietly low, a filter or permission scope silently dropping legitimate data,
results getting worse with no error, "we have no idea if this is any good", or before
trusting any pipeline whose only health signal is "it ran".

Sibling of `exploit-testing`, which hunts for a red. This one hunts for a GREEN THAT
LIES. NOT for: crashes, exceptions, failing tests, or security exploitation.

## Hard Rules

1. **A system's self-report is a claim, not a measurement.** "OK", exit 0, 200, "0
   errors" say the code RAN, never that the outcome is CORRECT. Never accept them as
   quality evidence.
2. No quality claim without a NUMBER, a frozen goldset it was measured on, and the date.
   "It works well" is not a result.
3. **Validate the metric before the system**: deliberately break the pipeline (empty the
   index, shuffle the retrieval, drop the reranker, corrupt an input) and confirm the
   metric DROPS. A metric that stays green on a broken system measures nothing — that is
   the first finding, and it outranks everything else.
4. Every stage reports **in / out / dropped, with a named reason per drop**. An
   unattributed drop is a defect, whatever the final output looks like.
5. Every filter, permission scope and truncation must be probed WITH and WITHOUT it, and
   the difference set inspected item by item. Over-restriction is invisible by design.
6. Never conclude from one sample or one run. Nondeterministic output is judged over N
   runs and over the goldset.
7. Report as a defect anything the system CANNOT observe about itself — a missing
   counter is why nobody noticed.

## Decision Gates

| Symptom | Attack |
|---|---|
| No idea whether quality is good | Build the goldset, measure, freeze the baseline — `references/measurement.md` |
| Quality got worse, no error anywhere | Stage counters + drop attribution — `references/silent-loss.md` |
| Everything green, output still bad | Break it on purpose; if the metric holds, the metric is the bug (Rule 3) |
| Don't know WHICH stage loses quality | Ablation + ceiling analysis — `references/measurement.md` |
| Suspect a filter/permission over-restricts | With/without diff of the result sets — `references/silent-loss.md` |
| Nobody would notice a regression tomorrow | Wire the metric to CI/alerting as a threshold, not a dashboard |

## Execution Steps

1. Write the OUTCOME the system exists to produce, and the number that would prove it.
   Not "ran", not "no errors" — the result quality.
2. Inventory every silent-loss site along the path (`references/silent-loss.md`) and
   instrument it: in/out/dropped + reason.
3. Build or freeze a goldset; measure the baseline; record the number and the date.
4. Validate the metric with a deliberate break (Rule 3) before believing any of it.
5. Attribute the loss: ablate each stage; replace each stage with a perfect oracle to see
   the ceiling. The stage whose removal changes nothing is broken or unmeasured.
6. Diff every filter/permission with and without it; justify every excluded item.
7. Fix the OBSERVABILITY GAP too, not just the defect: leave the counter and the
   threshold behind so the next regression announces itself.

## Output Contract

The stage table (stage → in → out → dropped → reason), the baseline number with its
goldset and date, the metric-validation result (what broke, how much it dropped), the
ablation table, and defects split into `pérdida de calidad` vs `ceguera del sistema`.
Say plainly what remains unmeasured.

## References

- [references/silent-loss.md](references/silent-loss.md) — where data and quality vanish without an error.
- [references/measurement.md](references/measurement.md) — goldsets, baselines, metric validation, ablation, ceiling analysis.
- [assets/quality-audit-template.md](assets/quality-audit-template.md) — audit artifact.
- `~/.claude/skills/test-strategy/SKILL.md` — what deserves testing at all (the router).
- `~/.claude/skills/exploit-testing/SKILL.md` — adversarial sibling (hunts reds).
- `~/.claude/skills/implementation-theater/SKILL.md` — sibling for code that runs but does nothing (a dead knob or a silent fallback is often the cause of the quality loss).

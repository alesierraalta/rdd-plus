# Dogfooding log — our own tool, used on our own work

One entry per real development task where breakcheck was applicable. The question this log answers is not
"did it run" but **"did the work get better because it ran"**, and the target is the sentence: *I would
rather not merge this code without passing it through the tool first.*

An entry also exists when the tool was NOT used: the reason matters as much as a finding.

---

## 2026-09-18 — #104, score the plan a workspace declares

**Task**: fix the benchmark's plan-path blind spot (a real bug found by forensics on the previous
iteration's measurement), on a real branch, before its PR.

**Tool used**: Yes — `breakcheck`, bounded campaign on the working tree.

**Findings**

| Finding | Verdict | What happened |
|---|---|---|
| F-104-1: a declared-but-absent plan is reported as a plain `no plan`, hiding the declaration | **USEFUL (limited)** | Recorded in the report with its reproducer. Not fixed here: it is pre-existing and outside the change's blast radius, and it is a diagnosability gap rather than a wrong score. |
| F-104-2: a workspace symlink lets the scorer read a plan outside the workspace | **LOW VALUE** | Pre-existing, limited impact for a benchmark corpus we control, and the scorer only parses the file. Recorded, not chased. |

**True positives**: 2 limited findings, 0 of them caused by the change.
**False positives**: 0.
**Missed issues**: none known yet. The independent verifier found the wording half of F-104-1 that this
campaign had missed — `bench/README.md` claimed "no plan anywhere" while the code selects the declared
path — which is a fair score against the campaign, and the reason the two are reported together here.
**Changes made because of the tool**: 2 wording corrections, both from F-104-1 — `bench/README.md` now
states the selected-path rule and the `NoPlan` comment no longer claims the fixed path. The fix itself was
not changed: the campaign added two recorded risks and one documentation correction, not a defect repair.
**Previously run by me**: focused suite (`./internal/bench/...`, `./internal/plan/...`), build, gofmt, and
a before/after over the two real kept workspaces. The tool's two findings were **not** among them,
including one gap the writer's own tests missed (declared-absent-with-default-present).

**Main friction**: three commands to run the campaign (copy, probe file, run) plus reading five probe
lines. No configuration, no model spend, no waiting. Cheap enough to repeat per task.

**Honest note on value**: the campaign's headline result here is *negative* — it did not find a defect in
the change, and one of its two findings is low value. Its value came from covering the one case the
tests skipped, which is exactly the gap this tool exists for, and from the low cost of getting it.

**Conclusion**: keep using it on changes of this kind. It is not yet at "I would not merge without it",
because both findings were limited and pre-existing; the check that decided this task's readiness
remains the focused suite plus the real-workspace before/after.

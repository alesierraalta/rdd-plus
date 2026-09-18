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

---

## 2026-09-18 — #106, name the declaration a missing plan overrode

**Task**: make a declared-but-absent plan report the declaration it honoured and the default plan it ignored,
on a real branch, before its PR. Same function as the previous entry's fix.

**Tool used**: Yes — `breakcheck`, five probes on the working tree.

**Findings**

| Finding | Verdict | What happened |
|---|---|---|
| F-106-1: the new note said `was not found` about a declared path that **exists** (an unreadable file, or a directory) | **TRUE POSITIVE — material, introduced by this change** | Fixed in this PR: the note now distinguishes an absent path from one that exists and was not read, and a test pins it. **The tool found a defect in the code under review, not in code that already shipped.** |
| F-106-2: the plain scorer treats an unreadable plan as absent while the adjudicated scorer refuses, because `ScorePlanFile` discards the read error | **USEFUL (limited)** | Pre-existing and outside the change's blast radius; recorded with its reproducer, candidate for a follow-up issue. |

**True positives**: 1 of them material and caused by this change; 1 limited and pre-existing.
**False positives**: 0.
**Missed issues**: none known; the writer's tests had covered absent files only, and this campaign covered the
case they skipped — the same shape of gap as the previous entry, which is now twice in a row.
**Changes made because of the tool**: the note logic was corrected and a test added, i.e. **the code shipped
is different, and truer, because the campaign ran**. This is the first entry where the tool changed behaviour
rather than documentation.
**Previously run by me**: focused suite, build, gofmt, and a review of the diff — none of which covered an
existing-but-unreadable declared path.

**Main friction**: none new. Copy, probe file, run: three commands, no configuration, no model spend. The
probe that found the defect took one `chmod` line to write.

**Conclusion**: this is the first time the tool earned its place on its own: it found a defect in the change
being reviewed, at a cost of minutes, and the fix is pinned by a test. Still not "no merge without it" — one
material finding in two tasks is a real signal, but the sample is two. Worth watching whether the next tasks
repeat the pattern, and worth noting what made it work: the campaign attacked the **new** logic, not the code
around it.

---

## 2026-09-18 — #107, refuse a plan that resolves outside the workspace

**Task**: make the bench refuse a plan path that a symlink resolves outside the workspace, on a real branch,
before its PR.

**Tool used**: Yes — `breakcheck`, five probes on the working tree.

**Findings**

| Finding | Verdict | What happened |
|---|---|---|
| F-107-1: a symlink **loop** fell through the guard's early return into the absent flow, and the note then claimed `was not found` about a path that exists | **TRUE POSITIVE — material, caused by this change** | Fixed in this PR: a non-`ErrNotExist` resolution failure is refused with a note naming it, pinned for both scorers. |
| Chains, a symlinked workspace, in-workspace links, plain files | **CONFIRMED CORRECT** | The guard follows chains to the real target, does not refuse a workspace reached through a symlink, and leaves legitimate links alone. |

**True positives**: 1, material and caused by this change.
**False positives**: 0.
**Missed issues**: none known at campaign time — but note *why* this one appeared: a symlink **loop** was not in
the task document's own list of test cases either. The campaign found the case that the specification, the tests
and my review all skipped, which is now three tasks in a row with the same shape. (The independent verifier
later found three further holes: see the note at the end of this entry.)
**Changes made because of the tool**: the loop is refused instead of reported as missing. Second time the code
that ships is different because the campaign ran.
**Previously run by me**: focused suite, build, gofmt, vet and a full diff review — none of which exercised a
link that cannot be followed.

**Main friction**: none. The probe that found this was six lines, and the whole campaign stayed inside the
throwaway-copy pattern already in use.

**Conclusion**: the tool has now earned its place twice on the same day, both times by attacking what the change
introduced. The pattern is stable enough to name: **write the probes for the cases the task document does not
list**. The value is not in finding bugs already shipped — it is in finding the one the author already
convinced themselves about.

**What the verifier added, and what it costs this entry's story**: the independent verification found **three
holes this campaign missed** — the guard skipped containment when the workspace root could not be resolved, the
kept-copy step still read a refused path, and a rejected declaration returned before the guard so it could read
an escaping default — plus loose assertions in the loop test. All three were closed before the PR. So the
campaign's score on this change is one material finding of its own and three it did not see, and the phrase "no
missed issues" in the table above was written before the verifier ran. Keeping that sentence as written, with
this paragraph under it, is the point of the log: a tool that is trusted without being checked is the failure
mode we are trying to avoid.

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

---

## 2026-09-18 — #109, name a failed plan read instead of calling it absent

**Task**: stop the plain scorer from reporting a plan that exists but cannot be read as if it were absent, and
pin the one difference that legitimately remains between the two scorers. The branch also had to carry a repair:
`master`'s suite was red because #107's commit omitted the guard in `adjudication.go`.

**Tool used**: Yes — `breakcheck`, six probes on the working tree.

**Findings**

| Finding | Verdict | What happened |
|---|---|---|
| — | **NONE** | Every probe matched its expectation: the failure note names the cause in both scorers, the declaration note stands down instead of repeating it, an absent plan is unchanged, and the refusal contract is restored identically in both scorers. |

**True positives**: 0.
**False positives**: 0.
**Missed issues**: unknown at campaign time — and the honest framing is that this run was cheap because the design was already pinned by the specification: six probes confirmed it rather than exploring it. Whether it missed anything is the verifier's question, not this campaign's.
**Changes made because of the tool**: none. The one defect in this branch — `master` red, and worse, the adjudicated scorer reading a path the plain one refuses — was found by the writer's own test run and the parent's investigation of it, not by probing.
**Previously run by me**: the focused suite (green here, red on `master`), build, gofmt, vet, and a diff review.

**Main friction**: none.

**What this entry is worth**: a campaign that finds nothing is only evidence if the probes could have failed.
These could: the note-composition probes are exactly where the previous two campaigns found their material
defects, and P5 checks a behaviour that `master` demonstrably lacks. So the honest read is "the change is clean
within its scope", not "the campaign earned its keep today". The expensive part of this task was elsewhere —
noticing that a verified revision had not reached the commit.

**Conclusion**: keep running it, and keep the expectation honest. Two material findings in four tasks, and one
task where the tool found nothing while a broken `master` sat behind a green worktree.

**Closed after the merge.** PR #111 landed as `970fc23` and the merged result was verified in a clean copy:
`internal/bench` and `internal/plan` pass, all sixteen refusal subtests that were red now pass, and the build is
clean. The loop #107 left open is closed.

**Two process rules this arc produced**, both from failures of mine rather than of the tool:

1. **Verification covers a revision, not a commit.** Freezing hashes on a worktree proves nothing about what
   ships if the commit is assembled by hand: #107's guard sat verified-but-uncommitted while `master` went red.
   After committing, compare each committed blob against the frozen hashes (`git show HEAD:<path> | sha256sum`).
2. **A merge is not verified by the worktree it came from.** Run the suite on the merged result — a clean copy of
   `origin/master` after the merge — or a red `master` stays red behind a green branch.

---

## 2026-09-18 — CI: make those two rules mechanical

**Task**: add the workflow that runs the suite on the pushed commit, so rule 2 stops depending on my memory.

**Tool used**: Yes — `breakcheck`, five probes on the workflow, the Makefile and the README.

**Findings**

| Finding | Verdict | What happened |
|---|---|---|
| — | **NONE against the candidate** | The probes checked the claims instead of the file: the command set fails at the commit that broke `master` (exit 1), the `gofmt` step fails on an unformatted tree, the triggers are what they say, and the YAML parses. |
| `make vet` printed unformatted files and exited 0 | **USEFUL (pre-existing)** | Found while probing the new `gofmt` step: the local lint a developer runs reported the problem and still succeeded. The Makefile in this change makes it fail. |
| Without `node`, the JavaScript half of the suite **skips and the run reports PASS, exit 0** | **USEFUL (design decision made measurable)** | Probe P3 on a compiled test binary with `node` removed from `PATH`. This is why the workflow installs node: otherwise CI would be green over a suite that quietly verified less. |

**True positives**: 0 in the new file; 2 real facts about the repository that the probes turned up.
**False positives**: 0.
**Missed issues**: unknown at campaign time. The honest framing: most of this task's value came from probes that the
campaign prescribes and that I ran as part of it — the difference between "the workflow exists" and "the workflow
would have caught the failure that motivated it".
**Changes made because of the tool**: the node step and its comment exist because a probe measured the skip, not
because the README says node is optional.
**Previously run by me**: the full suite in a clean copy of `master` (13 packages, 69s) to define what CI should run.

**Main friction**: none. Every probe was a shell command in a throwaway copy.

**Conclusion**: the campaign found no defect in the artifact and two facts about the repository it lives in, which
is the useful shape of a "clean" result: the file passed, and what the file depends on got measured instead of
assumed.

---

## 2026-09-18 — feedback record fidelity: where it lands, and what it says ran

**Task**: act on the review of the 178 accumulated retrospectives (that count was the ledger's size when the review ran) — a benchmark run's feedback no longer lands
in the operator's ledger, and the `skill` field stops carrying eight incompatible shapes.

**Tool used**: Yes — `breakcheck`, six probes on the change.

**Findings**

| Finding | Verdict | What happened |
|---|---|---|
| F-114-1: an operator running under Pi with `PI_CODING_AGENT_DIR` set now reads a different ledger; a history under `~/.claude` looks empty | **USEFUL (limited)** | Accepted with documentation: the README now states the resolution order and that a ledger elsewhere is reached with `--config-dir`. Fixing it in code would mean guessing which ledger the operator meant. |
| F-114-2: the shape is enforced on the CLI write path only — `feedback.Record()` still stores whatever a Go caller passes | **LOW VALUE** | Recorded, not chased: the CLI is the only production writer and `Record` is the internal API the tests use. |
| The README's own example named `breakcheck 0.1.0` while this change bumps the skill to `0.1.1` | **USEFUL (low)** | Fixed in this PR: the example now says `<its version>`, so the next bump cannot falsify it again. |

**True positives**: two limited and one low, all recorded; the load-bearing claim was confirmed rather than
repaired.
**False positives**: 0.
**Missed issues**: unknown at campaign time — the verifier's question, not this campaign's.
**Changes made because of the tool**: the stale example, and the sentence that tells an operator how to reach a
ledger living under another directory.
**Previously run by me**: the focused suite (`feedback`, `cmd`, `bench`, `assets`), build, gofmt, vet, and a
review of the diff.

**Main friction**: none from the tool. Two probes had to be rerun because I wrote them wrong — I submitted an
unfilled template, and my accept/refuse comparison printed ❌ for four probes that had behaved correctly. That is
my friction, not the tool's, and it is the kind of thing this log exists to keep visible: a probe's own bug looks
exactly like a finding until it is checked.

**Conclusion**: the probes here are cheap and the load-bearing one is end-to-end (the operator's ledger stays at
the operator's ledger gained no rows while the isolated one gained the record (178 rows at that moment, 179 when the verifier reproduced it), which is what a hygiene change needs. Worth noting what the
campaign did **not** do: it did not migrate the 137 rows already written, because rewriting an append-only ledger
to look tidy contradicts what it is for.

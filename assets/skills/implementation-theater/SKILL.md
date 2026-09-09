---
name: implementation-theater
description: "Trigger: esto realmente se usa, código muerto, nadie lo llama, está cableado, es un stub, finge que funciona, el flag está apagado, hay dos implementaciones, el nombre miente."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
  scope: [common]
  auto_invoke: "Proving code is real: reachable, doing the work, unique, and honest"
---

## Activation Contract

Load to decide whether code is REAL or PROPS: nothing calls it, a flag or env may keep it
off, it may be a stub that fakes the work, there may be a second copy that actually runs,
or its name/docs may claim something the body does not do. Standard pass over
AI-generated or long-lived code before trusting or deleting it.

Siblings: `exploit-testing` (hunts reds), `silent-degradation` (hunts a lying green).
This one hunts CODE THAT ISN'T DOING ANYTHING.

## Hard Rules

1. **"No callers" from a tool is a HYPOTHESIS, never a verdict.** Detection is easy;
   triage is the whole job. Resolve every candidate into exactly one: dead ·
   dynamically loaded · referenced from config/DI · public API for a consumer ·
   merged-but-never-wired. Never delete on a tool's say-so.
2. Reachability is proven FORWARD from the production entry point, and the evidence is an
   EXECUTION that touches the line (coverage, log, counter, trace) — not a call graph you
   read. State the highest rung proven (`references/reachability.md`, R0–R5).
3. **Merged ≠ running.** Flag state, env var, route mounted, DI registered, migration
   applied, consumer/cron actually up: all are part of reachability.
4. A branch you cannot make execute is dead OR untestable. Both are findings; decide which.
5. **Prove every knob is connected**: set it to an absurd value and observe a change. A
   parameter, timeout, threshold or config that changes nothing is decorative — report it.
6. Every swallowed error is a lie until proven otherwise (`except: pass`, empty catch,
   silent default, `?? []` with no counter).
7. Two implementations: find which one RUNS, prove the last fix landed there, and report
   the divergence — the duplication is not the finding, the DIVERGENCE is.
8. Names, docstrings, READMEs and PR text are CLAIMS. Falsify each against the line that
   would make it true. No such line → the claim is false, and that is a defect.

## Decision Gates

| Suspicion | Move |
|---|---|
| Nothing seems to call it | R0–R5 chain, then the triage taxonomy — `references/reachability.md` |
| It runs but may not do the work | Fakery catalog + the knob test — `references/fakery.md` |
| It works in tests, unknown in prod | Rung R3/R4: flag, env, wiring, then a counter in the real path |
| Two similar implementations | Divergence diff — `references/clones-and-claims.md` |
| The name/doc promises something | Claim falsification — `references/clones-and-claims.md` |
| It is genuinely dead | Prove R-level 0, then delete; deleting is the fix, not a comment |

## Execution Steps

1. List the production entry points (route table, CLI, worker, cron, composition root).
2. For each symbol under audit, climb R0→R5 and record the highest rung PROVEN.
3. Triage every unreachable candidate into the five-way taxonomy (Rule 1).
4. Walk the fakery catalog over the reachable code; run the knob test on every parameter,
   timeout and threshold it exposes.
5. Diff every near-duplicate; establish which copy runs and whether they have diverged.
6. Falsify every claim: name, docstring, README, PR description, comment.
7. Report; for confirmed dead code, delete it in the same change.

## Output Contract

The reachability table (symbol → highest rung proven → triage verdict → evidence), then
findings in three buckets: `no se ejecuta` · `se ejecuta pero finge` · `miente sobre sí
mismo`. Each with the execution evidence, not a reading. Name explicitly what stays at
rung R2 or below — unproven in production is a legitimate and useful verdict.

## References

- [references/reachability.md](references/reachability.md) — R0–R5 chain, triage taxonomy, deployment reality.
- [references/fakery.md](references/fakery.md) — stubs, error masking, disconnected knobs.
- [references/clones-and-claims.md](references/clones-and-claims.md) — divergent copies, false names and docs.
- [assets/theater-audit-template.md](assets/theater-audit-template.md) — audit artifact.
- Siblings: `~/.claude/skills/test-strategy/SKILL.md` (the router) · `exploit-testing` (hunts reds) · `silent-degradation` (green that lies) · `dependency-legitimacy` (is the dependency real).

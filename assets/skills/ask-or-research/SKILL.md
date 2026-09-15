---
name: ask-or-research
description: "Trigger: asking the user a question, clarification, requirements, 'pregúntame', preguntas técnicas, curiosity. Ask only what changes the outcome, at the level of intent and risk; research what you can find yourself."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
---

## Activation Contract

Apply before every question to the operator and before every investigation: ask, or find out. A
question spends someone's attention, so it earns its place only when the answer changes what gets
built, what it costs, or which risk is accepted. Curiosity is required — the failure is asking the
wrong person the wrong thing, or asking nothing when a fork was real.

## Hard Rules

- Ask only what you cannot find. Facts about this repository, its history, the docs, the dependencies,
  or the practice everyone else uses are yours to find; report the finding in one line instead of
  asking for it.
- Ask only what forks the work. If the answer changes nothing about the deliverable, the risk, or what
  "done" means, decide it and state the assumption in one line.
- Ask at the altitude of intent: outcome, priority, appetite for cost and risk, scope. Never naming,
  file layout, helper shape or field order — those are the architect's decisions, reported, not
  delegated upward.
- Every option states what it buys and what it costs, in words a person can weigh without reading the
  code. Two to four options, the recommendation first with its reason.
- Never hand the operator a machine value to compose: relay it verbatim, or resolve it yourself. Never
  re-ask what was already answered.

## Decision Gates

| Situation | Action |
|---|---|
| The fact is in the repo, the docs, or the web | find it; continue with one line naming the finding |
| The best practice is knowable | research it; propose it with its trade-offs |
| The answer changes behaviour, cost, or accepted risk | ask, at the level of intent |
| The answer changes only the implementation | decide it; disclose the decision in one line |
| Unsure whether the fork is real | research until it is concrete, then ask once |
| Several questions are pending | batch them in one invocation |

## Execution Steps

1. Before asking, write the answer you expect and check whether you can verify it yourself.
2. If you can, verify it and keep going. If you cannot, phrase the question around the outcome it
   decides, never the mechanism you would use.
3. Ask once: options ranked, costs stated, recommendation first.
4. When the answer arrives, restate the decision in one line and proceed; do not reopen it in this task.

## Output Contract

A question the operator can answer from what they want, without knowing the internals: the decision at
stake, two to four outcomes, what each buys and what each costs, and the recommendation with its
reason. Research done instead of asking is reported in one line, never narrated.

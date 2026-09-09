---
name: real-run-validation
description: "Trigger: terminé una implementación, validar que funciona de verdad, prueba real, ejercitar end-to-end, real-run, prove it runs. After finishing an implementation, drive the real code with real inputs and observe behavior — not just trust asserts."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
---

# Real-Run Validation

## Activation Contract

Run after finishing an implementation that touches product source, BEFORE
declaring it done or opening a PR. Green unit tests are necessary but not
sufficient: asserts prove the cases the author imagined; a real run reveals
what actually happens. Skip only for pure docs/comment/test-only diffs (no
runtime surface to drive).

## Hard Rules

- Exercise the REAL built artifact (installed module / running service / real
  CLI), never a mock or reimplementation.
- Use REAL, representative inputs — a plausible payload, secret, file, or
  request — not `foo`/`bar` placeholders.
- OBSERVE and report actual outputs and side effects; do not infer success
  from exit code alone.
- Explicitly probe the failure path (bad input, tamper, wrong auth) and
  confirm it fails as intended — fail-closed, not silently.
- State plainly what was validated vs what was NOT reachable this way.
- Never invent output. If you cannot run it, say so and stop.
- Evidence: every finding carries an executed evidence record per
  `~/.claude/skills/test-strategy/references/evidence.md`; no finding from reading alone.
- Put throwaway drivers in a scratchpad/temp dir, never in the repo diff. Remove them only after previewing the exact owned path and explicitly confirming destructive deletion; retain rollback/inspection artifacts when required and report skipped ambiguous items.

## Plan Contribution

When invoked by `test-strategy` in PLAN mode: do not execute. Return target rows for the plan:
target (journey or runtime surface) · check (real-artifact drive: happy path plus one failure
path) · target depth · consequence class · why. Cover the two or three journeys the business
cannot lose and every runtime surface the change touches.
Also fill the plan's "Real-run recipes" table for every journey you contribute (start command, data setup, sample requests, expected observable); EXECUTE reuses it instead of rediscovering ports, seeds, and codes.

## Decision Gates — how to drive by runtime surface

| Surface | How to drive it |
|---------|-----------------|
| Library / pure function | Throwaway driver script that imports the real module; run round-trip + failure case (see `assets/driver_template.py`) |
| HTTP endpoint / API | Real request (curl / http client) against the running service; inspect status + body |
| CLI | Invoke the built binary with real args; check stdout/stderr/exit + effect |
| DB / migration | Apply, then query to confirm the observable state changed |
| UI / flow | Drive the actual flow (the `run` skill); screenshot or read resulting state |

## Execution Steps

1. Identify the runtime surface of the change (table above).
2. Build/install so the driver hits the REAL artifact (right venv/deps).
3. Write a minimal driver with real inputs covering: happy path (exact
   round-trip / expected effect) AND at least one failure path.
4. Run it. Capture verbatim output.
5. Compare each observed behavior against the intended contract.
6. Report the observation table; flag any mismatch as a defect, not a nit.
7. Preview scratchpad cleanup and explicitly confirm deletion of only the owned driver/artifacts within a bounded target; retain rollback/inspection artifacts and report ambiguous or skipped items. Keep all drivers out of the commit.

## Output Contract

Return a compact table: behavior tested → observed result → matches contract?
State the two validation layers explicitly (deterministic tests vs observed
real run), and name anything the real run could not exercise.

## References

- `assets/driver_template.py` — starter driver: real inputs, happy + failure path.

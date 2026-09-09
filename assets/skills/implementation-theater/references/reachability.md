# Proving code actually runs

Static tools (Knip, Vulture, `deadcode`, CodeGraph blast radius) produce CANDIDATES.
Their known weakness is triage, not detection: a symbol with no callers may be loaded
dynamically, named in a config file the analyzer cannot parse, or be public API for a
consumer outside the repo. Treat their output as the start of the work.

## The R chain — climb it, then state the highest rung PROVEN

| Rung | Question | Evidence that closes it |
|---|---|---|
| R0 | Does it exist and import in the prod build? | The prod entry point imports it without error |
| R1 | Is there a static path from an entry point? | The call chain, named hop by hop (CodeGraph) |
| R2 | Is it WIRED? | It appears in the composition root / DI container / route table / registry / event subscription |
| R3 | Does the deployment ENABLE it? | Flag on for the audience, env var set, migration applied, worker/cron actually running, config default in prod |
| R4 | Does an execution touch the line? | Coverage from a real run, a temporary counter, a log line, a trace span |
| R5 | Does it run in PRODUCTION? | A nonzero counter / log / span from prod traffic |

A claim of "this works" is only as strong as the highest rung proven. R1 is where most
reviews stop and where most theater survives: the call chain reads fine and the flag is
off. **The cheapest honest step is R4** — add a counter or a log at the line, drive the
real entry point, confirm it fires, then remove the probe or keep it (`silent-degradation`
argues for keeping it).

Write the rung in the report. "R2 proven, R3 unverified — the flag default in prod is
unknown" is a real, useful answer. A confident "it works" backed by R1 is not.

## Triage taxonomy — every unreachable candidate lands in exactly one

| Verdict | How you confirm it | Action |
|---|---|---|
| **Dead** | No static path, no dynamic lookup, no config mention, no external consumer | Delete it in this change. A comment is not a fix |
| **Dynamic** | Reached via reflection, a plugin/registry decorator, `getattr`, a string-keyed dispatch, a framework hook, a serializer by name | Not dead. Add a test pinning the dynamic key, because a rename breaks it silently |
| **Config-referenced** | Its name appears in YAML/JSON/env/IaC/DB rows/a template | Not dead. The config is now part of its contract — grep the config repo too |
| **Public API** | Exported and consumed outside this repo | Not dead. Deprecate with a version, never delete silently |
| **Never wired** | Exists, tested, and no path reaches it: nobody constructs it, no route mounts it, the flag was never turned on | **The core finding of this skill.** It was merged and never released |

Search wide before declaring anything dead: the symbol name as a STRING (config, SQL,
templates, IaC, migrations, feature-flag definitions, other repos), not just as an
identifier. A grep restricted to source files is how a dynamically-loaded class gets
deleted.

## Deployment reality — R3 in detail

Merged is not running. Check each, per environment, and say which environment you checked:

- **Feature flag**: what is the DEFAULT in prod, for which audience, and who owns it?
  Flags decouple deploy from release by design; a merged flag that was never turned on is
  code that has never run. "Zombie flags" — logically dead branches left embedded — are a
  documented, common outcome.
- **Env / config**: is the variable actually set in the deployed environment, or is the
  code silently taking its default? Print the value the process really sees.
- **Wiring**: the class is constructed, the route is mounted, the handler is subscribed,
  the port has an adapter bound in the composition root. A new adapter nobody binds is
  the most common never-wired shape in a hexagonal layout.
- **Schema**: was the migration applied in that environment, in that order?
- **Runtime**: is the worker process, the consumer, the cron actually up? A queue with no
  consumer accepts messages forever and looks healthy.
- **Build**: is it in the shipped bundle at all? Tree-shaking, `--production` filtering,
  and per-environment builds can drop it.

## Delete with evidence

When the verdict is Dead, deleting IS the fix — it removes a permanent false signal from
the codebase. Ship the deletion with the evidence that justified it (the searches run,
the config repos checked, the consumers asked). If you cannot produce that evidence, the
verdict is not Dead; it is "unproven", and it stays.

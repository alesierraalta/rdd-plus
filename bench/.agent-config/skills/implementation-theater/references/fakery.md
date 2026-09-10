# Code that runs and does not do the work

Reachable is not the same as real. These are the shapes that execute happily while
producing nothing, and they are measurably rising in AI-assisted code — error masking is
one of the risk signals GitClear now tracks across 200M+ lines.

## The knob test — run it on EVERY parameter, threshold and config option

The fastest way to find a disconnected implementation: **set the knob to an absurd value
and observe.** `timeout=0`, `top_k=1`, `max_retries=0`, `limit=1`, a threshold at 1.0, a
flag inverted. If behavior does not change, that knob is decorative and every caller
believing it configures something is wrong.

This finds a class nothing else does: the parameter accepted by the signature, passed
down two layers, and then never read. The caller is confident, the code is green, and the
setting has never had any effect.

Same test for a config file: change a value, restart, observe. No change means the config
is not loaded, is overridden downstream, or is read once at import before the override.

## The fakery catalog

**Hardcoded and placeholder returns** — a constant where a computation belongs, sample or
seed data reachable from a prod path, `return []` / `return None` / `return True` as a
stand-in, a "response" assembled from the request without consulting anything,
`NotImplementedError` on a path that is actually called.

**Error masking** — `except: pass`, `except Exception:` with a debug-level log or none, an
empty catch block, `try/catch` returning a default, `?? []`, `.get(k, fallback)` on a key
that should always exist, a retry wrapper that swallows the final failure. Each one turns
a defect into a silent wrong answer. Rule: every swallowed error emits a counter with a
reason, or it is a finding.

**Always-one-way branches** — `if False`, a flag never enabled, a condition on a value
that is constant in practice, an `else` that cannot be reached, a `while` that never
loops. Prove it by forcing the other branch: if you cannot make it execute, say so.

**Functions that assert nothing** — a `validate_*` that never rejects, a `check_*` that
returns `True` unconditionally, a sanitizer that returns its input, a permission check
that only logs. Test them with input that MUST be rejected.

**Machinery that isn't** — a cache that never hits (wrong key, per-request instance), a
rate limiter with no effective limit, a "retry" with `max_attempts=1`, a connection pool
of size one, an index never used by the query planner, a debounce that fires every time,
an async task created and never awaited, a lock acquired on a per-call object.

**Metrics and logs that go nowhere** — a counter emitted but never registered/exported, a
log at a level the deployment filters out, a span with no exporter configured, a
correlation id generated but not propagated. This class is what makes the other classes
invisible; hunt it first when nothing anywhere seems to explain a symptom.

**Self-admitted debt** — `TODO`, `FIXME`, `XXX`, `HACK` sitting in a reachable path. The
research view is useful here: these markers are high-precision and low-recall, so treat
every hit as real and never assume their absence means the code is finished. Grep them,
then ask of each: is this path reachable in prod, and what does it do when it hits the
unfinished branch?

## How to prove it, not argue it

For each candidate, the evidence is an execution:

1. Drive the entry point with an input that MUST exercise the line.
2. Observe the effect that proves the work happened — the row written, the request sent,
   the value changed, the error raised — never the return value alone.
3. Then invert: break the thing it depends on (empty the cache, kill the dependency,
   pass an invalid value). If the output is identical either way, the code is not doing
   what its position in the flow implies.

Step 3 is the whole method compressed: **a component that produces the same result when
its input is destroyed is not participating.**

# Where quality vanishes without an error

Walk the whole path from input to outcome and mark every site below. Each one can drop,
truncate, coerce or replace data while every log stays green. Instrument each with
**in / out / dropped + reason**; a drop nobody can name is the finding.

## The catalog

**Filters and scopes** — a `WHERE`, a tenant/permission scope, a status or date filter,
a visibility rule, a feature flag. The failure mode is OVER-restriction: correct-looking,
silently removing legitimate items. Applied twice (once in the repository, once in the
service) it halves the result and nothing complains.

**Joins and lookups** — an inner join where one side is incomplete silently deletes rows;
a `.get(key, default)` turns a missing key into a plausible value; a lookup miss becomes
`None` and then becomes "no data" downstream.

**Truncations** — `LIMIT`, `top_k`, max tokens, chunk size, batch size, a
`[:100]` slice, a column width, a payload size cap, a context window. Truncation is
lossy and almost never logged. Ask at each: could the answer have been at position k+1?

**Deduplication and normalization** — a dedup key that is too coarse merges distinct
items; lowercasing, accent stripping, unicode normalization or trimming can collapse two
different values into one, or break an exact match that used to work.

**Fallbacks and defaults** — `except Exception: return []`, a default config used
because the real one failed to load, a cached/stale value served on error, a secondary
provider silently substituting the primary, an empty result treated as a valid answer.
Every fallback must announce itself; a silent fallback is a permanent quiet outage.

**Coercions** — float→decimal precision loss, tz-aware→naive datetimes, a number parsed
from a string with the wrong locale separator, an enum value that does not match falling
into an `else`, encoding mismatches turning text into mojibake that no longer matches.

**Ordering** — a query with no `ORDER BY` feeding a `top_k` returns a different (and
sometimes worse) set on every run. Nondeterministic ordering plus truncation equals
random quality.

**Async and partial work** — a batch where some items failed and the job still reports
success; a fire-and-forget task whose failure nobody observes; a retry that eventually
gives up quietly; a queue where one tenant's backlog starves the rest.

**Model and index mismatch (RAG/ML specific)** — the embedding model at index time
differs from query time; the index is stale after an update; deleted documents remain
retrievable; the distance metric differs from the one used to build the index; the
chunker splits the answer across two chunks so neither scores high; the reranker is
configured but never actually applied. All of these produce a working system with
quietly bad retrieval and zero errors.

**Prompt and context assembly** — context truncated to fit the window (which chunks got
cut?), the system prompt overwritten by a default, history crowding out the retrieved
context, a template variable rendering empty because the key was missing.

## The counting principle

For every stage: `in`, `out`, `dropped`, and a reason label per drop.
`out + dropped == in` must hold, and every unit in `dropped` must carry a reason a human
would accept. When the numbers do not add up, stop and find the leak before measuring
anything else. This single table finds more silent loss than any amount of reading.

## The with/without diff — for every filter, scope and truncation

Run the pipeline with the restriction and without it, then inspect the DIFFERENCE SET
item by item, not just its size.

- Every excluded item must be excluded for a stated reason. One item you cannot justify
  is a defect.
- Do it per role/tenant when a permission is involved: over-restriction and
  under-restriction are found by the same diff, and only one of them is a security issue
  — the other is the quality bug nobody reports.
- For a truncation, compare `top_k` against `top_k * 5` and check how often the item
  that matters sits beyond the cut.

## The self-report trap

Treat as unproven, always: "0 errors", exit code 0, HTTP 200, "job completed", a green
dashboard, a passing smoke test, "the tests pass", and the model's own claim that it
answered well. They report EXECUTION. Quality is a separate measurement, and if nobody
computes it, the honest state is "unknown" — write exactly that.

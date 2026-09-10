# Benchmark corpus

Fifteen small, realistic projects whose planted defects a "write tests that pass" approach never
finds. Every fixture ships with a README that states its contract honestly, source that violates
that contract in a specific collision the README does not mention, and a green happy-path suite a
reasonable developer would have written. The corpus exists to measure whether a testing flow
finds what the happy path hides, and to compare one version of the flow against another on the
same ground.

## Layout

```
bench/cases/<id>/
  KEY.json          the answer key: defects, exact lines, triggers with observed output
  fixture/          the project the agent sees: README.md, src (or *.go), tests, package.json / go.mod
  fix/all/          corrected copies of only the defective files, at the same relative paths
  fix/keep-<ID>/    every other defect fixed, defect <ID> left exactly as in fixture/ (multi-defect cases)
```

Node fixtures run with `node --test` (ESM, no dependencies). Go fixtures run with `go test ./...`
(standard library only). Every suite is green with the defects present.

## Fixed versions

`fix/all/` holds the corrected version of every file named in `KEY.json`, and nothing else. For a
case with more than one defect, `fix/keep-<ID>/` fixes every defect except `<ID>`, which stays
byte-for-byte as in `fixture/`; a single-defect case has only `fix/all/`. Overlaying a variant on
a copy of `fixture/` gives a project whose happy-path suite is still green.

The agent never sees `fix/`. The runner uses it to attribute a catch from suite exit codes alone:
the agent's tests must be green on `fixture/` + `fix/all/`, and defect D counts as caught when the
same tests are red on `fixture/` + `fix/keep-D/`. A test that is green on every variant never
touched a defect, whatever its finding text says.

Known limits of the overlays, verified by running every trigger against every variant:

- `n05-keyset-pagination`: the happy suite pins the cursor to a plain `created_at` string, so the
  fix appends `#<id>` to the cursor only when another row shares that `created_at`.
- `n09-json-ids`: JavaScript numbers cannot hold the keyed id, so the fix parses integer ids as
  `BigInt`; `trigger.expected` lists the values, not their runtime type.
- `n04-slug-normalize`: the fix NFC-normalizes the title and keeps non-ASCII letters as
  separators (`café` and `café` both become `caf`), the second form the key accepts; folding
  accents would add behaviour the contract does not promise.
- `n03-subscription-window`: the fix reads the calendar day from the UTC clock, so a
  `new Date("YYYY-MM-DD")` stays on that day in every zone; a local-midnight `Date` in a zone east
  of UTC still lands on the previous day, a limit of representing a day as an instant.
- `n08-sliding-limiter`: the D2 trigger as written (limit 1, one prior hit) is also red while D1
  is present, so a test copied from it is attributed to both defects. A probe that isolates D2
  makes two prior hits on `token:ABC` before calling `token:abc`.

## Key schema

```json
{"id":"<id>","language":"node|go","suite":"node --test|go test ./...","surface":"library|cli|http|worker|etl|state-machine",
 "defects":[{"id":"D1","file":"src/x.js","line":42,"class":"<class>","keywords":["k1","k2","k3"],
   "description":"one sentence, what is wrong",
   "trigger":{"input":"<exact call or command>","expected":"<per contract>","actual":"<observed>"},
   "why_missed":"one sentence: why a happy-path suite and branch coverage never reach it"}]}
```

`line` is the line of the defective statement in `file` (relative to `fixture/`). `keywords` are
three to five lowercase words a correct finding would contain. `trigger.actual` is pasted from a
real execution against the fixture; a defect whose trigger was never run does not belong in a key.

## Scoring

The runner copies `fixture/` into a fresh workspace, runs the flow under evaluation there, and
reads the `docs/testing/test-plan.md` it produced. For every key defect:

- **found** when a finding row, together with the Evidence ledger rows it cites, names the same
  file and either a line within ±5 of the key line or any of the key's keywords;
- **missed** otherwise.

A finding row must cite at least one id that exists in the Evidence ledger; a row that names the
defect without linked evidence is recorded as `unlinked` and does not count, whatever it says. An
empty ledger makes every citation dangling.

One finding row is one claim: a row is credited to the defect it names by keyword (to several
only if it names several), and a row that matches by line alone goes to the nearest defect. Four
cases plant two defects within the line tolerance of each other, so without this a run that
noticed one of them would read as having noticed both.

A **false positive** is a finding whose location is not in the key. Recall is found over key
defects; precision is found over findings.

That is the **reported** measure. The **caught** measure asks whether the agent's tests distinguish
the defective code from the correct one, plan or no plan: the test files the agent added or changed
are carried onto `fixture/` + `fix/all/` (they must be green there) and onto `fixture/` +
`fix/keep-<ID>/` (red means some test distinguishes `<ID>`). Both measures are recorded per defect,
in the summary, the aggregate, and the history. A run that reports a defect without a test that
catches it, or catches it without reporting it, shows up as a gap between the two columns.

A test red on `fix/all` but green on `fix/keep-D` asserts the defective behaviour: it locks the
bug in and goes red the moment someone fixes it. Those are counted as **inverted** and named in
the run's notes, apart from tests that are simply red everywhere.

**Pinned** is the third column: it counts defects whose finding names a test in the plan's
`Pinning test` cell. It measures the claim, caught measures the outcome, and a run that pins more
than it catches is naming tests that distinguish nothing.

Every run keeps the plan it produced as `test-plan.md` beside its `result.json`, even when the
workspace is removed, so older runs can be re-scored when the rule changes:
`rdd-plus bench score --case bench/cases/<id> --plan <results>/<id>/<run>/test-plan.md`.

A valid run that never wrote `docs/testing/test-plan.md` scores zero and is reported as
`NO PLAN` (`no_plan` in the aggregate and the history): the flow ran and did not persist its
deliverable, which is a different failure from missing the defect. Runs where the agent did not
complete are `FAILED`, excluded from recall, and make the command exit 3.

## Rules

- The key never travels into the workspace the agent sees. Only `fixture/` is copied.
- Fixtures are never edited to make a run pass. A wrong key is fixed in the key; a weak defect is
  replaced by a real one and re-verified.
- Suites stay green with the defects present. A suite that goes red has stopped being the happy
  path and no longer measures anything.

## Adding a case

1. Pick a collision the contract is silent about (two features meeting, a boundary, a second
   call, a representation limit, a missing lock).
2. Write the README as the project would; state the guarantee, not the trap.
3. Implement realistically, without comments that point at the defect.
4. Write the suite a busy developer would write; run it; it must pass.
5. Execute every trigger; paste the observed output verbatim into the key.
6. Record the exact line of each defective statement.

## Cases

| id | language | surface | defects | classes |
|---|---|---|---|---|
| n01-csv-rfc4180 | node | library | 2 | escaping, parsing |
| n02-money-rounding | node | library | 2 | float-precision, rounding |
| n03-subscription-window | node | library | 2 | timezone, calendar |
| n04-slug-normalize | node | library | 2 | idempotence, unicode-normalization |
| n05-keyset-pagination | node | library | 1 | pagination-cursor |
| n06-retry-policy | node | http | 2 | unbounded-retry, fail-open |
| n07-tenant-scope | node | library | 2 | authorization |
| n08-sliding-limiter | node | library | 2 | off-by-one, normalization |
| n09-json-ids | node | etl | 1 | integer-precision |
| n10-order-state | node | state-machine | 2 | unreachable-state, state-machine |
| n11-html-escape | node | library | 2 | injection, idempotence |
| n12-path-normalize | node | http | 1 | path-traversal |
| g01-inventory-reserve | go | library | 2 | data-race, off-by-one |
| g02-config-merge | go | library | 2 | config-merge |
| g03-batch-writer | go | worker | 2 | data-loss, error-reporting |

Twenty-seven defects across fifteen cases.

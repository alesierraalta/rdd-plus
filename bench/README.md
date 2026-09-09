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
```

Node fixtures run with `node --test` (ESM, no dependencies). Go fixtures run with `go test ./...`
(standard library only). Every suite is green with the defects present.

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

- **found** when a finding in the plan cites the same file and either a line within ±5 of the key
  line or any of the key's keywords;
- **missed** otherwise.

A **false positive** is a finding whose location is not in the key. Recall is found over key
defects; precision is found over findings. A finding without an executed evidence record does not
count as found, whatever it says.

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

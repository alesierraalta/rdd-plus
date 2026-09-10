# Test plan — rdd-plus

Created: 2026-09-09 · Last updated: 2026-09-09 · Plan path: `docs/testing/test-plan.md` · Sandbox: `t.TempDir()` repositories and temp config dirs under `/tmp`, created and removed by the suites
Baseline: `initial working tree, no commits` · untracked files: 122 · fingerprint: `44b3914aa4c1db70c6de01824eabd20d783e3a824942069b70bdf31c1410014d` (`assets/skills/test-strategy/assets/fingerprint.sh --root .`)
Findings precision: 0 / 0

Inferred: new repository, no plan; whole-project PLAN then EXECUTE, same run. The gate package arrived already validated (its nine-finding cycle is recorded in `~/.claude/docs/testing/test-plan.md`); this plan covers what is new here: the CLI, sync, doctor, the embedded assets, and the packaging.

## Inventory

| Surface | Entry points | Owner module | Notes |
|---|---|---|---|
| CLI | `rdd-plus gate|sync|doctor|version`, `--config-dir`, `--dry-run`, `--json` | `cmd/rdd-plus/main.go` | exit 2 on no/unknown command; no test files of its own yet |
| Gate (Stop hook) | stdin JSON → decision → telemetry log → stdout | `internal/gate/` | moved from `~/.claude/hooks/testing-gate/`; `Decide(Input, Deps)` with injected git, stat, transcript, clock |
| Sync | embedded skills → `<config>/skills/<name>/`; Stop hook merged into `<config>/settings.json` | `internal/sync/` | backs up a differing existing skill dir; removes pre-repo gate entries; refuses invalid settings JSON |
| Doctor | installed skills, hook wiring, PATH capabilities, degradation notes | `internal/doctor/` | exit 1 when git, a skill, or the hook is missing |
| Assets | 12 embedded skills (103 files) | `assets/embed.go`, `internal/assets/` | `go:embed` cannot cross to a parent dir, so the embed lives beside `assets/skills/` |
| Mutation runner | 23 literal mutants over `internal/gate` | `tools/mutants/` | `go run ./tools/mutants`, exit 1 on any survivor |
| Skill-side tooling (pending port) | `evals/run_evals.py`, `evals/selftest.py`, `assets/seed-mutants.py`, `assets/fingerprint.sh` | `assets/skills/test-strategy/` | Python and shell inside the assets; the user requires validation code in Go |

## Ranked targets

Rows are never removed by budget; budget changes order and status only.

| Target | Blast radius | Churn / past fixes | Consequence class | Existing evidence | Altitude | Target rung | Sibling skill | Verdict | Status |
|---|---|---|---|---|---|---|---|---|---|
| 1. Sync settings merge: preserves every existing hook and setting, wires exactly one gate, removes pre-repo gate entries, idempotent, refuses invalid JSON | every user's `settings.json` (a wrong merge breaks all their hooks) | new | data destroyed or corrupted (a user's config) | E1 unit (7 tests), E2 on a copy of the real settings | unit + real-file | L3 (real file, real content) | `exploit-testing` | Probe | done |
| 2. Gate decision and robustness (moved package) | every session's Stop | 9 defects fixed before install | silently wrong answer | 150-run suite incl. 18 differential scenarios (E3); mutants (E5) | unit + process | L3 + real-run | `exploit-testing` | Probe | done |
| 3. Doctor: reports missing git/skills/hook with exit 1, healthy with 0, JSON shape stable | install experience | new | visible error | E1 unit (6 tests), E4 on a synced config | unit + real config | L2 | `exploit-testing` | Pin | done |
| 4. Assets integrity: every embedded skill has a `SKILL.md` whose `name` matches its directory; no run artifacts embedded | every install | new | silently wrong answer (a skill that does not load) | E1 unit (2 tests) | unit | L1 | `exploit-testing` | Pin | done |
| 5. CLI contract: usage and exit codes, flag parsing per subcommand | every invocation | new | visible error | E6 (manual: no args and unknown → exit 2) | process | L1 | `exploit-testing` | Pin | pending (no automated test in `cmd/`; add a table-driven test over exit codes and usage text) |
| 6. Port `run_evals.py`, `selftest.py`, `seed-mutants.py`, `fingerprint.sh` to Go subcommands (`evals`, `calibrate`, `fingerprint`) | eval and calibration numbers, plan baselines | — | silently wrong answer | Python versions validated earlier (see the home plan E9, E10, E1) | unit | L2 | `exploit-testing` | Probe | pending |
| 7. Install journey from a clean machine: `go install …@latest` → `sync` → `doctor` | every new user | — | visible error | none (needs the GitHub remote) | real-run | real-run | `real-run-validation` | Probe | pending (blocked on publishing) |
| 9. Benchmark dataset: 15 cases / 27 planted defects, every suite green with the defect present, every trigger reproduces the wrong output | the number every future comparison rests on | new | silently wrong answer (a defect that does not reproduce inflates recall for free) | E8 (my re-execution of 6 triggers + all 15 suites), E9 (dry-run through the runner) | process | L2 | `exploit-testing` | Probe | done |
| 10. Bench runner and scorer (`bench run|score|history`): scaffold never copies the key, suite must be green, scorer matches by file and line or keyword, history is append-only | every benchmark number | new | silently wrong answer (a lenient scorer rewards prose) | unit 20 tests incl. wrong-file and placeholder negatives; E9 | unit + process | L2 | `exploit-testing` | Probe | done (baseline run pending row) |
| 8. `evidence verify` subcommand: re-execute a ledger row's command, compare the observed digest, re-apply the declared mutation and require red | the falsifiability of every finding | — | silently wrong answer | design only | unit + process | L3 | `exploit-testing` | Probe | pending |

## Real-run recipes

| Journey | Start command | Data setup | Sample requests | Expected observable |
|---|---|---|---|---|
| Install into a fresh config | `bin/rdd-plus sync --config-dir <tmp>` then `bin/rdd-plus doctor --config-dir <tmp>` | empty temp dir, optionally a copy of a real `settings.json` | `--dry-run` first | doctor prints 12 skills ok, `Stop gate wired`, capabilities, `verdict: healthy`, exit 0; a copy of the real settings keeps its gentle-ai hooks |
| Gate inside a real session | see `~/.claude/docs/testing/test-plan.md` (E12): two `claude -p` arms | scratch repo under `/tmp` | edit without the skill; edit after reading it | telemetry lines `fired:true` and `fired:false` with `skills_loaded` |

## Layer matrix

| Layer | Skill | Scope | Status |
|---|---|---|---|
| Security (input handling) | `appsec-adversarial-auditor` | stdin and settings.json are untrusted shapes; git via argument arrays; embedded assets are read-only | folded into targets 1–2; done |
| Runtime and faults | `runtime-reliability-testing` | git timeout, entry flood, huge transcript (gate suite) | done |
| Persistence | `database-persistence-testing` | settings.json rewrite is atomic in content (refuses before writing on invalid JSON); skill backups timestamped under `.rdd-plus-backup/` | done (E1, E2) |
| Architecture conformance | `clean-architecture-audit` | `cmd/` thin, decisions in `internal/`, process boundaries injected | done by construction |
| Critical e2e journeys | `real-run-validation` | install journey (target 7), gate journey (done in the home plan) | partial |
| Sandbox | `docker-test-containers` | not needed | n/a |

## Not testing, on purpose

| Target | Reason |
|---|---|
| The skills' prose | validated by the behavioral eval suite inside `assets/skills/test-strategy/evals`, not by unit tests |
| Node hook `~/.claude/hooks/testing-gate.mjs` | kept only as the differential oracle; not shipped |
| LICENSE text | official Apache-2.0 text |

## Characterization (legacy)

| Test | Behavior pinned | Believed correct? | Promote or delete after the change |
|---|---|---|---|

## Execution log

| Date | Target | Rung reached | Findings (path:line) | Promoted tests | Evidence (ledger id) | Notes |
|---|---|---|---|---|---|---|
| 2026-09-09 | 1. sync settings merge | L3 | none | `internal/sync/sync_test.go` (7) | E1, E2 | real-settings copy: gentle-ai stop hook and 3 other hook events preserved, 16 top-level keys kept, old gate entries removed, one gate wired |
| 2026-09-09 | 2. gate (moved) | L3 + differential | none new | `internal/gate/gate_test.go`, `integration_test.go` | E3, E7 | smoke through the binary observed: fires naming `src/a.ts`, loop guard silent |
| 2026-09-09 | 3. doctor | L2 | none | `internal/doctor/doctor_test.go` (6) | E1, E4 | |
| 2026-09-09 | 4. assets | L1 | none | `internal/assets/assets_test.go` (2) | E1 | |
| 2026-09-09 | 5. CLI contract | L1 (manual) | none | none yet | E6 | automated test pending |
| 2026-09-09 | 9. benchmark dataset | L2 | none | fixture suites (72 tests across 15 cases) | E8 | six triggers re-executed by me reproduce the keyed wrong output verbatim |
| 2026-09-09 | 10. bench runner | L2 | UX: a bare `--cases "*"` matched nothing (pattern resolved against cwd); fixed to resolve bare names under `bench/cases` with a unit test | `internal/bench/*_test.go` (20) + `glob_test.go` (4) | E9 | |

## Findings

A rejected or wontfix finding is a known non-issue: it is never re-proposed unless the fingerprint
of its cited files changed; when a run skips it, it cites the row.

| Id | Finding (path:line, one line) | Severity (consequence class) | Data safe? | Evidence id | Status | Verdict by / date | Reason | Cited-files fingerprint at verdict |
|---|---|---|---|---|---|---|---|---|
| F1 | `internal/bench/score.go` (Score, before 36934b0): a finding row counted as FOUND with no linked evidence, contrary to `bench/README.md` | silently wrong answer (recall inflated by prose) | yes | E10 | fixed | alesierraalta / 2026-09-10 | evidenceLinked; empty ledger makes every citation dangling | 36934b0 |
| F2 | `assets/skills/test-strategy/evals/run_evals.py:240` (before 36934b0): a citation into an EMPTY ledger passed `findings_have_evidence` | silently wrong answer (grader passes a plan with no evidence) | yes | E11 | fixed | alesierraalta / 2026-09-10 | dangling check no longer gated on a non-empty ledger; selftest negative fixture | 36934b0 |
| F3 | `internal/bench/run.go` (before 36934b0): failed or invalid cases left exit code 0 and a partial recall read as a corpus number | silently wrong answer | yes | E12 | fixed | alesierraalta / 2026-09-10 | ExitPartial = 3; history row carries failed/invalid/no_plan | 36934b0 |
| F4 | `internal/bench/score.go` (before b85f035): a finding naming the symbol with the file only in its evidence row scored 0 (n08: both defects caught, recall 0) | silently wrong answer (recall deflated) | yes | E13 | fixed | alesierraalta / 2026-09-10 | file and line may come from linked ledger rows; keywords only from the finding row (E14 negative control) | b85f035, f016acd |
| F5 | `internal/bench/run.go` (before b85f035): successful runs deleted the workspace with the only copy of the plan, so a scorer change could not re-score them | silently wrong answer (numbers not comparable across scorer versions) | yes | E13 | fixed | alesierraalta / 2026-09-10 | every run keeps test-plan.md beside result.json; `bench score --plan` | b85f035 |
| F6 | benchmark measured reported findings, not whether the agent's tests distinguish defective from correct code (n07: plan absent, tests catch 2/2) | silently wrong answer (a plan-writing flow scores; a test-writing flow does not) | yes | E15 | fixed | alesierraalta / 2026-09-10 | Discriminate: agent tests green on fix/all, red on fix/keep-D ⇒ D caught; reported and caught are both recorded | f016acd |
| F7 | `bench/cases/g02-config-merge` (before redesign): the trigger `Config{Debug:false}` equals `Config{}`, so no implementation could satisfy the key within the API | silently wrong answer (an unscorable case) | yes | E16 | fixed | alesierraalta / 2026-09-10 | pointer fields for Debug and Retries; explicit false / 0 must win | pending commit |

Statuses: open · confirmed · fixed · rejected · wontfix.

## Evidence ledger

One row per `observado` conclusion (`references/evidence.md`). `razonado` items go under Hypotheses.

| Id | Claim | Executed | Inputs and parameters | Observed | Mutation or negative control → result | Reproduction | Label (`observado` / `razonado`, literal) |
|---|---|---|---|---|---|---|---|
| E1 | the module builds, vets, and its suites pass | `gofmt -l .`, `go vet ./...`, `go test ./... -count=1` | Go 1.26.5 | gofmt empty; vet clean; `ok internal/assets 0.004s`, `ok internal/doctor 0.155s`, `ok internal/gate 3.854s`, `ok internal/sync 0.167s` (150 runs per the writer's verbose run) | mutation pass (E5) | commands above | observado |
| E2 | sync preserves a real settings file and wires exactly one gate | `bin/rdd-plus sync --config-dir <tmp>` on a copy of `~/.claude/settings.json` | the user's real settings (16 top-level keys, 4 hook events, a gentle-ai Stop hook, an old `.mjs` gate entry and an old `hooks/bin/testing-gate` entry) | Stop hooks after sync: `gentle-ai review stop-hook --agent claude-code` and `"…/bin/rdd-plus" gate`; `gentle-ai stop-hook preserved: True`, `old gate entries removed: True`, `exactly one rdd-plus gate: True`, hook events `['PreToolUse','SessionStart','Stop','UserPromptSubmit']`, 16 keys kept | unit tests cover the negative controls: invalid JSON refused untouched, idempotent second run | recipe above | observado |
| E3 | the moved gate still agrees with the Node oracle | `go test ./internal/gate -count=1 -run Differential -v` (as part of E1) | 18 repository scenarios | differential subtests ran (not skipped) and passed | — | `go test ./internal/gate -run Differential -v` | observado |
| E4 | doctor is healthy on a synced config | `bin/rdd-plus doctor --config-dir <tmp>` after E2 | synced temp config | `Stop gate wired: "…/bin/rdd-plus" gate`; 8 capabilities present; `verdict: healthy`; exit 0; 12 skills installed | unit tests: missing git → exit 1; missing skill/hook reported | recipe above | observado |
| E5 | every decision predicate of the gate is covered by a test that goes red when it is broken | `go run ./tools/mutants` | 23 literal mutants over `internal/gate` | `23 mutants applied, 23 killed, 0 survived` | each mutant is its own negative control | `go run ./tools/mutants` | observado |
| E8 | planted defects reproduce and the happy suites stay green | `node -e` / a throwaway `_test.go` per trigger; `node --test` and `go test ./...` per fixture | n01 D2, n04 D1, n09 D1, n12 D1, n06 D2, g02 D1+D2; all 15 suites | `["a","\"b","c\"","d"]`; `["-a","a"]`; ids both `9007199254740992`, dedupe → 1; `/etc/passwd`; `{"ok":true,"data":null}`; `Debug=true Retries=5` despite an override of false/0; suites: 12 node cases pass 5–6 each, 3 Go cases `ok` | the green suites are the negative control: the defect is real yet invisible to them | commands in the transcript; KEY.json `trigger` fields | observado |
| E9 | the runner scaffolds every case without leaking the key and validates the suites | `bin/rdd-plus bench run --dry-run --cases '*' --out /tmp/rp-bench-dry` | 15 cases | summary: `cases: 15 · defects: 27`, no `INVALID`, `find … -name KEY.json | wc -l` → 0 | unit tests: KEY present in fixture → scaffold refuses; red suite → invalid case | recipe in bench/README.md | observado |
| E6 | CLI exits 2 with usage on no or unknown command | `bin/rdd-plus`; `bin/rdd-plus bogus` | — | `no args exit=2`, `unknown exit=2`, usage line `usage: rdd-plus <command> [flags]` on stderr | — | run the two commands | observado |
| E7 | the gate fires through the built binary and honors the loop guard | `bin/rdd-plus gate --config-dir <tmp>` with a scratch repo and a timestamped transcript | fresh `src/a.ts` edit (mtime +2 min), `session_id: rp-smoke`; second call with `stop_hook_active:true` | `fires: Stop | src/a.ts`; log `{"session":"rp-smoke","changed_source":1,"skills_loaded":[],"fired":true}`; loop guard: empty stdout, exit 0 | the loop-guard call is the negative control | recipe above | observado |
| E10 | FOUND requires linked evidence | `go test ./internal/bench -run 'TestFoundRequiresLinkedEvidence|TestScore' -count=1` | plan fixtures: linked / empty ledger / wrong id / no section | linked → found; the other three → not found; `ok` | before the fix the same fixtures reported found (the suite went red on the new test first) | same command | observado |
| E11 | the Python grader rejects a citation into an empty ledger | `python3 assets/skills/test-strategy/evals/selftest.py` | negative fixture "citation into an EMPTY ledger is dangling" plus one positive and one negative fixture per grader type | `every grader accepted its positive fixture and rejected its negative one`, exit 0 | with the old `if ledger_ids and ...` line the negative fixture passed (selftest red) | same command | observado |
| E12 | partial runs exit 3 | `go test ./internal/bench -run Retry -count=1` (fake agent that fails) | one failed case | aggregate failed=1, exit 3, log line `partial: 1 failed, 0 invalid` | with `code == 0` kept, the assertion on exit 3 fails | same command | observado |
| E13 | file from a linked ledger row counts; the kept plan re-scores | `go test ./internal/bench -run 'LinkedLedger|PlanExisted|KeepsThePlan' -count=1`; `bin/rdd-plus bench score --case bench/cases/n08-sliding-limiter --workspace bench/results/20260910-020623/n08-sliding-limiter/1/ws` | n08 kept workspace (real agent output) | tests `ok`; n08 `found 2/2 recall 1 plan_found true`, both `keyword` | linked row citing another file, or the citing row not the linked one → not found | same commands | observado |
| E14 | keywords in a linked ledger row do not credit another defect | `go test ./internal/bench -run KeywordsInTheLinkedLedgerRow -count=1`; `bench score` on n06 | n06 kept workspace: F1 cites `src/retry.js:18`, its E1 row says "maxAttempts" | test `ok`; n06 `reported 1/2 caught 1` | before the restriction n06 read `reported 2/2` with one defect caught (test red first) | same commands | observado |
| E15 | the discriminating check attributes a catch per defect from suite exit codes | `go test ./internal/bench -run 'Discriminate|IsTestFile|Corpus' -count=1`; `bench score --workspace` on n07, n08, n06, g02 kept workspaces | shell-suite synthetic case (2 defects, all/keep-D1/keep-D2); real workspaces | synthetic: D1-only test → D1 caught, D2 not; broken test → all_green false, nothing caught; source-only change → nothing; n07 `reported 0/2 caught 2/2`; n08 `2/2 caught 2/2`; n06 `1/2 caught 1`; g02 (old API) `all_green false`, build error recorded | a test passing everywhere catches nothing; multi-defect case without keep-D → not checkable, note recorded | same commands | observado |
| E16 | every corpus case ships fix/all and keep-<D>, and the happy suite is green on the fixture and every variant | `go test ./internal/bench -run Corpus -count=1` | 15 cases, 27 defects, 40 variants | `ok` (8.1s) | a variant that broke the suite would fail the test with the suite tail | same command | observado |

### Hypotheses (razonado)

| Id | Hypothesis | Probe that would settle it |
|---|---|---|
| H1 | `go install …@latest` works from a machine with only Go and git | target 7 after publishing |

## Calibration history

| Date | Skill version | K | Found | Recall | Misses (file:line operator, why) | False positives |
|---|---|---|---|---|---|---|

## Blocked by testability

| Target | Rung | Why | Minimal change that opens it |
|---|---|---|---|
| 7. install journey | real-run | no remote yet | publish the repository, then run the recipe from a clean `GOPATH` |

## Remaining, in order

1. Baseline benchmark run with the current skill and the final runner (reported and caught per defect); then install the upgraded skill content and run again
2. Target 5: table-driven CLI test (exit codes, usage, flag errors per subcommand)
2. Target 6: Go port of the eval harness, self-test, calibration seeding, and fingerprint
3. Target 8: `evidence verify`
4. Target 7: install journey after publishing

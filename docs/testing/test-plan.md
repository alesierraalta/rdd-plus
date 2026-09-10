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
| 5. CLI contract: usage and exit codes, flag parsing per subcommand | every invocation | new | visible error | E6 (manual: no args and unknown → exit 2) | process | L1 | `exploit-testing` | Pin | done |
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
| F12 | `bench/cases/g01-inventory-reserve/KEY.json`: the case declares suite `go test ./...`, which cannot observe a data race, so its race defect can never flip a test outcome and the catch measure marks it missed however good the agent's concurrency test is | silently wrong answer (the benchmark under-reports catching on one of 27 defects) | yes | E22 | open | — | fix after the in-flight run: suite becomes `go test -race ./...`; changing the corpus mid-run would contaminate it | pending |
| F11 | with the plan persisted, the flow now reports more than it pins: reported 24 of 27 against 19 caught by a test, and two cases report a defect with no discriminating test at all (n12 wrote none, n09 wrote one green on both variants) | silently wrong answer (a finding nobody can re-run is the failure mode the whole skill exists to remove) | yes | E21 | open | — | candidate rule for 3.2: a confirmed finding leaves a test in the suite that fails without the fix; a scratchpad probe is not enough | pending |
| F10 | `internal/doctor/doctor.go` (before 2026-09-10): the gate check matched only `<binary> gate`, so a working Stop hook installed under its own name (`~/.claude/hooks/bin/testing-gate`) was reported "NOT wired" with exit 1 | visible error (a false alarm on a healthy setup trains people to ignore the doctor) | yes | E20 | fixed | alesierraalta / 2026-09-10 | HookKind distinguishes rdd-plus, standalone and none; only none is a problem | pending commit |
| F8 | the flow reports findings it never persists: 6 of 15 baseline runs answered in chat and left no `docs/testing/test-plan.md`, so their verdicts cannot be honored or re-scored | silently wrong answer (the deliverable is missing while the answer looks complete) | yes | E17 | fixed | alesierraalta / 2026-09-10 | test-strategy 3.1 rule 12: the plan is on disk before the final message | pending after-run |
| F9 | the catch check discarded a whole test file when any test in it was red on the fixed code (g01 and g02 reported 2/2 and were scored 0 caught for writing no test; n01, n06, n12 lost real catches) | silently wrong answer (caught deflated) | yes | E18 | fixed | alesierraalta / 2026-09-10 | per-test outcomes; a test red on the fixed code is ignored, the rest still count | 7fbd565 |
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
| E22 | the race defect is invisible to the declared suite and visible with -race | fixture + `fix/keep-D1` + the agent's `inventory_test.go` from `bench/results/20260910-102659`, run five times with `go test -run TestConcurrentReservesNeverOversell -count=1 ./...`, then once with `-race`; then the pristine fixture and `fix/all` with `go test -race ./...` | g01, 100 goroutines reserving 1 unit of 100 | five passes without `-race`; `FAIL` with `-race`; the happy suite stays `ok` under `-race` on both fixture and fix/all | the same test under `-race` goes red on the defect and green on the fix, which is the outcome flip the measure needs | same commands | observado |
| E21 | rule 12 removed the persistence failure and did not move the catch rate | `bin/rdd-plus bench run --cases '*' --model sonnet` with 3.1 installed, then `bin/rdd-plus bench compare bench/results/20260910-023251/rescored bench/results/20260910-095629` | the same 15 cases, 27 defects, same fixed variants, same scorer | no plan 6 → 0; reported 15 → 24 (0.56 → 0.89); caught 20 → 19 (0.74 → 0.70); false positives 1 → 0; $8.37 → $8.04; turns 319 → 311 | the plan-writing rule was the only change between the two runs; n05 and n08 baseline catches are carried over from the original run because their workspaces were deleted, so they were not re-checked under the current rules | `bin/rdd-plus bench compare …` | observado |
| E20 | doctor tells a gate under another name from no gate at all | `go test ./internal/doctor -count=1`; `rdd-plus doctor` against the live `~/.claude` | six hook commands (rdd-plus subcommand, with flags, standalone binary, node script, an unrelated hook, a command mentioning the word) | tests `ok`; live report `Stop gate wired to a standalone gate binary: "…/hooks/bin/testing-gate"`, `verdict: healthy`, exit 0 (was `NOT wired`, exit 1) | three mutants (any hook counts, standalone reported as none, always wired) all killed | same commands | observado |
| E19 | every CLI exit code and usage line is pinned by a test that goes red when it breaks | `go test ./cmd/... -count=1`; six manual mutants over `cmd/rdd-plus/main.go` | 14 process-level cases (no args, unknown command, bench subcommands, flag errors, gate on empty/malformed stdin) | `ok`; every mutant killed | usage exit 2→0, gate bad-flag 0→2, score/rescore/compare argument checks removed, usage line dropped: all six killed | `go test ./cmd/... -count=1` | observado |
| E17 | the baseline flow leaves no plan in 40% of runs | `bin/rdd-plus bench run --cases '*' --model sonnet` with test-strategy 3.0 installed | 15 cases, 27 planted defects | reported 15/27 (0.56), caught 20/27 (0.74), no plan 6, false positives 1, $8.37 | the after-run with 3.1 rule 12 is the control | `bin/rdd-plus bench rescore bench/results/20260910-023251` | observado |
| E18 | per-test attribution recovers catches a file-level verdict discarded | `bin/rdd-plus bench rescore bench/results/20260910-023251` before and after the per-test change | the same 15 kept workspaces | caught 11 → 20 of 27; n01, n02, n03, n12 went from 0 to their real count with the broken test ignored | a synthetic node case with one always-red test still attributes D1 and not D2 (`TestDiscriminatePerTestSurvivesABrokenTest`) | `go test ./internal/bench -run SurvivesABrokenTest -count=1` | observado |
| E16 | every corpus case ships fix/all and keep-<D>, and the happy suite is green on the fixture and every variant | `go test ./internal/bench -run Corpus -count=1` | 15 cases, 27 defects, 40 variants | `ok` (8.1s) | a variant that broke the suite would fail the test with the suite tail | same command | observado |

### Hypotheses (razonado)

| Id | Hypothesis | Probe that would settle it |
|---|---|---|
| H1 | `go install …@latest` works from a machine with only Go and git | target 7 after publishing |

## Calibration history

| Date | Skill version | K | Found | Recall | Misses (file:line operator, why) | False positives |
|---|---|---|---|---|---|---|
| 2026-09-10 | 3.1 (installed) | 27 planted defects, 15 cases, same corpus and rules as the row below | reported 24, caught by a test 19 | reported 0.89 · caught 0.70 | every run wrote its plan (0 of 15 missing); n12 reported without writing any test; n09 wrote a test that is green on both variants; n06 and n08 reported one of two | 0 |
| 2026-09-10 | 3.0 (installed) | 27 planted defects, 15 cases | reported 15, caught by a test 20 | reported 0.56 · caught 0.74 | 6 of 15 runs wrote no plan at all (g03, n02, n07, n10, n11, n12); g01 and g02 reported everything and wrote no test; n04, n06, n11 caught one of two | 1 (n09) |

## Blocked by testability

| Target | Rung | Why | Minimal change that opens it |
|---|---|---|---|
| 7. install journey | real-run | no remote yet | publish the repository, then run the recipe from a clean `GOPATH` |

## Remaining, in order

1. Baseline benchmark run with the current skill and the final runner (reported and caught per defect); then install the upgraded skill content and run again
2. Target 6: Go port of the eval harness, self-test, calibration seeding, and fingerprint
3. Target 8: `evidence verify`
4. Target 7: install journey after publishing

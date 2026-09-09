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

## Findings

A rejected or wontfix finding is a known non-issue: it is never re-proposed unless the fingerprint
of its cited files changed; when a run skips it, it cites the row.

| Id | Finding (path:line, one line) | Severity (consequence class) | Data safe? | Evidence id | Status | Verdict by / date | Reason | Cited-files fingerprint at verdict |
|---|---|---|---|---|---|---|---|---|

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
| E6 | CLI exits 2 with usage on no or unknown command | `bin/rdd-plus`; `bin/rdd-plus bogus` | — | `no args exit=2`, `unknown exit=2`, usage line `usage: rdd-plus <command> [flags]` on stderr | — | run the two commands | observado |
| E7 | the gate fires through the built binary and honors the loop guard | `bin/rdd-plus gate --config-dir <tmp>` with a scratch repo and a timestamped transcript | fresh `src/a.ts` edit (mtime +2 min), `session_id: rp-smoke`; second call with `stop_hook_active:true` | `fires: Stop | src/a.ts`; log `{"session":"rp-smoke","changed_source":1,"skills_loaded":[],"fired":true}`; loop guard: empty stdout, exit 0 | the loop-guard call is the negative control | recipe above | observado |

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

1. Target 5: table-driven CLI test (exit codes, usage, flag errors per subcommand)
2. Target 6: Go port of the eval harness, self-test, calibration seeding, and fingerprint
3. Target 8: `evidence verify`
4. Target 7: install journey after publishing

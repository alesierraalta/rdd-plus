# rdd-plus

A deterministic companion for the testing discipline in Claude Code. It complements gentle-ai; it
does not replace it. gentle-ai owns the review lifecycle (frozen candidate, refuter, receipts).
rdd-plus owns what happens before that: a testing flow the model runs and a set of Go binaries
that keep it honest.

The split is deliberate. The model does the creative work: enumerating the classes of a contract,
imagining the inputs nobody expects, writing the probe. The binaries do the deterministic work:
installing the skills, wiring the hook that keeps them invoked, and reporting what the
environment can do so a skill degrades explicitly instead of failing on a tool it assumed.

## Install with an agent

This block is a prompt, not a description: hand it to an agent that has a shell and let it drive the
install end to end. An install has five steps a model can each skip in silence — the binary that
never reaches `PATH`, the hook wired to a binary you will delete tomorrow, the rehearsal that never
ran, the warning read as a verdict, the host declared working because its configuration file was
written. This project's whole subject is that last failure, so the prompt asks for the exit code and
the output line behind every claim, and rehearses into a throwaway configuration directory before it
touches yours.

````text
Install rdd-plus on this machine and leave it working. Answer in the language the operator wrote to
you in, and prove every step with the command you ran, its exit code, and the output line that
carries the claim.

Rules:
- Never write "installed", "wired" or "working" without the exit code and the output that shows it.
- Never edit ~/.claude/settings.json by hand. `rdd-plus sync` owns that merge: it preserves every
  existing setting and hook, adds the gate once, and refuses a file it cannot parse.
- Decide where the binary will live before the first `sync`. The hook records the absolute path of
  the binary that ran sync, so run sync from the binary you intend to keep, after it is on PATH.
- When a step fails, stop and report the failure verbatim instead of improvising around it.

1. Prerequisites. `git --version` and `go version`; go.mod declares `go 1.26`, and the module has no
   third-party dependencies, so Go and git are the whole requirement. docker, node, python3, claude,
   codegraph, rtk, gentle-ai and engram are optional. `rdd-plus doctor` names what each one's absence
   degrades, and that report is the authority rather than a list here.

2. Install the binary. Pick one:
   a. Published: `go install github.com/alesierraalta/rdd-plus/cmd/rdd-plus@latest`, which lands in
      `$(go env GOPATH)/bin`. Ensure that directory is on PATH, then confirm with
      `command -v rdd-plus`.
   b. From a source checkout, which is what running the tests and the mutants requires:
      `git clone https://github.com/alesierraalta/rdd-plus.git && cd rdd-plus && make build` writes
      `bin/rdd-plus`. `make test` runs the suite, `make vet` runs gofmt and go vet, and
      `go run ./tools/mutants` applies 23 literal mutations to a copy of the tree and requires every
      one killed. That last one copies the whole working tree file by file, so a checkout carrying
      local tool state — a `.codegraph/daemon.sock`, for instance — fails before it applies
      anything: report that, and do not delete the directory to get past it.

3. Prove the binary runs: `rdd-plus version` prints the version and the revision it was built from —
   `unknown` for a `go install` build, a commit hash for a build from a checkout, with `+dirty` when
   that checkout has uncommitted changes.

4. Rehearse the install without touching the real configuration. `--config-dir` targets one Claude
   directory and nothing else, which is what makes this safe:

   ```sh
   tmp=$(mktemp -d)
   rdd-plus sync --config-dir "$tmp/cfg" --dry-run     # the plan, writing nothing
   rdd-plus sync --config-dir "$tmp/cfg"               # $tmp/cfg/skills plus settings.json
   rdd-plus doctor --config-dir "$tmp/cfg"             # must exit 0
   ```

   The report carries one line per embedded skill, the command the wired hook runs, whether that
   command answered, and the capabilities; the line that decides is `verdict: healthy` with exit 0.
   `doctor --json` prints the same report as a machine reads it.

5. Install for real: `rdd-plus sync` with no `--config-dir` (default `~/.claude`; `--hosts` narrows
   the run to named hosts), then `rdd-plus doctor`, which must exit 0. A skill you edited locally is
   moved to `<config root>/backups/<timestamp>/` before it is replaced, so the run
   is reversible. Read doctor's warning about the binaries, not only its verdict: doctor compares the
   binary the hook invokes with the `rdd-plus` on PATH, and when they are different files it says the
   two would give different verdicts. If it warns, run sync again from the PATH binary and re-check.

6. Two hosts cannot be wired from here. Pi takes `assets/hosts/pi/settings.stop-hook.json` merged
   into its settings (Pi reads the same `~/.claude/skills`, so it already has the skills), and OpenCode
   takes `assets/hosts/opencode/rdd-plus.ts` as a plugin. Both shapes were read from their
   documentation and nobody has watched them fire here: install them if asked, and say plainly that
   they are unverified instead of reporting them as working.

7. Prove the hook answers without waiting for a real session. In a throwaway git repository, commit a
   file, write a transcript whose first timestamp precedes the change, touch a production source file,
   then hand the gate the payload its host would:

   ```sh
   printf '%s' "{\"session_id\":\"install-check\",\"transcript_path\":\"$tmp/transcript.jsonl\",\"cwd\":\"$tmp/repo\",\"hook_event_name\":\"Stop\",\"stop_hook_active\":false}" \
     | TESTING_GATE_LOG="$tmp/gate.jsonl" rdd-plus gate
   ```

   Expected: exit 0, a Stop payload naming the changed file, and one line in `$tmp/gate.jsonl` reading
   `"fired":true`. A payload the gate cannot read (`printf '{ broken' | rdd-plus gate`) also exits 0
   and leaves one line reading `"skipped":"unreadable_payload"`. Exit 0 is the contract: the gate
   grades the turn, it never breaks it, so a non-zero exit is a defect and not a refusal.

8. To leave a project under the discipline and not only the machine: `rdd-plus plan init` writes
   `docs/testing/test-plan.md`, and `rdd-plus check` exits 1 naming what the repository still owes,
   layer by layer, from git and the plan alone — no hook payload, no transcript, no host, which is
   also what makes it usable from CI.

Report a table of command → exit code → the line that proves it, then name what you could not
verify and why. If doctor does not exit 0 with `verdict: healthy`, the install is not finished: say
which problem line it printed.
````

A correct run ends with `rdd-plus doctor` exiting 0, `verdict: healthy`, one line per embedded skill,
and the Stop hook wired to the binary you kept. The section below is the same install, for a person.

## Install

```sh
go install github.com/alesierraalta/rdd-plus/cmd/rdd-plus@latest
rdd-plus sync      # installs skills into every discovered host; wires the Stop hook only for Claude
rdd-plus doctor    # verifies the install and lists optional capabilities
```

`sync` installs the embedded skills into every host it finds: `~/.claude/skills`,
`~/.config/opencode/skills`, `~/.gemini/skills`, and `~/.codex/skills`. The Stop hook is wired only
where its transport is known—Claude Code's `~/.claude/settings.json`; OpenCode, Gemini, and Codex
receive skills but their transports are documented rather than wired. Claude's sync merges into
`~/.claude/settings.json`: every existing hook and setting is preserved, the gate is added once,
and running it again changes nothing. A file modified after rdd-plus installed it is not replaced
by default; pass `--force` to replace it after rdd-plus snapshots the edited file in the central
backup store described below. An unparseable `settings.json` aborts the run before anything is
written. Use `--config-dir` to target another Claude directory, `--hosts` to narrow installation,
and `--dry-run` to see the plan. Without an explicit `--config-dir`, commands resolve the directory
in this order: `CLAUDE_CONFIG_DIR`, then `PI_CODING_AGENT_DIR`, then `~/.claude`; empty values are
ignored, and a ledger that lives under a different directory is reached with `--config-dir`.

### State and safety

Keep the local state at `<config root>/state.json`; the default is `~/.config/rdd-plus/state.json`,
and `RDD_PLUS_HOME` overrides it. It records installed hosts, components, and files. Keep it local-only.
Classify paths as managed (installed by rdd-plus), modified-by-the-user (never overwrite without
`--force`; always snapshot first), or foreign (never write or delete). Store backups at
`<config root>/backups/<timestamp>/{manifest.json, files/…}`.

Opt into feedback with `rdd-plus feature enable feedback`. Use `--preview` to see one anonymized
record and the two local-only files never publishable. While feedback is off, `feedback --file`
refuses and the Stop hook stops offering it. The interactive TUI now exists as `rdd-plus tui`,
and the lifecycle commands `update`, `uninstall`, `restore`, and `repair` are documented in the
Commands table below.

## Commands

| Command | What it does |
|---|---|
| `rdd-plus gate` | The Stop hook. Reads the hook payload on stdin, decides, logs one line, and emits Stop feedback when a session changed production source without loading the adversarial testing discipline. Always exits 0. |
| `rdd-plus sync [--dry-run] [--force]` | Installs the embedded skills into discovered hosts and wires Claude's Stop hook. `--dry-run` prints the plan and writes nothing; `--force` replaces modified managed files after snapshotting them. Idempotent. |
| `rdd-plus doctor` | Reports installed skills (and whether they drift from the embedded version), whether the hook is wired, and which optional tools are on PATH with what degrades without each. `--json` for machines. Exit 1 when git, a skill, or the hook is missing. |
| `rdd-plus status [--json]` | Reports local installation state, features, and available version — the cached result after `update` has checked, or `unknown (no update check yet)` before the first check. |
| `rdd-plus feature list\|enable\|disable <id> [--preview]` | Lists or toggles optional features; preview without changing state. |
| `rdd-plus tui` | Interactive menu over the status report, feature toggles (list, enable/disable, preview), and the sync dry-run plan. Needs an interactive terminal on Linux or macOS; elsewhere it refuses and points at `status`, `feature`, and `sync --dry-run`. |
| `rdd-plus update [--check]` | Checks the Go module proxy for a newer release and installs it with `go install github.com/alesierraalta/rdd-plus/cmd/rdd-plus@<tag>` (prints the command when `go` is absent); `--check` only refreshes the offline cache. |
| `rdd-plus uninstall [--dry-run] [--orphans] [--force] [--config-dir <dir>]` | Removes managed assets and unwires the Stop hook; never touches foreign files. Modified content needs `--force` (snapshot first); `--orphans` also removes assets the manifest no longer ships; `--dry-run` writes nothing. |
| `rdd-plus restore [--id <backup-id>] [--dry-run]` | Copies a backup store entry back onto its original paths (default: the latest backup). |
| `rdd-plus repair [--config-dir <dir>] [--dry-run] [--force]` | Brings a broken install back to what `doctor` reports healthy (missing/drifted managed skills and Stop hook re-wire); a healthy install is a no-op. |
| `rdd-plus plan` | Writes the plan skeleton, checks the contract, names the breadth still owed, and records one Findings row from flags. `add-finding` writes that row only: it refuses a row the checker would reject and never writes an evidence row. |
| `rdd-plus plan admit` | Reads the plan's Evidence ledger and decides every row. A dry run by default: `--execute` runs each admitted row's one command through `sh -c` twice, so a pin is only recorded over an output that held still, `--sandbox` observes it in a container with the tree mounted read-only and no network (it needs docker, and the default image is pulled on first use) and replays a declared `Mutate` edit against a writable copy of the tree, where the command must go red under the edit and green once the file is put back, a row whose `Expect` cell is `fail` pins a test observed red (only an exit from 1 to 125 qualifies, so a missing command (127) is never pinned as red; a zero exit is refused as `expected-failure-passed`), `--only <ids>` narrows the run, `--timeout` bounds one command, and `--record <ids>` writes the observed digest into the plan together with the mode it was observed in. Exit 1 when a row is refused. |
| `rdd-plus feedback` | Records one honest process report about the testing method itself, or reads the reports back. `--template` prints a fillable skeleton, `--file <path>` records it, `--summary` (the default) answers whether the method is earning its keep. |
| `rdd-plus version` | Prints the version. |

## How the gate decides

At the end of every turn the gate fires only when all of these hold:

- the working directory is inside a git repository with at most 20 000 status entries (a home
  directory has hundreds of thousands; a project never does);
- at least one production source file changed after the session started, where the session start
  is the first `timestamp` in the session transcript and production source means a source
  extension outside test, fixture, vendored, build, and Claude configuration trees;
- neither `test-strategy` nor `exploit-testing` was actually invoked in the session (a name in the
  available-skills listing does not count; a Skill call or a read of its `SKILL.md` does);
- the repository has no `.no-testing-gate` file at its root;
- the turn is not already continuing because of a previous Stop hook.

When it fires, the feedback names the files and asks for `test-strategy` ("haz el testing").
It is a reminder, not an approval gate.

Telemetry: every decision appends one JSON line to `~/.claude/telemetry/testing-gate.jsonl`
(override with `TESTING_GATE_LOG`), so the invocation rate is measurable over time. Its `repo`, `session`
and `plan` fields are pseudonyms, not names.

## Skills

The embedded skills live under `assets/skills/`. `test-strategy` is the organic entry point
("haz el testing"): it infers the mode from repository state, persists a plan that never
shrinks, sweeps the specialized skills for their own checks, and executes through
`exploit-testing`, whose ladder climbs from contract classes and hostile inputs to real
collaborators, real seams, injected faults, and concurrency. Every finding carries an executed
evidence record; anything not executed is a hypothesis.

`breakcheck` is an explicitly invoked bounded adversarial campaign for one candidate between the Verifier and RDD; it reports evidence and a readiness disposition, not a score.

## Development

```sh
go test ./...           # unit, integration (real git repositories), and differential tests
go run ./tools/mutants  # 23 literal mutants on the gate; every one must be killed
make build              # bin/rdd-plus
```

Integration tests build the CLI once and drive it with real repositories in temporary
directories; they are skipped under `-short`. The differential test compares the Go gate with the
original Node hook when `node` and `~/.claude/hooks/testing-gate.mjs` are present.

CI runs that suite on every pull request and on every push to `master`, with `node` installed so the
benchmark's JavaScript cases run instead of skipping. It checks `gofmt`, `go vet`, the build and
`go test ./... -count=1` — the same commands `make vet`, `make build` and `make test` run locally.

## Pending

- The eval harness (`assets/skills/test-strategy/evals/run_evals.py`, `selftest.py`) and the
  calibration tools (`assets/skills/test-strategy/assets/seed-mutants.py`, `fingerprint.sh`) are
  still Python and shell. They will become subcommands.
- Replaying a ledger row's mutation before a finding is accepted, and `status --next-transition`,
  are the next binaries.

## Plan

The structure of `docs/testing/test-plan.md` is a contract, and deriving it from prose every
session is where compliance goes wrong: in one 15-case benchmark, three runs wrote their findings
as prose sections instead of the template's tables, and every one of those had never opened the
template.

For both `check` and `plan`, a relative `--path` is resolved against the worktree root. An absolute
`--path` is taken as given, except in `check`, which refuses it as a usage error. This is the same
relative resolution and containment rule used for the `planPath` declaration in `.rdd-plus.json`.

The effective plan path follows one precedence rule: an explicit `--path` wins, then `planPath` in
`.rdd-plus.json` at the worktree root, then `docs/testing/test-plan.md`. Declare a scoped plan like
this:

```json
{"planPath": "docs/testing/test-plan-redis-stream-pool.md"}
```

A malformed, unreadable or unusable declaration fails closed; it never silently falls back to the
default. `rdd-plus check`, the Stop-hook gate and every `plan *` command honor the same declaration.

```
rdd-plus plan init                # write the skeleton, tables and all; never overwrites silently
rdd-plus plan check               # exit 1 and name every breach of the contract
rdd-plus plan add-finding ...     # write one Findings row; refuses a row the checker would reject
```

`check` reports a Findings section that is not a table, a finding that cites no `path:line`, a
finding citing an evidence id that is not in the ledger, a settled finding that names no pinning
test, a `razonado` row sitting in the evidence ledger, and an `n/a` layer row without a reason in
its `Scope` cell when the plan declares `Light:`. It says nothing about whether the testing was
good; it says the plan can be located, honoured, and re-scored.

A plan may declare itself scoped. A bounded change — one target inside one or two files, touching
none of the classes the skill's boundary list forbids — plans its blast radius instead of the whole
app, and says so in one header line: `Light: <blast radius> · touches <classes>`. Every layer the
run did not touch keeps its row with status `n/a` and states why in the `Scope` cell, and the "Not
testing, on purpose" table names what a full run would have added. Execution is unchanged: the
target climbs to its target rung through its sibling, every confirmed finding leaves a pinning test
and an evidence row, and the run closes on `plan check`. The declaration is the half a binary can
check; whether the change was really bounded stays with the run and the plan's reader.

## Hosts

The decisions live in Go; only the transport is per host. That split is what lets the same rules
serve more than one agent, and it is why `check` exists:

```
rdd-plus check          # exit 1 and say what this repository owes, from git and the plan alone
```

It reads no hook payload, no transcript and no host configuration, so anything that can run a
command can use it: another agent, a Makefile, a pre-push script, CI. What a host integration adds
on top is knowing what THIS session did, which the repository cannot tell you.

| Host | Status |
|---|---|
| Claude Code | verified: `sync` wires the Stop hook, `gate` reads its payload and answers in its schema, `doctor` runs the wired command and requires exit zero |
| Anything that runs a command | verified: `check`, `plan init`, `plan check`, `plan gaps` need no host at all |
| Gemini CLI | not implemented: its `settings.json` takes command hooks under different event names, and its payload and output schemas are not verified here |
| Codex, OpenCode, Pi | not implemented: each has its own extension surface, and guessing a payload schema would ship a hook that silently never fires |

Nothing above is a promise about a host that is not listed as verified. A hook that looks wired and
never answers is the failure this project keeps finding, so a host counts as supported when its
command has been run and its exit code checked, not when its configuration file has been written.

## The two questions at the Stop

The gate asks one question when a session changed production source and never loaded the
discipline: run it. It asks a different one when the discipline ran and stopped halfway.

Covering a diff and reporting as though the surface were covered is the failure that survives
every green check: the depth work succeeds, the breadth work is never started, and the summary
reads as complete. The plan makes it detectable, because a layer the plan assigned carries an
owner and a status: `Security | appsec-adversarial-auditor | ... | pending` after the run means
assigned and never invoked.

```
rdd-plus plan gaps                # exit 1 and name every layer still owed, with its owner
```

`plan gaps` reads the layer matrix's status cells. A row counts as swept when its status reads
`done`, `fixed` or `closed`, and `n/a`, `na`, `none` and `skipped` leave the denominator entirely:
marking a layer out of scope is a decision the plan records, not a silent omission. The `Skill`
cell only labels the rows still owed, so a row with no owner still has to be swept.

Nothing verifies that an assigned sibling was really invoked — the cell is what the check has, and
that is deliberate, because the check must run with no host and no transcript. So the routing
ledger in the reply says how each sibling ran: through a Skill tool, or `inline: <path to its
SKILL.md>` where the harness has none. Without that line an inline run and a skipped one look
identical in the plan.

At the Stop the same check runs by itself. The model is told which surfaces went unexamined and
asked to say so plainly in its final message rather than let depth read as coverage, and the
operator gets one line in their own terminal offering feedback on the run, so the offer does not
depend on the model remembering to make it. It is a reminder, not an approval gate, and a
`.no-testing-gate` file silences it.

## Run feedback

The most valuable artifact a run can hand back is an honest report on the method itself: what
paid off, what was ceremony, where a rule had to be reverse-engineered, and whether it earned its
keep. The gate offers it at every Stop; `feedback` is where the answer lands. At the end of a beta run—
including blocked or partial runs—the executing agent records its own retrospective. The reply carries
only a brief acknowledgment after a successful write; the summary groups by project and names any
verdict it could not classify.

```sh
rdd-plus feedback --template          # a fillable skeleton with the run's identity already filled
rdd-plus feedback --file report.md    # record one report; refusals exit 2 and write nothing
rdd-plus feedback --summary           # counts per verdict and per skill version, plus the guesses
rdd-plus feedback                     # no flags: the summary, the cheapest path to the answer
```

One report is `paid`, `cost`, `reason`, and a `verdict` of `paid`, `partly`, or `ceremony`; `guess`
and `freeform` are optional. The `skill` field names what actually ran as `<name>` or `<name> <version>`; for example, a breakcheck run records `breakcheck <its version>`. Each report appends one JSON line to
`<config-dir>/telemetry/run-feedback.jsonl` and one section to `run-feedback.md`, beside the gate's
own log. The summary counts reports, verdicts, and skill versions and prints the `guess` lines of
the most recent reports; it clusters nothing and invents no score.

### Sanitization

Before writing, identity values become stable per-installation pseudonyms so runs of the same target
still compare. Free text loses the project-specific instance while keeping what the method did. A
secret in `paid`, `cost`, or `reason` refuses the whole report and writes nothing; in `guess` or
`freeform`, it is redacted. New rows carry `"sanitized":true`. The local-only files
`<config-dir>/telemetry/.salt` and `<config-dir>/telemetry/.pseudonyms.jsonl` must never be published:
the first reverses every pseudonym and the second maps them back to names. Rows written before this
stage existed keep their old shape and are counted in the summary.

The same project is not correlated across the two ledgers: the feedback ledger seals the absolute
worktree root, while the gate ledger seals the repository directory name.

## Benchmark

`rdd-plus bench` measures whether the testing skill finds defects it was never told about. Each
case under `bench/cases/<id>/` is a fixture project whose own suite is green plus a sealed
`KEY.json` listing the planted defects (file, line, class, keywords, trigger). The key is never
copied into a workspace.

```
rdd-plus bench run --cases 'bench/cases/*' --model sonnet --runs 1 --max-cost-usd 20
rdd-plus bench score --case bench/cases/<id> --workspace <ws>     # re-score after grader changes
rdd-plus bench history                                            # every run, never rewritten
rdd-plus bench rescore bench/results/<run>                        # re-read a finished run with today's rules
rdd-plus bench compare bench/results/<before> bench/results/<after>   # per-case reported and caught, side by side
```

`bench run` scaffolds `<out>/<case>/<run>/ws` (default `bench/results/<timestamp>/`), commits the
fixture, checks its suite is green (otherwise the case is invalid and skipped), spawns
`claude -p "haz el testing"` in it, then scores `docs/testing/test-plan.md`:

Two measures per planted defect, because saying it and catching it are different claims.

**Reported** reads `docs/testing/test-plan.md`: a defect is found when a Findings row cites its
file (suffix match, so `render.js:9` and `fixture/src/render.js` both name `src/render.js`) and
either a line within ±5 of the planted line or one of its keywords. The row must cite an id that
exists in the Evidence ledger; an unlinked row is prose and does not count, and an empty ledger
makes every citation dangling. The file and line may come from the linked ledger rows, the
keyword only from the finding itself. A row matching no planted defect is a **false positive**.

**Caught** ignores the plan and asks whether the tests distinguish broken code from correct code.
Each case ships `fix/all/` (every defect fixed) and, when it has more than one, `fix/keep-<ID>/`
(every defect but that one fixed). The test files the agent added or changed are carried onto each
variant and run per test: a test green on `fix/all` and red on `fix/keep-D` catches D. A test red
on `fix/all` is broken or written to another API, so it is ignored and counted separately; a test
green everywhere catches nothing.

**Pinned** reads the plan's `Pinning test` cell: it counts defects whose finding names a test that
holds them. Naming a test and having one that distinguishes the defect are different claims, so
pinned sits next to caught, never instead of it. Plans written before rule 13 have no such column
and simply claim nothing.

The gap between the columns is the interesting number. A run can report a defect it never
pinned with a test, or catch one it never wrote down.

Per run: `result.json` and the `test-plan.md` it produced, kept even when the workspace is
removed. Per bench: `aggregate.json`, `summary.md`, one line in `bench/history.jsonl` and one row
in `bench/history.md` with the installed `test-strategy` version, so skill versions compare on
identical fixtures. `--dry-run` scaffolds and checks fixtures without spawning or recording;
`--max-cost-usd` stops early with exit code 2; a run with any failed or invalid case exits 3, so a
partial number is never read as a corpus result.

The Pi runner (`--runner pi`, the default) measures the same cases with Pi itself:
`pi -p "haz el testing" --mode json --model <provider>/<model>[:<thinking>] --no-session`, spawned in
the workspace with `PI_CODING_AGENT_DIR` pointed at a throwaway agent directory. That directory holds
the same embedded skills and a `settings.json` that loads no packages, so the operator's extensions,
memory protocol, and MCP servers stay out of the reading. `--model` is optional: Pi defaults to
`opencode/muse-spark-1.3-contributor-free`, while the `claude` runner takes a bare name and defaults
to `haiku`. Pi has no turn cap, so `--timeout` is its only wall-clock limit. `--runner` defaults to
`pi`; `claude` stays as the last-resort alternative for when Pi's providers are unavailable, and every
reading already in the history still names the runner and the model it used.

The Pi agent directory copies `auth.json` and `models-store.json`; it never links them. The Claude
throwaway config linked the operator's credentials, and a refresh that failed wrote the empty token
state straight through the link, logging the operator out of Claude Code everywhere (F25 in
`docs/testing/test-plan.md`). A copy confines that failure to a file the run throws away, and a test
holds the line: the copy is a regular file, not a link, and the source still holds its token
afterwards. When the operator has no `auth.json`, the run is not stopped: it says so and lets the CLI
report the authentication error itself, as a failed case.

## License

Apache-2.0. See `LICENSE` and `NOTICE` for the attribution of the skills derived from the
Gentleman-Programming skills.

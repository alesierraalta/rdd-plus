# rdd-plus

A deterministic companion for the testing discipline in Claude Code. It complements gentle-ai; it
does not replace it. gentle-ai owns the review lifecycle (frozen candidate, refuter, receipts).
rdd-plus owns what happens before that: a testing flow the model runs and a set of Go binaries
that keep it honest.

The split is deliberate. The model does the creative work: enumerating the classes of a contract,
imagining the inputs nobody expects, writing the probe. The binaries do the deterministic work:
installing the skills, wiring the hook that keeps them invoked, and reporting what the
environment can do so a skill degrades explicitly instead of failing on a tool it assumed.

## Install

```sh
go install github.com/alesierraalta/rdd-plus/cmd/rdd-plus@latest
rdd-plus sync      # installs the skills into ~/.claude/skills and wires the Stop hook
rdd-plus doctor    # verifies the install and lists optional capabilities
```

`sync` merges into `~/.claude/settings.json`: every existing hook and setting is preserved, the
gate is added once, and running it again changes nothing. A skill you edited locally is moved to
`~/.claude/skills/.rdd-plus-backup/<name>-<timestamp>/` before it is replaced. An unparseable
`settings.json` aborts the run before anything is written. Use `--config-dir` to target another
directory and `--dry-run` to see the plan.

## Commands

| Command | What it does |
|---|---|
| `rdd-plus gate` | The Stop hook. Reads the hook payload on stdin, decides, logs one line, and emits Stop feedback when a session changed production source without loading the adversarial testing discipline. Always exits 0. |
| `rdd-plus sync` | Installs the embedded skills and wires the gate. Idempotent. |
| `rdd-plus doctor` | Reports installed skills (and whether they drift from the embedded version), whether the hook is wired, and which optional tools are on PATH with what degrades without each. `--json` for machines. Exit 1 when git, a skill, or the hook is missing. |
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
(override with `TESTING_GATE_LOG`), so the invocation rate is measurable over time.

## Skills

The embedded skills live under `assets/skills/`. `test-strategy` is the organic entry point
("haz el testing"): it infers the mode from repository state, persists a plan that never
shrinks, sweeps the specialized skills for their own checks, and executes through
`exploit-testing`, whose ladder climbs from contract classes and hostile inputs to real
collaborators, real seams, injected faults, and concurrency. Every finding carries an executed
evidence record; anything not executed is a hypothesis.

## Development

```sh
go test ./...           # unit, integration (real git repositories), and differential tests
go run ./tools/mutants  # 23 literal mutants on the gate; every one must be killed
make build              # bin/rdd-plus
```

Integration tests build the CLI once and drive it with real repositories in temporary
directories; they are skipped under `-short`. The differential test compares the Go gate with the
original Node hook when `node` and `~/.claude/hooks/testing-gate.mjs` are present.

## Pending

- The eval harness (`assets/skills/test-strategy/evals/run_evals.py`, `selftest.py`) and the
  calibration tools (`assets/skills/test-strategy/assets/seed-mutants.py`, `fingerprint.sh`) are
  still Python and shell. They will become subcommands.
- Deterministic admission of evidence (re-executing a ledger row's command and re-applying its
  mutation before a finding is accepted), plan validation, and `status --next-transition` are the
  next binaries.

## License

Apache-2.0. See `LICENSE` and `NOTICE` for the attribution of the skills derived from the
Gentleman-Programming skills.

# Host adapters

The decisions live in the binary. A host adapter is transport: it answers when the agent
finishes, hands the binary the working directory, and puts the answer somewhere a person or a
model will read it.

Two shapes cover every host seen so far.

`tpp sync` installs the embedded skills into every host configuration directory it finds: Claude
Code and Pi read `~/.claude/skills`, OpenCode reads `~/.config/opencode/skills`, Gemini reads
`~/.gemini/skills`, and Codex reads `~/.codex/skills`. The Stop hook is wired only where its transport
is known—Claude Code's `~/.claude/settings.json`; OpenCode, Gemini, and Codex receive the skills, but
their transports are documented rather than wired by `sync`.

**Command hooks.** The host runs a command when the agent stops and reads JSON back. Claude Code
and Pi both work this way, and Pi's payload carries the same fields Claude's does
(`session_id`, `transcript_path`, `cwd`, `hook_event_name`, `stop_hook_active`), so
`tpp gate` serves both. Wire it with `tpp sync` on Claude Code, or by merging
`pi/settings.stop-hook.json` into Pi's settings.

**Plugins.** The host loads code and gives it an event bus. OpenCode works this way:
`opencode/tpp.ts` subscribes to `session.idle`, runs `tpp check`, and on a non-zero
exit shows a toast and appends the report to the prompt.

A host with neither still gets everything except "what did THIS session do": `tpp check`
reads git and the persisted plan and needs no host at all, which is also what makes it usable
from a Makefile, a pre-push script, or CI.

## Verified

| Host | Transport | Verified here |
|---|---|---|
| Claude Code | Stop command hook | yes: wired, run, exit code checked by `tpp doctor` |
| Anything with a shell | `tpp check` | yes |
| Pi | Stop command hook, Claude-compatible payload | contract read from the pi-hooks package; not run here |
| OpenCode | `session.idle` plugin event | contract read from the OpenCode plugin docs; not run here |
| Codex, Gemini CLI | not implemented | their payload and output schemas are not verified here |

"Not run here" means exactly that: the shape comes from each project's own documentation, and
nobody has watched it fire on this machine. A hook that looks wired and never answers is the
failure this project keeps finding, so treat those two rows as ready to test, not as working.

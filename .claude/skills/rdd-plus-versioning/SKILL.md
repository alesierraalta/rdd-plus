---
name: rdd-plus-versioning
description: "Trigger: versionar, version bump, release, subir versión, install latest rdd-plus. Bump rdd-plus and skill versions, ship, install the build."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
---

## Activation Contract

Load when a change to the binary (`cmd/`, `internal/`) or an embedded skill (`assets/skills/`) is about to
ship, when the operator asks to version or release, or to install the latest build on this machine.

## Hard Rules

- Every delivery that changes shipped behaviour bumps a version in the same PR or in one release PR
  right after; never leave merged behaviour on an old version string.
- Binary version lives only in `internal/buildinfo/buildinfo.go` (`var Version`). Bump the patch
  (`0.3.8` → `0.3.9`); a minor bump is the operator's call.
- A skill whose `SKILL.md`, `assets/` or `references/` changed bumps its own `metadata.version` patch.
- Set a skill's `requires_rdd_plus` to the new binary version only when the skill relies on behaviour
  that version introduced; otherwise leave it.
- Never rewrite historical records: `docs/testing/test-plan.md` rows, `bench/history.md`, fixture reports.
- No git tags or GitHub releases unless the operator asks; this repo does not use them.
- `main` is protected: ship through a PR whose `test` check passes, merged with a merge commit.
- Build the installed binary from a clean clone of `origin/main`, never from a checkout with staged or
  unstaged foreign changes.

## Decision Gates

| Changed | Bump |
|---|---|
| Binary only | `buildinfo.Version` patch |
| Skill text/assets only | that skill's `metadata.version` patch |
| Skill uses new binary behaviour | both, plus `requires_rdd_plus` |
| Tests, CI, `odd/`, `.claude/` only | nothing |

## Execution Steps

1. Diff against the last version bump: `git log --oneline -1 -- internal/buildinfo/buildinfo.go` and
   `git diff <that>..HEAD --stat -- cmd internal assets/skills`; pick bumps from the table.
2. Edit the version strings, then update every live mention: `rtk proxy grep -rn '<old>' internal cmd
   assets README.md` — the prose in `SKILL.md` ("written for `rdd-plus X`", "aligned with <skill> X")
   and `TestEmbeddedSkillIdentityNamesTheEmbeddedSkill` in `internal/feedback/feedback_test.go`.
3. `gofmt -l`, `go vet ./...`, `go test ./... -short -count=1`; `go run ./cmd/rdd-plus version`.
4. Commit `chore(release): rdd-plus X, <skill> Y` on a branch, open the PR listing what ships, wait for
   `test`, merge.
5. Install: follow [references/install.md](references/install.md).

## Output Contract

Report old → new for each version, the PR and merge commit, the installed `rdd-plus version` line, the
`doctor` verdict, and anything skipped.

## References

- [references/install.md](references/install.md) — build from a clean clone, back up, install, sync, doctor.
- `README.md` "Install with an agent" — the operator-facing install procedure this follows.

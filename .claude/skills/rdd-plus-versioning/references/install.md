# Install the latest rdd-plus on this machine

Run after the release PR is merged. `$S` is a scratch directory (the session scratchpad).

1. Clean clone, so no foreign staged change and no worktree lands in the binary. A `git worktree`
   build loses the VCS stamp (`rdd-plus version` prints `unknown`); a clone keeps it.

   ```sh
   rm -rf "$S/clone" && git clone -q --branch main https://github.com/alesierraalta/rdd-plus.git "$S/clone"
   (cd "$S/clone" && go build -o "$S/rdd-plus-new" ./cmd/rdd-plus)
   "$S/rdd-plus-new" version        # must print the new version and the merge commit, no +dirty
   ```

2. Back up and install over the binary on PATH (`command -v rdd-plus`, usually `~/.local/bin/rdd-plus`):

   ```sh
   cp -p "$(command -v rdd-plus)" "$S/rdd-plus-<old-version>.bak"
   install -m 0755 "$S/rdd-plus-new" "$(command -v rdd-plus)"
   rdd-plus version
   ```

3. Rehearse, then sync the embedded skills for real (edited skills are moved to
   `<config root>/backups/<timestamp>/`, so it is reversible):

   ```sh
   tmp=$(mktemp -d) && rdd-plus sync --config-dir "$tmp/cfg" && rdd-plus doctor --config-dir "$tmp/cfg"
   rdd-plus sync
   rdd-plus doctor                  # must exit 0 with verdict: healthy
   ```

4. Read doctor's binaries warning: the hook binary and the PATH binary must be the same file. If they
   differ, run `rdd-plus sync` again from the PATH binary and re-check.

Rollback: `install -m 0755 "$S/rdd-plus-<old-version>.bak" "$(command -v rdd-plus)"` and `rdd-plus sync`.

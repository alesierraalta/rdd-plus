# Install the latest tpp on this machine

Run after the release PR is merged. `$S` is a scratch directory (the session scratchpad).

1. Clean clone, so no foreign staged change and no worktree lands in the binary. A `git worktree`
   build loses the VCS stamp (`tpp version` prints `unknown`); a clone keeps it.

   ```sh
   rm -rf "$S/clone" && git clone -q --branch main https://github.com/alesierraalta/tpp.git "$S/clone"
   (cd "$S/clone" && go build -o "$S/tpp-new" ./cmd/tpp)
   "$S/tpp-new" version        # must print the new version and the merge commit, no +dirty
   ```

2. Back up and install over the binary on PATH (`command -v tpp`, usually `~/.local/bin/tpp`). On a
   machine that only has the pre-rename `rdd-plus`, there is nothing to back up: install to
   `~/.local/bin/tpp`, and step 3's `tpp sync` rewrites the old `rdd-plus gate` hook to the new binary.

   ```sh
   cp -p "$(command -v tpp)" "$S/tpp-<old-version>.bak"
   install -m 0755 "$S/tpp-new" "$(command -v tpp)"
   tpp version
   ```

3. Rehearse, then sync the embedded skills for real (edited skills are moved to
   `<config root>/backups/<timestamp>/`, so it is reversible):

   ```sh
   tmp=$(mktemp -d) && tpp sync --config-dir "$tmp/cfg" && tpp doctor --config-dir "$tmp/cfg"
   tpp sync
   tpp doctor                  # must exit 0 with verdict: healthy
   ```

4. Read doctor's binaries warning: the hook binary and the PATH binary must be the same file. If they
   differ, run `tpp sync` again from the PATH binary and re-check.

Rollback: `install -m 0755 "$S/tpp-<old-version>.bak" "$(command -v tpp)"` and `tpp sync`.

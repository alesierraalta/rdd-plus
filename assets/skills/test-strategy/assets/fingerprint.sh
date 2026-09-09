#!/usr/bin/env bash
# Content fingerprint for plan baselines and finding verdicts.
# Usage: fingerprint.sh [--root <repo>] [paths...]
#   no paths  -> fingerprint of every tracked + untracked (non-ignored) file in the repo
#   paths     -> fingerprint of exactly those files (a finding's cited files)
# Output: sha256 hex over "path<TAB>sha256(content)" lines, sorted; deterministic across machines.
set -euo pipefail
ROOT="."
if [[ "${1:-}" == "--root" ]]; then ROOT="$2"; shift 2; fi
cd "$ROOT"
if [[ $# -eq 0 ]]; then
  git ls-files -co --exclude-standard -z
else
  printf '%s\0' "$@"
fi | sort -z | while IFS= read -r -d '' f; do
  [[ -f "$f" ]] || continue
  printf '%s\t%s\n' "$f" "$(sha256sum -- "$f" | cut -c1-64)"
done | sha256sum | cut -c1-64

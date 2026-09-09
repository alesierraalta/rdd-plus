#!/usr/bin/env bash
# Preview ephemeral objects. Destructive mode requires a bounded ownership label
# and a separate confirmation; the ephemeral label alone is never sufficient.
#
#   ./sweep-ephemeral.sh
#   ./sweep-ephemeral.sh --scope-label run=ci-123
#   ./sweep-ephemeral.sh --apply --scope-label run=ci-123 --confirm
set -Eeuo pipefail

LABEL="${EPHEMERAL_LABEL:-ephemeral=1}"
APPLY=0
CONFIRM=0
SCOPE=""
usage() {
  cat <<'USAGE'
Usage: sweep-ephemeral.sh [--scope-label KEY=VALUE] [--apply --confirm]

Dry-run is the default. --apply requires both an exact ownership label and
--confirm. Scope must include a run/project-unique value; the ephemeral label
alone or a scope reused across projects is ambiguous and unsafe. Objects carrying
only the ephemeral label are retained and reported.
USAGE
}
while [ "$#" -gt 0 ]; do
  case "$1" in
    --scope-label) [ "$#" -ge 2 ] || { usage >&2; exit 2; }; SCOPE="$2"; shift 2 ;;
    --apply) APPLY=1; shift ;;
    --confirm) CONFIRM=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
  esac
done
[[ "$SCOPE" =~ ^[A-Za-z0-9_.-]+=[^[:space:]]+$ ]] || [ -z "$SCOPE" ] || { echo "invalid --scope-label: $SCOPE" >&2; exit 2; }
if [ "$APPLY" = 1 ] && { [ -z "$SCOPE" ] || [ "$CONFIRM" != 1 ]; }; then
  echo "--apply requires --scope-label KEY=VALUE and separate --confirm" >&2; exit 2
fi

hr() { printf '\n%s\n' "── $* ──────────────────────────────"; }
filters=(--filter "label=$LABEL")
[ -n "$SCOPE" ] && filters+=(--filter "label=$SCOPE")

list_ids() {
  local kind="$1" target="$2" output
  shift 2
  if ! output="$("$@" 2>/dev/null)"; then
    echo "ERROR: docker list failed for $kind; refusing to treat failure as empty" >&2
    return 1
  fi
  local -n result="$target"
  result=()
  if [ -n "$output" ]; then mapfile -t result <<<"$output"; fi
}
report_set() {
  local kind="$1"; shift
  local -a ids=("$@")
  if [ "${#ids[@]}" -eq 0 ]; then echo "  EMPTY: no selected $kind"; else printf '  %s\n' "${ids[@]}"; fi
}

list_ids containers all_cids docker ps -aq --filter "label=$LABEL"
list_ids containers selected_cids docker ps -aq "${filters[@]}"
list_ids volumes all_vids docker volume ls -q --filter "label=$LABEL"
list_ids volumes selected_vids docker volume ls -q "${filters[@]}"
list_ids networks all_nids docker network ls -q --filter "label=$LABEL"
list_ids networks selected_nids docker network ls -q "${filters[@]}"

hr "Selected containers${SCOPE:+ ($SCOPE)}"; report_set containers "${selected_cids[@]}"
hr "Selected volumes${SCOPE:+ ($SCOPE)}"; report_set volumes "${selected_vids[@]}"
hr "Selected networks${SCOPE:+ ($SCOPE)}"; report_set networks "${selected_nids[@]}"

if [ -z "$SCOPE" ]; then
  hr "Unowned/ambiguous ephemeral objects"
  echo "  ${#all_cids[@]} containers, ${#all_vids[@]} volumes, ${#all_nids[@]} networks; no scope selected"
else
  declare -A selected=()
  for id in "${selected_cids[@]}" "${selected_vids[@]}" "${selected_nids[@]}"; do [ -n "$id" ] && selected["$id"]=1; done
  hr "Retained unowned/ambiguous ephemeral objects"
  retained=0
  for id in "${all_cids[@]}" "${all_vids[@]}" "${all_nids[@]}"; do
    [ -n "$id" ] && [ -z "${selected[$id]+x}" ] && { echo "  $id"; retained=1; }
  done
  [ "$retained" = 0 ] && echo "  none"
fi

hr "Containers that resurrect (restart policy != no)"
found=0
list_ids containers all_names docker ps -a --format '{{.Names}}'
for name in "${all_names[@]}"; do
  [ -z "$name" ] && continue
  if ! policy="$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$name" 2>/dev/null)"; then
    echo "ERROR: docker inspect failed for $name; refusing to assume restart policy is no" >&2
    exit 1
  fi
  if [ "$policy" != "no" ] && [ -n "$policy" ]; then
    printf '  %-18s %s\n' "$policy" "$name"
    found=1
  fi
done
[ "$found" = 0 ] && echo "  none"
echo "Restarting containers are never removed automatically."
echo "Manual retirement requires identified ownership, explicit authorization, and confirmation."

if [ "$APPLY" = 1 ]; then
  if [ "${#selected_cids[@]}" -eq 0 ] && [ "${#selected_vids[@]}" -eq 0 ] && [ "${#selected_nids[@]}" -eq 0 ]; then
    hr "EMPTY"; echo "No owned objects selected; successful no-op."
    exit 0
  fi
  hr "Deleting selected objects"
  ((${#selected_cids[@]})) && docker rm -f -v "${selected_cids[@]}"
  ((${#selected_vids[@]})) && docker volume rm "${selected_vids[@]}"
  ((${#selected_nids[@]})) && docker network rm "${selected_nids[@]}"
else
  hr "DRY RUN"; echo "Nothing was deleted."
fi


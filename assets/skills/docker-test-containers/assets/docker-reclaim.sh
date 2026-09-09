#!/usr/bin/env bash
# Staged Docker reclaim, safest step first. DRY RUN by default.
#
#   ./docker-reclaim.sh
#   ./docker-reclaim.sh --scope-label run=ci-123
#   ./docker-reclaim.sh --apply --scope-label run=ci-123 --confirm
#
# Destructive mode delegates only to the ownership-scoped sweep. It never runs
# global volume/container/image prune, removes unlabeled/ambiguous named volumes, or removes tagged images.
set -Eeuo pipefail

APPLY=0
CONFIRM=0
SCOPE=""
usage() {
  cat <<'USAGE'
Usage: docker-reclaim.sh [--scope-label KEY=VALUE] [--apply --confirm]

Reports first and is dry-run by default. --apply delegates container, volume,
and network deletion to one ownership-scoped sweep execution.
Unlabeled or ambiguous named volumes are retained; scope-labeled volumes are
owned by that scope and removed by the sweep.
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
SWEEP=("$(dirname "$0")/sweep-ephemeral.sh")
[ -n "$SCOPE" ] && SWEEP+=(--scope-label "$SCOPE")
before="$(docker system df)"
hr "BEFORE"; echo "$before"

hr "1. Ownership-scoped ephemeral objects"
orphans="$(docker volume ls -qf dangling=true | grep -E '^[0-9a-f]{64}$' || true)"
echo "unused anonymous volumes visible: $(printf '%s\n' "$orphans" | grep -c . || true)"
named_unused="$(docker volume ls -qf dangling=true | grep -vE '^[0-9a-f]{64}$' || true)"
if [ -n "$named_unused" ]; then
  echo "RETAINED unlabeled/ambiguous named volumes:"; printf '  %s\n' "$named_unused"
fi
if [ "$APPLY" = 1 ]; then
  "${SWEEP[@]}" --apply --confirm
else
  "${SWEEP[@]}"
fi

hr "2. Stopped containers"
echo "stopped/created visible: $(docker ps -aq -f status=exited -f status=created | grep -c . || true)"
echo "Only selected, owned ephemeral containers can be removed by step 1; global prune is disabled."

hr "3. Dangling images"
echo "dangling visible: $(docker images -qf dangling=true | grep -c . || true)"
echo "Retained; inspect ownership and remove selected image IDs manually."

hr "4. Build cache (report only)"
docker system df -v | grep -i -A2 'build cache' || echo "Build cache details unavailable in this Docker version."

after_report() { hr "AFTER"; docker system df; }
if [ "$APPLY" = 1 ]; then
  hr "APPLY"; after_report
else
  hr "DRY RUN"; echo "Nothing was deleted."
fi

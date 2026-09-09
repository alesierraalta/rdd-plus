#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  cat <<'EOF'
Usage:
  migration-idempotency-check.sh --mode reversible --target LABEL -- \
    UP_EXECUTABLE [UP_ARGS...] -- DOWN_EXECUTABLE [DOWN_ARGS...] -- \
    DUMP_EXECUTABLE [DUMP_ARGS...]

The three argv arrays are separated by --. Commands must be explicitly supplied;
this check never uses DATABASE_URL or chooses a migration direction. The caller
must complete higher-level target authorization and preflight first.
EOF
}

fail() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 2
}

if (( $# == 0 )); then
  usage >&2
  exit 2
fi

mode=''
target=''
while (( $# )); do
  case "$1" in
    --mode)
      (( $# >= 2 )) || fail '--mode requires a value'
      mode=$2
      shift 2
      ;;
    --target)
      (( $# >= 2 )) || fail '--target requires a value'
      target=$2
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    *)
      fail "unexpected option or argument: $1"
      ;;
  esac
done

[[ $mode == reversible ]] || {
  case "$mode" in
    expand|contract|expand-contract|irreversible)
      fail "migration mode '$mode' is not reversible; use the compatibility/rollback/backup-restore path; DOWN was not run"
      ;;
    '') fail 'explicit --mode reversible is required' ;;
    *) fail "unsupported migration mode '$mode'; declare reversible or use the compatibility/rollback/backup-restore path" ;;
  esac
}
[[ -n $target ]] || fail 'nonempty --target label is required; higher-level authorization remains the caller responsibility'

up=()
down=()
dump=()
phase=up
for arg in "$@"; do
  if [[ $arg == -- ]]; then
    case "$phase" in
      up) phase=down ;;
      down) phase=dump ;;
      dump) fail 'too many command separators' ;;
    esac
  else
    case "$phase" in
      up) up+=("$arg") ;;
      down) down+=("$arg") ;;
      dump) dump+=("$arg") ;;
    esac
  fi
done
[[ $phase == dump ]] || fail 'three command argv arrays require two -- separators'
(( ${#up[@]} && ${#down[@]} && ${#dump[@]} )) || fail 'each command must be nonempty'

umask 077
workspace=$(mktemp -d -t migration-check.XXXXXXXXXX)
cleanup() {
  local status=$?
  trap - EXIT
  rm -rf -- "$workspace"
  exit "$status"
}
trap cleanup EXIT

run_dump() {
  local name=$1
  shift
  "${dump[@]}" > "$workspace/$name"
}

printf 'Target label accepted; caller-owned authorization/preflight is not established by this label.\n'
printf 'Canonical schema dumps only; this does not prove data preservation, semantic dump normalization, or application compatibility.\n'
printf '[1/7] Dumping baseline schema...\n'
run_dump baseline.sql
printf '[2/7] Applying migration UP...\n'
"${up[@]}"
printf '[3/7] Dumping post-UP schema...\n'
run_dump post-up.sql
printf '[4/7] Reverting migration DOWN...\n'
"${down[@]}"
printf '[5/7] Dumping rollback schema and comparing with baseline...\n'
run_dump rollback.sql
diff -u "$workspace/baseline.sql" "$workspace/rollback.sql"
printf '[6/7] Re-applying migration UP and dumping schema...\n'
"${up[@]}"
run_dump reapplied.sql
diff -u "$workspace/post-up.sql" "$workspace/reapplied.sql"
printf 'SUCCESS: canonical baseline and rollback dumps match, and the two post-UP canonical dumps match; this reversible migration check passed.\n'

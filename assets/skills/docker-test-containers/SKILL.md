---
name: docker-test-containers
description: "Trigger: start a throwaway container for a test, spin up Postgres/Redis/Mongo for an integration run, docker compose for local testing, 'no space left on device', reclaiming Docker disk, containers or volumes piling up, WSL disk keeps growing. Ephemeral containers that cost zero disk, plus staged reclaim."
license: Apache-2.0
metadata:
  author: "alesierraalta"
  version: "1.0"
  scope: [global]
  auto_invoke: "Before starting any container used for testing, and whenever Docker disk usage is the problem"
---

## Activation Contract

Load before starting **any** container for tests, evals, or local experiments, and whenever
Docker disk usage is the complaint.

This skill is harness-neutral: Claude Code, opencode, and pi all read it from the same file.

## Cost Model — read this before deciding what to delete

"Can I keep containers around without paying disk?" The answer is different per object type,
and getting it wrong is why disks fill up.

| Object | What keeping it actually costs | Verdict |
|---|---|---|
| **Image** | Content-addressed layers, shared. Twenty containers off one image cost **one** image. | Keep a small pinned set. Cheap. |
| **Stopped container** | Only its writable layer — usually a few MB **of disk**. | Cheap in bytes, expensive in clutter: see below. |
| **Named volume** | A full copy of the data directory. Survives `docker rm` **and** `docker compose down`. | Never keep one for a test. |
| **Anonymous volume** | Same full copy, with no name to notice it by. One created per run. | This is the leak. |
| **Build cache** | Grows unbounded until a GC policy caps it. | Cap it in `daemon.json`. |

Measured on this machine (Docker 29.1.5, native in WSL2, 2026-08-23):

- 50 containers totalled **183 MB**. Containers were never the problem.
- 120 volumes, **108 anonymous**, **99 of them fully orphaned**, holding **~25.7 GB**.
- Images: 37.4 GB, 24.4 GB reclaimable — from many distinct tags, not from many containers.

So: **volumes are what costs disk.** But "cheap in bytes" is not "harmless" — containers accumulate
in three ways that bytes do not measure, and all three were present on this machine:

- **Name and port collisions.** `docker run --name test-pg` fails outright once a dead `test-pg`
  still exists, and a leftover `-p 55432` steals the port from the next run.
- **Resurrection.** 20+ containers here carry `restart: unless-stopped`. They come back on every
  daemon start, hold their ports, and keep their volumes referenced — so `docker volume prune`
  cannot reclaim those volumes at all.
- **Unfindability.** 50 containers here, **0 with a label**. With nothing marking which ones were
  throwaway, the only safe sweep is none, so they stay forever. 27 were sitting `exited`.

The fix for all three is the label discipline plus `--rm` below. Measured here: **`AutoRemove` was
`false` on all 50 containers** — the rule exists, it is the one that never gets applied. Prefer a
mechanism that survives forgetting it: label everything, sweep by label.

## Hard Rules

- Every test `docker run` carries **both** `--rm` and `--label ephemeral=1`. `--rm` removes the
  container and its anonymous volumes on exit; the label is the safety net for when `--rm` did not
  fire — daemon restart, `kill -9`, a machine reboot. Measured here: `AutoRemove` was `false` on
  all 50 existing containers, so assume `--rm` will be forgotten and make the sweep possible anyway.
- Compose gets the same label, once, per project:
  `docker compose -p <project> --label-file` does not exist — put it in the YAML instead:
  `labels: { ephemeral: "1" }` on every service.
- `docker compose run` leaves the container behind. It is **`docker compose run --rm`**, always.
  `docker compose up` does not need it because `down -v` handles teardown.
- **Never give a test container a restart policy.** No `--restart`, no `restart:` in the YAML.
  A resurrecting container holds its port and keeps its volume referenced, which means
  `docker volume prune` can never reclaim it. Find offenders with the sweep section below.
- `docker rm <c>` does **not** remove the anonymous volume — verified: the volume count stayed up
  after `docker rm`. Only `docker rm -v <c>` removes it.
- `docker compose down` does **not** remove named volumes. Always
  `docker compose -p <owned-test-project> down -v --remove-orphans` (after preview and explicit confirmation; retain state when an inspection or rollback exception applies).
- Put test state on `--tmpfs`, never on a volume. It lives in RAM, costs zero disk, and dies with
  the container. Verified: `pgvector/pgvector:pg16` with `PGDATA` on tmpfs accepted connections in
  **2 s** and created **zero** volumes.
- Never build a new image tag per test run. Reuse one tag, or use the official image plus mounted
  init scripts. A fresh tag per run leaves the previous one dangling — that is the 24 GB above.
- Never run bare `docker system prune -a` to fix disk pressure. It deletes images that stopped work
  still needs and the entire build cache. Use the staged reclaim below.
- Bind a fixed high port per service (`-p 55432:5432`), or `-p 0:5432` and read the port back.
  Never bind the default port: a real local service or a parallel test will steal it.
- Clean up in a `trap`, not at the end of the script. An interrupted run must not leak.
- WSL2: freeing space inside Docker does **not** return disk to Windows. The backing VHDX only
  grows. See [References](#references).

## Decision Gates

| Situation | Do this |
|---|---|
| Need a DB for a test run | tmpfs recipe + `--rm`. Zero disk, zero cleanup. |
| Need >1 service wired together | Compose with a unique `-p` project and a `trap ... down -v`. |
| Need to inspect state after a failure | Run **without** `--rm`, inspect, then `docker rm -v <c>`. Never leave it. |
| Dataset too large for RAM | Named volume with an explicit name you can find, plus teardown in the same script. |
| Same service across many projects | One pinned tag everywhere. Ten projects, one image. |
| "No space left on device" | Staged reclaim below, in order. Stop at the first step that frees enough. |

## Recipes

### Postgres + pgvector, zero disk (verified)

```bash
RUN_ID="pg-$$"
docker run --rm -d --name test-pg --label ephemeral=1 --label "run=$RUN_ID" \
  -e POSTGRES_PASSWORD=test -e PGDATA=/pgdata \
  --tmpfs /pgdata:rw,size=512m,mode=1777 \
  -p 55432:5432 pgvector/pgvector:pg16
```

`PGDATA` points **outside** the image's declared `VOLUME`, so no anonymous volume is ever created.
Mounting the tmpfs directly at `/var/lib/postgresql/data` also works — both were verified to leave
the volume count unchanged. Plain `postgres:16` has no `vector` extension; use the pgvector image
when the code needs embeddings.

Wait on readiness, never on `sleep`:

```bash
for i in $(seq 1 30); do docker exec test-pg pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
```

### Redis, no persistence

```bash
RUN_ID="redis-$$"
docker run --rm -d --name test-redis --label ephemeral=1 --label "run=$RUN_ID" -p 56379:6379 \
  redis:7-alpine redis-server --save "" --appendonly no
```

Disabling both snapshots and the AOF keeps Redis entirely out of the filesystem.

### Compose for a test run

```bash
P="test-$$"
trap 'docker compose -p "$P" down -v --remove-orphans' EXIT INT TERM
docker compose -p "$P" up -d --wait
# one-off command in a new container: --rm or it stays behind
docker compose -p "$P" run --rm app pytest
```

The unique `-p` lets parallel runs coexist and makes teardown exact. `--wait` blocks on each
service's healthcheck, so no polling loop is needed.

```yaml
services:
  db:
    image: pgvector/pgvector:pg16
    labels:
      ephemeral: "1"
      run: "${TEST_RUN_ID:?set TEST_RUN_ID}"
    environment:
      POSTGRES_PASSWORD: test
      PGDATA: /pgdata
    tmpfs:
      - /pgdata:size=512m,mode=1777
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 1s
      retries: 30
```

### Helper scripts

- `assets/ephemeral-pg.sh` — starts a tmpfs Postgres on a free port, waits for readiness, prints
  the DSN, and removes it on exit.
- `assets/sweep-ephemeral.sh` — previews exact container/volume/network IDs, then removes only an explicitly confirmed ownership scope (`--apply --scope-label run=... --confirm`); retains and reports ambiguous/unowned items. Dry-run by default. Listing or inspect failures are errors, never empty results.
- `assets/docker-reclaim.sh` — the staged reclaim below, dry-run by default; apply delegates one sweep execution to avoid stale preview/delete selections.

## Sweep — the safety net for owned, labelled state

`--rm` fails open: if the daemon restarts, the host reboots, or the process is `kill -9`'d, the
container survives. The label makes survivors findable, but ownership and the test-run target still must be confirmed. A bounded preview identifies candidates without implying that every labelled object is disposable:

```bash
./assets/sweep-ephemeral.sh --scope-label run=ci-123       # preview exact IDs
# After checking the preview, confirm this bounded scope explicitly:
./assets/sweep-ephemeral.sh --apply --scope-label run=ci-123 --confirm
```

The label is necessary but not sufficient: identify the owning test run/project and preview the exact target set first. The scope value must include a unique run/project ID (for example, `run=ci-123`); a scope equal only to `ephemeral=1`, or reused by multiple projects, cannot distinguish ownership and is unsafe. Require explicit confirmation before destructive deletion, keep scope bounded, and never treat all labelled state as disposable when ownership is uncertain. Report skipped ambiguous or unowned items. Unlabelled project services remain out of reach.

**`-v` is not optional, and the label alone will not find the volumes.** Docker does **not**
propagate a container's labels to its anonymous volumes, so
`docker volume ls --filter label=ephemeral=1` comes back empty even when those volumes exist. The
`-v` on `docker rm` is the only thing that reclaims them. Verified: two survivors had leaked two
anonymous volumes, and `docker rm -f -v` took the count from 24 back to exactly 22.

That is also why the sweep must run **before** `docker volume prune`: a volume still attached to a
surviving container is "in use" and prune skips it. Containers first, volumes second.

Volumes and networks you create *explicitly* should carry both labels so they are findable within one bounded run:

```bash
docker volume create --label ephemeral=1 --label run=ci-123 test-data
docker network create --label ephemeral=1 --label run=ci-123 test-net
./assets/sweep-ephemeral.sh --scope-label run=ci-123
./assets/sweep-ephemeral.sh --apply --scope-label run=ci-123 --confirm
```

Run the ownership-filtered preview at the **start** of a test script, not only the end. The
`ephemeral=1` label alone never authorizes deletion; an ambiguous or unowned object is reported. Unlabeled or ambiguous named volumes are retained, while explicitly scope-labeled named volumes are owned by that scope and removed.
and retained.

### Finding what resurrects

```bash
docker ps -a --format '{{.Names}}' | while read n; do
  p=$(docker inspect -f '{{.HostConfig.RestartPolicy.Name}}' "$n")
  [ "$p" != "no" ] && echo "$p  $n"
done
```

Anything listed here comes back on its own. Before manually retiring one, identify its owner,
obtain explicit authorization and confirmation, and verify the intended retention decision. Only
then may an authorized operator disable its restart policy and remove it; never treat
`docker update --restart=no <name> && docker rm -v <name>` as an unguarded action.

`assets/sweep-ephemeral.sh` previews all ephemeral candidates but deletes only the explicitly
owned scope, dry-run by default. `assets/docker-reclaim.sh` preserves the report-first stages and
routes any destructive cleanup through that scoped sweep.

## Reclaim — staged, safest first

Stop at the first step that frees enough. Never skip ahead.

1. **Measure.** `docker system df -v`. Read which row is actually large before deleting anything.
2. **Ownership-scoped ephemeral volumes and containers** — run `assets/sweep-ephemeral.sh --scope-label run=<id>`
   to preview exact IDs, then use `--apply --scope-label run=<id> --confirm` only after checking the
   preview. `docker-reclaim.sh --apply` delegates one bounded sweep execution; it never runs global
   volume/container/image prune. Unlabeled or ambiguous named volumes are retained and reported;
   explicitly scope-labeled named volumes are owned and removed.
3. **Dangling images** (the untagged leftovers of rebuilds). Report them, inspect ownership, and
   remove selected image IDs manually; helpers never delete images automatically.
4. **Build cache.** Report its size before choosing a separately approved cache policy. Do not let
   a host-wide prune hide ownership decisions.
5. **Tagged images, only with a list in hand.** `docker image ls --format '{{.Size}}\t{{.Repository}}:{{.Tag}}' | sort -h`,
   then remove chosen tags by name. Do not use `docker image prune -a`.

### Cap the growth instead of repeating the cleanup

There is no `/etc/docker/daemon.json` on this machine, so no builder GC policy is in force. Add one:

```json
{
  "builder": {
    "gc": {
      "enabled": true,
      "defaultKeepStorage": "20GB"
    }
  }
}
```

Then `sudo systemctl restart docker`. This bounds the build cache permanently; the other four
categories are bounded by the `--rm` / `down -v` habits above.

## Output Contract

When this skill is applied, report:

- the exact `docker run` / compose invocation used, and that it carries `--rm` or a `trap` teardown;
- the port bound and how readiness was confirmed;
- for a reclaim: the `docker system df` figures **before and after**, and which step freed the space.

Never report a reclaim as "cleaned up" without before/after numbers, selected targets, and skipped ambiguities. Never run destructive deletion without explicit confirmation; orphaned data is still data until ownership and retention are established.

## References

- `assets/ephemeral-pg.sh`, `assets/sweep-ephemeral.sh`, `assets/docker-reclaim.sh` — the helpers.
- WSL2 reclaim: freeing space inside the Linux filesystem never shrinks the backing VHDX. From
  Windows: `wsl --shutdown`, then `Optimize-VHD -Path <ext4.vhdx> -Mode Full` (Hyper-V), or
  `diskpart` → `select vdisk file="<path>"` → `compact vdisk`. Find the disk with
  `wsl.exe --list --verbose` and the distro's `BasePath` in the registry under
  `HKCU\Software\Microsoft\Windows\CurrentVersion\Lxss`.
- Docker docs: `docker volume prune`, `docker builder prune`, compose `tmpfs` and `--wait`.

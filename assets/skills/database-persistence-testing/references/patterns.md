# Database & Persistence Testing Reference Patterns

## 0. Safe Reversible Migration Check

`assets/migration-idempotency-check.sh` is limited to explicitly declared reversible migrations. Invoke it with a nonempty `--target` label and three nonempty executable-plus-argument argv arrays separated by `--`:

```sh
./assets/migration-idempotency-check.sh --mode reversible --target users-v3 -- \
  alembic upgrade head -- alembic downgrade -1 -- pg_dump --schema-only --dbname test_database
```

The caller must complete authorization and preflight; the label is not proof of authorization. The script uses a private temporary workspace and proves only canonical schema-dump equality: baseline vs rollback and first UP vs re-applied UP. It does not prove data preservation, semantic dump normalization, or application compatibility. Expand/contract or irreversible migrations must use the compatibility and documented rollback or backup/restore path instead; this check never runs DOWN for them.

## 1. Unindexed Foreign Key Detection Query (PostgreSQL)

Unindexed foreign keys cause PostgreSQL to acquire full-table `SHARE ROW EXCLUSIVE` locks on the referencing table during updates/deletes to the referenced table's primary key:

```sql
SELECT
    c.conrelid::regclass AS table_name,
    c.conname AS foreign_key_name,
    pg_get_constraintdef(c.oid) AS constraint_definition
FROM pg_constraint c
WHERE c.contype = 'f'
  AND NOT EXISTS (
      SELECT 1
      FROM pg_index i
      WHERE i.indrelid = c.conrelid
        AND c.conkey = (i.indkey::smallint[])[1:array_length(c.conkey, 1)]
  )
ORDER BY table_name, foreign_key_name;
```

## 2. Zero-Downtime Expand/Contract Migration Protocol

### Phase 1: Expand
Add new column as nullable, or with non-blocking default (Postgres 11+ supports instant metadata defaults):
```sql
-- Migration 20260904_expand_user_full_name.sql
SET lock_timeout = '2s';
ALTER TABLE users ADD COLUMN full_name VARCHAR(255);
```
Code writes to BOTH `first_name`/`last_name` AND `full_name`. Code reads from `first_name`/`last_name`.

### Phase 2: Backfill (Chunked Background Migration)
```python
# background_backfill.py
BATCH_SIZE = 1000
cursor = 0

while True:
    rows_updated = db.execute("""
        WITH target AS (
            SELECT id FROM users
            WHERE id > :cursor AND full_name IS NULL
            ORDER BY id ASC LIMIT :batch_size
        )
        UPDATE users u
        SET full_name = CONCAT(u.first_name, ' ', u.last_name)
        FROM target
        WHERE u.id = target.id
        RETURNING u.id;
    """, {"cursor": cursor, "batch_size": BATCH_SIZE})
    
    if not rows_updated:
        break
    cursor = max(r[0] for r in rows_updated)
    time.sleep(0.05) # Throttle to prevent replication lag and WAL saturation
```

### Phase 3: Contract
After all rows are backfilled and code reads exclusively from `full_name`:
```sql
-- Migration 20260905_contract_drop_legacy_names.sql
SET lock_timeout = '2s';
ALTER TABLE users ALTER COLUMN full_name SET NOT NULL;
ALTER TABLE users DROP COLUMN first_name;
ALTER TABLE users DROP COLUMN last_name;
```

## 3. Concurrency Lock Assertion (Pessimistic Queue Pattern)

```python
import threading
import pytest

def test_concurrent_worker_skip_locked(db_engine):
    """Ensure two workers never process the same queue item."""
    processed_items = []
    lock = threading.Lock()

    def worker():
        with db_engine.connect() as conn:
            with conn.begin():
                row = conn.execute("""
                    SELECT id FROM task_queue 
                    WHERE status = 'pending' 
                    FOR UPDATE SKIP LOCKED 
                    LIMIT 1;
                """).fetchone()
                if row:
                    task_id = row[0]
                    with lock:
                        processed_items.append(task_id)
                    conn.execute("UPDATE task_queue SET status = 'completed' WHERE id = :id", {"id": task_id})

    threads = [threading.Thread(target=worker) for _ in range(10)]
    for t in threads: t.start()
    for t in threads: t.join()

    # Assert no duplicate processing
    assert len(processed_items) == len(set(processed_items))
```

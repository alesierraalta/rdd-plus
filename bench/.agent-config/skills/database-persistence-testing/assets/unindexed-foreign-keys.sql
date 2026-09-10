-- Audit for unindexed foreign keys in PostgreSQL
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

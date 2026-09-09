# keyset-pagination

Cursor pagination over an event feed ordered by `created_at`.

- `page(rows, { after, limit })` returns the next `limit` rows in feed order and a cursor for the
  following call; `after` is the cursor returned by the previous page, or `null` for the first.
- `collectAll(rows, limit)` walks every page until the cursor is exhausted.
- Walking the pages yields every row exactly once, whatever the page size.

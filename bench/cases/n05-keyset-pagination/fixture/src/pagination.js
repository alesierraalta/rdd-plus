// Feed order: created_at ascending, then id for a stable order.
function inFeedOrder(rows) {
  return rows.slice().sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id));
}

// One page after the cursor; the cursor is the created_at of the last row returned.
export function page(rows, { after = null, limit = 2 } = {}) {
  if (!Number.isInteger(limit) || limit < 1) throw new RangeError("limit must be a positive integer");
  const ordered = inFeedOrder(rows);
  const remaining = after === null ? ordered : ordered.filter((r) => r.created_at > after);
  const items = remaining.slice(0, limit);
  const cursor = items.length === limit && remaining.length > limit ? items[items.length - 1].created_at : null;
  return { items, cursor };
}

// Every page, concatenated.
export function collectAll(rows, limit) {
  const out = [];
  let after = null;
  for (;;) {
    const { items, cursor } = page(rows, { after, limit });
    out.push(...items);
    if (cursor === null) return out;
    after = cursor;
  }
}

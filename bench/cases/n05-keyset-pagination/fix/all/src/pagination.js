// Feed order: created_at ascending, then id for a stable order.
function inFeedOrder(rows) {
  return rows.slice().sort((a, b) => a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id));
}

const CURSOR_SEP = "#";

// A cursor is the created_at of the last row, plus its id when another row shares that created_at.
function encodeCursor(last, rows) {
  const shared = rows.some((r) => r !== last && r.created_at === last.created_at);
  return shared ? `${last.created_at}${CURSOR_SEP}${last.id}` : last.created_at;
}

function decodeCursor(after) {
  const at = after.indexOf(CURSOR_SEP);
  return at === -1 ? { created_at: after, id: null } : { created_at: after.slice(0, at), id: after.slice(at + 1) };
}

function isAfter(row, cursor) {
  if (row.created_at !== cursor.created_at) return row.created_at > cursor.created_at;
  return cursor.id !== null && row.id > cursor.id;
}

// One page after the cursor.
export function page(rows, { after = null, limit = 2 } = {}) {
  if (!Number.isInteger(limit) || limit < 1) throw new RangeError("limit must be a positive integer");
  const ordered = inFeedOrder(rows);
  const remaining = after === null ? ordered : ordered.filter((r) => isAfter(r, decodeCursor(after)));
  const items = remaining.slice(0, limit);
  const cursor = items.length === limit && remaining.length > limit ? encodeCursor(items[items.length - 1], ordered) : null;
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

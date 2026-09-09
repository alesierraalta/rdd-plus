import { test } from "node:test";
import assert from "node:assert/strict";
import { page, collectAll } from "../src/pagination.js";

const rows = [
  { id: "c", created_at: "2026-01-03T00:00:00Z" },
  { id: "a", created_at: "2026-01-01T00:00:00Z" },
  { id: "b", created_at: "2026-01-02T00:00:00Z" },
  { id: "d", created_at: "2026-01-04T00:00:00Z" },
];

test("first page in feed order with a cursor", () => {
  const { items, cursor } = page(rows, { limit: 2 });
  assert.deepEqual(items.map((r) => r.id), ["a", "b"]);
  assert.equal(cursor, "2026-01-02T00:00:00Z");
});

test("second page continues after the cursor", () => {
  const { items, cursor } = page(rows, { after: "2026-01-02T00:00:00Z", limit: 2 });
  assert.deepEqual(items.map((r) => r.id), ["c", "d"]);
  assert.equal(cursor, null);
});

test("collecting every page yields every row once", () => {
  assert.deepEqual(collectAll(rows, 3).map((r) => r.id), ["a", "b", "c", "d"]);
});

test("a limit larger than the feed returns everything", () => {
  assert.equal(page(rows, { limit: 10 }).items.length, 4);
});

test("an empty feed has no items and no cursor", () => {
  assert.deepEqual(page([], { limit: 2 }), { items: [], cursor: null });
});

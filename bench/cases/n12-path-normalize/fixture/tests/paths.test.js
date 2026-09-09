import { test } from "node:test";
import assert from "node:assert/strict";
import { safeJoin, PathTraversal } from "../src/paths.js";

const root = "/srv/files";

test("joins a plain relative path", () => {
  assert.equal(safeJoin(root, "docs/report.pdf"), "/srv/files/docs/report.pdf");
});

test("decodes url-encoded names", () => {
  assert.equal(safeJoin(root, "docs/annual%20report.pdf"), "/srv/files/docs/annual report.pdf");
});

test("rejects a parent segment", () => {
  assert.throws(() => safeJoin(root, "../etc/passwd"), PathTraversal);
});

test("rejects an absolute path", () => {
  assert.throws(() => safeJoin(root, "/etc/passwd"), PathTraversal);
});

test("rejects an empty path", () => {
  assert.throws(() => safeJoin(root, ""), TypeError);
});

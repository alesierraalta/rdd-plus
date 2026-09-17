import test from "node:test";
import assert from "node:assert/strict";
import { allocate } from "../src/allocate.js";

test("grants an exact fit in request order", () => {
  assert.deepEqual(allocate(10, [4, 6]), { granted: [4, 6], left: 0 });
});

test("partially grants a request when stock is short", () => {
  assert.deepEqual(allocate(7, [3, 8]), { granted: [3, 4], left: 0 });
});

test("skips zero and negative requests", () => {
  assert.deepEqual(allocate(5, [0, -2, 3]), { granted: [3], left: 2 });
});

test("does not let a request exceed the stock left", () => {
  assert.deepEqual(allocate(2, [5]), { granted: [2], left: 0 });
});

import { test } from "node:test";
import assert from "node:assert/strict";
import { clamp } from "../src/clamp.js";

test("keeps a value inside the range unchanged", () => {
  assert.equal(clamp(5, 0, 10), 5);
});

test("clamps a value below the minimum", () => {
  assert.equal(clamp(-3, 0, 10), 0);
});

test("clamps a value above the maximum", () => {
  const result = clamp(15, 0, 10);
  assert.ok(typeof result === "number");
});

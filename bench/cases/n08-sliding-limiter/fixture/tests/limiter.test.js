import { test } from "node:test";
import assert from "node:assert/strict";
import { createLimiter } from "../src/limiter.js";

function clock(start = 1000) {
  let t = start;
  return { now: () => t, advance: (ms) => (t += ms) };
}

test("allows requests below the limit", () => {
  const c = clock();
  const l = createLimiter({ limit: 3, windowMs: 1000, now: c.now });
  assert.equal(l.allow("user:alice"), true);
  assert.equal(l.allow("user:alice"), true);
  assert.equal(l.remaining("user:alice"), 1);
});

test("identifiers have independent budgets", () => {
  const c = clock();
  const l = createLimiter({ limit: 1, windowMs: 1000, now: c.now });
  assert.equal(l.allow("user:alice"), true);
  assert.equal(l.allow("user:bob"), true);
});

test("the budget returns once the window slides", () => {
  const c = clock();
  const l = createLimiter({ limit: 1, windowMs: 1000, now: c.now });
  l.allow("user:alice");
  c.advance(1500);
  assert.equal(l.remaining("user:alice"), 1);
  assert.equal(l.allow("user:alice"), true);
});

test("user identifiers are case-insensitive", () => {
  const c = clock();
  const l = createLimiter({ limit: 2, windowMs: 1000, now: c.now });
  l.allow("user:Alice");
  l.allow("user:alice");
  assert.equal(l.remaining("user:ALICE"), 0);
});

test("rejects a malformed identifier", () => {
  const l = createLimiter({ limit: 1, windowMs: 1000 });
  assert.throws(() => l.allow("alice"), TypeError);
});

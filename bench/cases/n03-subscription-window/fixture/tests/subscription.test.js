import { test } from "node:test";
import assert from "node:assert/strict";
import { isActive, nextRenewal } from "../src/subscription.js";

const sub = { start: "2026-03-01", end: "2026-03-31" };

test("active in the middle of the window", () => {
  assert.equal(isActive(sub, new Date("2026-03-15")), true);
});

test("inactive before the window", () => {
  assert.equal(isActive(sub, new Date("2026-02-10")), false);
});

test("inactive after the window", () => {
  assert.equal(isActive(sub, new Date("2026-04-20")), false);
});

test("accepts a date string", () => {
  assert.equal(isActive(sub, "2026-03-20"), true);
});

test("renews one year later", () => {
  assert.equal(nextRenewal("2026-05-10"), "2027-05-10");
});

test("rejects an invalid date", () => {
  assert.throws(() => isActive(sub, "not a date"), TypeError);
});

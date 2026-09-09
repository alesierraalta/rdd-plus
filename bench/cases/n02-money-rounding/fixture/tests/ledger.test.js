import { test } from "node:test";
import assert from "node:assert/strict";
import { createLedger, formatCents } from "../src/ledger.js";

test("posting whole amounts accumulates the balance", () => {
  const l = createLedger();
  l.post("a", "10");
  l.post("b", "25");
  assert.equal(l.balance(), 35);
});

test("posting two-decimal amounts", () => {
  const l = createLedger();
  l.post("a", "12.50");
  l.post("b", "7.25");
  assert.equal(l.balance(), 19.75);
});

test("the same entry id is posted once", () => {
  const l = createLedger();
  l.post("a", "5");
  l.post("a", "5");
  assert.equal(l.balance(), 5);
});

test("formats with two decimals", () => {
  assert.equal(formatCents("2.5"), "2.50");
  assert.equal(formatCents(19.75), "19.75");
});

test("rounds half up", () => {
  assert.equal(formatCents("10.125"), "10.13");
});

test("rejects non-numeric input", () => {
  assert.throws(() => formatCents("abc"), TypeError);
});

import { test } from "node:test";
import assert from "node:assert/strict";
import { applyPercentDiscount, qualifiesForFreeShipping } from "../src/pricing.js";

test("applies a flat percent discount", () => {
  assert.equal(applyPercentDiscount(1000, 10), 900);
});

test("rounds the discounted price to the nearest cent", () => {
  assert.equal(applyPercentDiscount(333, 10), 299);
});

test("leaves a 0% discount unchanged", () => {
  assert.equal(applyPercentDiscount(500, 0), 500);
});

test("free shipping kicks in above the threshold", () => {
  assert.equal(qualifiesForFreeShipping(6000, 5000), true);
});

test("orders well under the threshold do not ship free", () => {
  assert.equal(qualifiesForFreeShipping(1000, 5000), false);
});

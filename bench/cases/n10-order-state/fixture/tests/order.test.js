import { test } from "node:test";
import assert from "node:assert/strict";
import { transition, InvalidTransition } from "../src/order.js";

test("the normal path", () => {
  let s = "created";
  s = transition(s, "pay");
  s = transition(s, "ship");
  s = transition(s, "deliver");
  assert.equal(s, "delivered");
});

test("a paid order can be cancelled", () => {
  assert.equal(transition("paid", "cancel"), "cancelled");
});

test("a created order can be cancelled", () => {
  assert.equal(transition("created", "cancel"), "cancelled");
});

test("an unpaid order cannot ship", () => {
  assert.throws(() => transition("created", "ship"), InvalidTransition);
});

test("a delivered order can be returned", () => {
  assert.equal(transition("delivered", "return"), "returned");
});

test("unknown state is rejected", () => {
  assert.throws(() => transition("lost", "pay"), RangeError);
});

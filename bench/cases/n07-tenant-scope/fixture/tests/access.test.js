import { test } from "node:test";
import assert from "node:assert/strict";
import { canAccess, isAdmin } from "../src/access.js";

test("a user reaches resources of their own tenant", () => {
  assert.equal(canAccess({ tenant: "acme", roles: ["viewer"] }, { tenant: "acme" }), true);
});

test("a user does not reach another tenant", () => {
  assert.equal(canAccess({ tenant: "acme", roles: ["viewer"] }, { tenant: "globex" }), false);
});

test("an admin reaches any tenant", () => {
  assert.equal(canAccess({ tenant: "acme", roles: ["admin"] }, { tenant: "globex" }), true);
});

test("legacy roles as a comma-separated string are accepted", () => {
  assert.equal(isAdmin({ roles: "viewer,admin" }), true);
  assert.equal(isAdmin({ roles: "viewer" }), false);
});

test("a viewer is not an admin", () => {
  assert.equal(isAdmin({ roles: ["viewer", "editor"] }), false);
});

test("missing user or resource is denied", () => {
  assert.equal(canAccess(null, { tenant: "acme" }), false);
  assert.equal(canAccess({ tenant: "acme", roles: [] }, null), false);
});

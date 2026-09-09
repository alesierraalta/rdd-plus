import { test } from "node:test";
import assert from "node:assert/strict";
import { fetchWithRetry } from "../src/retry.js";

const noSleep = async () => {};

function sequence(responses) {
  let i = 0;
  return async () => responses[Math.min(i++, responses.length - 1)];
}

test("returns the body on first success", async () => {
  const fetchFn = sequence([{ status: 200, headers: {}, body: { id: 1 } }]);
  assert.deepEqual(await fetchWithRetry(fetchFn, "/x", { sleep: noSleep }), { ok: true, data: { id: 1 } });
});

test("retries a 503 and returns the eventual success", async () => {
  const fetchFn = sequence([{ status: 503, headers: {} }, { status: 200, headers: {}, body: "ok" }]);
  assert.deepEqual(await fetchWithRetry(fetchFn, "/x", { sleep: noSleep }), { ok: true, data: "ok" });
});

test("backs off exponentially between 5xx attempts", async () => {
  const waits = [];
  const fetchFn = sequence([{ status: 503, headers: {} }, { status: 503, headers: {} }, { status: 200, headers: {}, body: 1 }]);
  await fetchWithRetry(fetchFn, "/x", { baseDelayMs: 100, sleep: async (ms) => waits.push(ms) });
  assert.deepEqual(waits, [100, 200]);
});

test("does not retry a client error", async () => {
  let calls = 0;
  const fetchFn = async () => (calls++, { status: 404, headers: {} });
  assert.deepEqual(await fetchWithRetry(fetchFn, "/x", { sleep: noSleep }), { ok: false, error: "http 404" });
  assert.equal(calls, 1);
});

test("honors Retry-After on 429 before succeeding", async () => {
  const waits = [];
  const fetchFn = sequence([{ status: 429, headers: { "retry-after": "2" } }, { status: 200, headers: {}, body: "later" }]);
  const result = await fetchWithRetry(fetchFn, "/x", { sleep: async (ms) => waits.push(ms) });
  assert.deepEqual(result, { ok: true, data: "later" });
  assert.deepEqual(waits, [2000]);
});

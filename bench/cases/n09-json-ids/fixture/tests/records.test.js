import { test } from "node:test";
import assert from "node:assert/strict";
import { parseRecords, dedupeById } from "../src/records.js";

test("parses the records of a batch", () => {
  const records = parseRecords('{"records":[{"id":1,"name":"a"},{"id":2,"name":"b"}]}');
  assert.equal(records.length, 2);
  assert.equal(records[1].name, "b");
});

test("keeps the first record of a repeated id", () => {
  const out = dedupeById([{ id: 7, v: "first" }, { id: 8, v: "x" }, { id: 7, v: "second" }]);
  assert.deepEqual(out.map((r) => r.v), ["first", "x"]);
});

test("distinct ids are all kept", () => {
  const out = dedupeById([{ id: 100 }, { id: 101 }, { id: 102 }]);
  assert.equal(out.length, 3);
});

test("rejects a batch without records", () => {
  assert.throws(() => parseRecords('{"items":[]}'), TypeError);
});

test("rejects a record without id", () => {
  assert.throws(() => dedupeById([{ name: "x" }]), TypeError);
});

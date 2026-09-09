import { test } from "node:test";
import assert from "node:assert/strict";
import { renderLine, parseLine } from "../src/csv.js";

test("renders plain fields joined by commas", () => {
  assert.equal(renderLine(["a", "b", "c"]), "a,b,c");
});

test("quotes a field that contains the separator", () => {
  assert.equal(renderLine(["a", "b,c"]), 'a,"b,c"');
});

test("parses plain fields", () => {
  assert.deepEqual(parseLine("a,b,c"), ["a", "b", "c"]);
});

test("removes the quotes of a quoted field", () => {
  assert.deepEqual(parseLine('"x",y'), ["x", "y"]);
});

test("a doubled quote inside a quoted field is a literal quote", () => {
  assert.deepEqual(parseLine('"a""b",c'), ['a"b', "c"]);
});

test("plain fields round-trip", () => {
  const fields = ["id", "name", "42"];
  assert.deepEqual(parseLine(renderLine(fields)), fields);
});

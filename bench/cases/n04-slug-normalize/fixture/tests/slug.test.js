import { test } from "node:test";
import assert from "node:assert/strict";
import { slugify, dedupeTitles } from "../src/slug.js";

test("lowercases and joins words with dashes", () => {
  assert.equal(slugify("Hello World"), "hello-world");
});

test("punctuation acts as a separator", () => {
  assert.equal(slugify("Node.js 24: what's new"), "node-js-24-what-s-new");
});

test("no dash at the end", () => {
  assert.equal(slugify("Trailing!"), "trailing");
});

test("digits are kept", () => {
  assert.equal(slugify("Top 10"), "top-10");
});

test("dedupes titles with the same slug", () => {
  assert.deepEqual(dedupeTitles(["Hello World", "hello world!", "Other"]), ["Hello World", "Other"]);
});

import { test } from "node:test";
import assert from "node:assert/strict";
import { escapeHtml, renderAttr, renderTitle } from "../src/escape.js";

test("escapes tags", () => {
  assert.equal(escapeHtml("<b>bold</b>"), "&lt;b&gt;bold&lt;/b&gt;");
});

test("escapes ampersands", () => {
  assert.equal(escapeHtml("a & b"), "a &amp; b");
});

test("escapes double quotes in attributes", () => {
  assert.equal(renderAttr("alt", 'say "hi"'), 'alt="say &quot;hi&quot;"');
});

test("renders a title attribute", () => {
  assert.equal(renderTitle("Home"), "title='Home'");
});

test("leaves plain text unchanged", () => {
  assert.equal(escapeHtml("plain text 123"), "plain text 123");
});

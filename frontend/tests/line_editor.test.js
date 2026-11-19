import test from "node:test";
import assert from "node:assert/strict";
import { insertAt, backspaceAt, deleteAt, moveCursor } from "../src/line_editor.js";

test("left/right arrows clamp to beginning and end", () => {
  assert.equal(moveCursor("repository sync", 0, -1), 0);
  assert.equal(moveCursor("repository sync", 0, 1), 1);
  assert.equal(moveCursor("repository sync", 2, 999), 15);
});

test("insert at caret rather than at the end", () => {
  assert.deepEqual(insertAt("show reos", 7, "p"), { line: "show repos", cursor: 8 });
  assert.deepEqual(insertAt("abcd", 2, "XY"), { line: "abXYcd", cursor: 4 });
});

test("backspace removes before caret, delete removes after caret", () => {
  assert.deepEqual(backspaceAt("abc", 2), { line: "ac", cursor: 1 });
  assert.deepEqual(deleteAt("abc", 1), { line: "ac", cursor: 1 });
  assert.deepEqual(backspaceAt("abc", 0), { line: "abc", cursor: 0 });
  assert.deepEqual(deleteAt("abc", 3), { line: "abc", cursor: 3 });
});

test("unicode input is handled as code points", () => {
  assert.deepEqual(insertAt("a🌸c", 2, "b"), { line: "a🌸bc", cursor: 3 });
  assert.deepEqual(backspaceAt("a🌸c", 2), { line: "ac", cursor: 1 });
});

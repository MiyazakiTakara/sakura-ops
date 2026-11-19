import test from "node:test";
import assert from "node:assert/strict";
import { commonPrefix } from "../src/completion.js";

test("Tab completes 'repo' to common 'repository '", () => {
  assert.equal(commonPrefix(["repository add", "repository delete"]), "repository ");
});

test("completion for a unique command returns full command", () => {
  assert.equal(commonPrefix(["show repositories"]), "show repositories");
});

test("no matches yield empty prefix", () => {
  assert.equal(commonPrefix([]), "");
});

test("unrelated matches share no prefix", () => {
  assert.equal(commonPrefix(["help", "show"]), "");
});

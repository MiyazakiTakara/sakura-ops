// Pure command-line cursor operations, independent of xterm rendering.
export function insertAt(line, cursor, input) {
  const chars = Array.from(line);
  const fragment = Array.from(input);
  return { line: [...chars.slice(0, cursor), ...fragment, ...chars.slice(cursor)].join(""),
    cursor: cursor + fragment.length };
}

export function backspaceAt(line, cursor) {
  const chars = Array.from(line);
  if (cursor <= 0) return { line, cursor: 0 };
  chars.splice(cursor - 1, 1);
  return { line: chars.join(""), cursor: cursor - 1 };
}

export function deleteAt(line, cursor) {
  const chars = Array.from(line);
  if (cursor >= chars.length) return { line, cursor };
  chars.splice(cursor, 1);
  return { line: chars.join(""), cursor };
}

export function moveCursor(line, cursor, delta) {
  return Math.max(0, Math.min(Array.from(line).length, cursor + delta));
}

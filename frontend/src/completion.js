export function commonPrefix(strings) {
  if (!strings.length) return "";
  let prefix = strings[0];
  for (const item of strings.slice(1)) {
    while (!item.startsWith(prefix)) prefix = prefix.slice(0, -1);
  }
  return prefix;
}

import { build } from "esbuild";
import { mkdir, copyFile } from "node:fs/promises";
const destination = "../backend/app/static/generated";
await mkdir(destination, { recursive: true });
await build({
  entryPoints: ["src/terminal.js"],
  bundle: true,
  format: "iife",
  outfile: destination + "/terminal.js",
  target: ["es2020"],
  minify: false,
});
await copyFile("node_modules/@xterm/xterm/css/xterm.css", destination + "/terminal.css");
console.log("SakuraOps browser terminal built in", destination);

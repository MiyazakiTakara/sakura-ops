import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";

const win = document.getElementById("window");
const titlebar = document.getElementById("titlebar");
const mount = document.getElementById("xterm");
const csrf = document.querySelector('meta[name="sakuraops-csrf"]')?.content || "";
document.getElementById("logout-csrf").value = csrf;
let history = [];
let historyIndex = 0;
let line = "";
let prompt = "SakuraOps# ";
let busy = false;
let drag = null;
let maximized = false;
let editorOpen = false;

const terminal = new Terminal({
  cursorBlink: true, convertEol: true,
  fontFamily: 'Consolas, "Cascadia Mono", "DejaVu Sans Mono", monospace',
  fontSize: 13, scrollback: 2000,
  theme: { background: "#101820", foreground: "#E6E9EE", cursor: "#D9A0B7",
    green: "#98CFA8", selectionBackground: "#526274" },
});
const fitAddon = new FitAddon();
terminal.loadAddon(fitAddon);
terminal.open(mount);
terminal.writeln("\x1b[35mSakuraOps 0.2.0-dev\x1b[0m");
terminal.writeln("Restricted administrative console. Wpisz ? lub help.");
terminal.write("\r\n\x1b[32m" + prompt + "\x1b[0m");

const narrow = () => matchMedia("(max-width:600px)").matches;
function refit() { if (!win.hidden && !editorOpen) { fitAddon.fit(); terminal.focus(); } }
function openTerminal() { win.hidden = false; requestAnimationFrame(refit); }
function hideTerminal() { if (editorOpen && !closeEditor()) return; win.hidden = true; document.getElementById("open-terminal").focus(); }
document.getElementById("open-terminal").addEventListener("click", openTerminal);
document.getElementById("minimize").addEventListener("click", hideTerminal);
document.getElementById("close").addEventListener("click", hideTerminal);
document.getElementById("maximize").addEventListener("click", () => {
  maximized = !maximized;
  win.classList.toggle("max", maximized);
  const btn = document.getElementById("maximize");
  btn.textContent = maximized ? "❐" : "□";
  btn.setAttribute("aria-label", maximized ? "Przywróć" : "Maksymalizuj");
  bound();
  requestAnimationFrame(refit);
});
function bound() {
  if (narrow() || maximized) return;
  win.style.left = Math.max(0, Math.min(innerWidth - 90, win.offsetLeft)) + "px";
  win.style.top = Math.max(70, Math.min(innerHeight - 42, win.offsetTop)) + "px";
  if (win.offsetWidth > innerWidth) win.style.width = innerWidth + "px";
  if (win.offsetHeight > innerHeight - 70) win.style.height = Math.max(210, innerHeight - 75) + "px";
}
titlebar.addEventListener("pointerdown", event => {
  if (narrow() || maximized || event.button !== 0 || event.target.closest("button")) return;
  drag = { x: event.clientX, y: event.clientY, left: win.offsetLeft, top: win.offsetTop };
  titlebar.setPointerCapture(event.pointerId);
  titlebar.classList.add("dragging");
});
titlebar.addEventListener("pointermove", event => {
  if (!drag) return;
  win.style.left = Math.max(0, Math.min(innerWidth - 90, drag.left + event.clientX - drag.x)) + "px";
  win.style.top = Math.max(70, Math.min(innerHeight - 42, drag.top + event.clientY - drag.y)) + "px";
});
function endDrag() { drag = null; titlebar.classList.remove("dragging"); }
titlebar.addEventListener("pointerup", endDrag);
titlebar.addEventListener("pointercancel", endDrag);
titlebar.addEventListener("keydown", event => {
  if (narrow() || maximized || !event.altKey) return;
  const amount = event.ctrlKey ? 1 : 12;
  const delta = { ArrowLeft: [-amount, 0], ArrowRight: [amount, 0],
    ArrowUp: [0, -amount], ArrowDown: [0, amount] }[event.key];
  if (!delta) return;
  event.preventDefault();
  if (event.shiftKey) {
    win.style.width = Math.max(330, win.offsetWidth + delta[0]) + "px";
    win.style.height = Math.max(210, win.offsetHeight + delta[1]) + "px";
  } else {
    win.style.left = win.offsetLeft + delta[0] + "px";
    win.style.top = win.offsetTop + delta[1] + "px";
  }
  bound(); refit();
});
window.addEventListener("resize", () => { bound(); refit(); });
const resizeObserver = new ResizeObserver(() => { if (!win.hidden && !editorOpen) fitAddon.fit(); });
resizeObserver.observe(win);

function writeOutput(value) {
  if (value) for (const row of String(value).split("\n")) terminal.writeln(row.replace(/[\x00-\x1f\x7f-\x9f]/g, ""));
}
function drawPrompt() { terminal.write("\x1b[32m" + prompt + "\x1b[0m" + line); }
function replaceLine(value) {
  terminal.write("\r\x1b[2K");
  line = value;
  drawPrompt();
}
async function request(url, options = {}) {
  const response = await fetch(url, { credentials: "same-origin", ...options,
    headers: { "X-CSRF-Token": csrf, ...(options.headers || {}) } });
  if (response.status === 401) throw new Error("Sesja wygasła. Zaloguj się ponownie.");
  const data = await response.json();
  if (!response.ok) throw new Error(typeof data.detail === "string" ? data.detail : "Błąd API");
  return data;
}
async function runCommand() {
  const command = line.trim();
  terminal.write("\r\n");
  line = "";
  if (!command) { drawPrompt(); return; }
  history.push(command); historyIndex = history.length;
  busy = true;
  try {
    if (command === "history") {
      writeOutput(history.map((entry, i) => (i + 1) + "  " + entry).join("\n"));
    } else {
      const data = await request("/api/console", {
        method: "POST", headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command }) });
      prompt = data.prompt || "SakuraOps# ";
      if (data.action === "clear") terminal.clear();
      else if (data.action === "history") writeOutput(history.map((entry, i) => (i + 1) + "  " + entry).join("\n"));
      else if (data.action === "files") {
        const files = await request("/api/workspace");
        writeOutput(files.files.length ? files.files.join("\n") : "Brak plików roboczych.");
      } else if (data.action === "read") {
        const file = await request("/api/workspace/" + encodeURIComponent(data.filename));
        writeOutput(file.content || "Plik nie istnieje albo jest pusty.");
      } else if (data.action === "edit") {
        await openEditor(data.filename);
      } else writeOutput(data.output);
    }
  } catch (error) {
    writeOutput(error.message || "Błąd połączenia");
  } finally {
    busy = false;
    if (!editorOpen) drawPrompt();
  }
}
async function autocomplete() {
  try {
    const data = await request("/api/console/complete", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prefix: line }) });
    const choices = data.matches;
    if (choices.length === 1) replaceLine(choices[0] + " ");
    else if (choices.length > 1) {
      terminal.write("\r\n"); writeOutput(choices.join("    ")); drawPrompt();
    }
  } catch (error) { terminal.write("\r\n"); writeOutput(error.message); drawPrompt(); }
}
terminal.onData(async data => {
  if (busy || editorOpen || win.hidden) return;
  if (data === "\r") { await runCommand(); return; }
  if (data === "\u007f" || data === "\b") {
    if (line.length) { line = line.slice(0, -1); terminal.write("\b \b"); }
    return;
  }
  if (data === "\u000c") { terminal.clear(); drawPrompt(); return; }
  if (data === "\u0003") { terminal.write("^C\r\n"); line = ""; drawPrompt(); return; }
  if (data === "\t") { busy = true; await autocomplete(); busy = false; return; }
  if (data === "\x1b[A") { if (historyIndex > 0) replaceLine(history[--historyIndex]); return; }
  if (data === "\x1b[B") { if (historyIndex < history.length) historyIndex++; replaceLine(history[historyIndex] || ""); return; }
  if (data === "?") {
    {
      terminal.write("\r\n");
      try {
        const result = await request("/api/console/complete", {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ prefix: line }) });
        writeOutput(result.matches.join("\n") || "Brak poleceń.");
      } catch(error) { writeOutput(error.message); }
      drawPrompt(); return;
    }
  }
  // Paste/input is never interpreted as ANSI or system commands.
  const clean = data.replace(/[\x00-\x1f\x7f-\x9f\x1b]/g, "").replace(/[\r\n]/g, "");
  if (clean && line.length + clean.length <= 500) { line += clean; terminal.write(clean); }
});

const editor = document.getElementById("editor-pane");
const editorText = document.getElementById("editor-content");
const editorTitle = document.getElementById("editor-filename");
const editorStatus = document.getElementById("editor-status");
let editingName = null;
let savedValue = "";
async function openEditor(filename) {
  const file = await request("/api/workspace/" + encodeURIComponent(filename));
  editingName = filename; savedValue = file.content;
  editorTitle.textContent = filename; editorText.value = file.content;
  editorStatus.textContent = "Plik roboczy, UTF-8, max 64 KiB. Ctrl+S zapisuje.";
  editor.hidden = false; editorOpen = true; editorText.focus();
}
function closeEditor() {
  if (editorText.value !== savedValue && !confirm("Odrzucić niezapisane zmiany?")) return false;
  editor.hidden = true; editorOpen = false; editingName = null;
  drawPrompt(); requestAnimationFrame(refit);
  return true;
}
document.getElementById("editor-close").addEventListener("click", closeEditor);
async function saveEditor() {
  if (!editingName) return;
  editorStatus.textContent = "Zapisywanie...";
  try {
    await request("/api/workspace/" + encodeURIComponent(editingName), {
      method: "PUT", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content: editorText.value }) });
    savedValue = editorText.value; editorStatus.textContent = "Zapisano " + editingName;
  } catch(error) { editorStatus.textContent = error.message; }
}
document.getElementById("editor-save").addEventListener("click", saveEditor);
editorText.addEventListener("keydown", event => {
  if (event.ctrlKey && event.key.toLowerCase() === "s") { event.preventDefault(); saveEditor(); }
  if (event.key === "Escape") { event.preventDefault(); closeEditor(); }
  if (event.key === "Tab") {
    event.preventDefault();
    const start = editorText.selectionStart, end = editorText.selectionEnd;
    editorText.setRangeText("  ", start, end, "end");
  }
});
document.getElementById("editor-open").addEventListener("click", () => {
  terminal.write("\r\n"); writeOutput("Otwórz edytor: edit notatki.md lub nano inventory.ini"); drawPrompt();
});
fetch("/api/me", {credentials:"same-origin"}).then(r => r.ok ? r.json() : null)
  .then(data => { if(data?.username) document.getElementById("username").textContent = data.username; }).catch(()=>{});
if (new URLSearchParams(location.search).get("terminal") === "1") {
  openTerminal(); window.history.replaceState(null, "", "/");
}

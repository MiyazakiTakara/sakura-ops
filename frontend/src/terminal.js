import { commonPrefix } from "./completion.js";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { basicSetup } from "codemirror";
import { EditorState, Compartment } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection } from "@codemirror/view";
import { syntaxHighlighting, defaultHighlightStyle } from "@codemirror/language";
import { yaml } from "@codemirror/lang-yaml";
import { markdown } from "@codemirror/lang-markdown";

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
      } else {
        writeOutput(data.output);
        if (/^(?:skr\\s+)?repository\\s+(?:add|delete|sync)\\b/i.test(command) && !repositoryView.hidden)
          void refreshRepositories();
      }
    }
  } catch (error) {
    writeOutput(error.message || "Błąd połączenia");
  } finally {
    busy = false;
    if (!editorOpen) drawPrompt();
  }
}
let lastCompletionList = null;
async function autocomplete() {
  try {
    const data = await request("/api/console/complete", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prefix: line }) });
    const choices = data.matches;
    if (!choices.length) return;
    const shared = commonPrefix(choices);
    if (shared.length > line.length) {
      replaceLine(shared + (choices.length === 1 && !shared.endsWith(" ") ? " " : ""));
      lastCompletionList = null;
    } else if (lastCompletionList !== line) {
      terminal.write("\r\n");
      writeOutput(choices.join("    "));
      drawPrompt();
      lastCompletionList = line;
    }
  } catch (error) {
    terminal.write("\r\n");
    writeOutput(error.message);
    drawPrompt();
  }
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
  lastCompletionList = null;
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
const editorMount = document.getElementById("editor-content");
const editorTitle = document.getElementById("editor-filename");
const editorStatus = document.getElementById("editor-status");
let editingName = null;
let savedValue = "";
let editorView = null;
const editorTheme = EditorView.theme({
  "&": { height: "100%", backgroundColor: "#121A21", color: "#E6E9EE", fontSize: "13px" },
  ".cm-scroller": { fontFamily: "Consolas, monospace", lineHeight: "1.65" },
  ".cm-gutters": { backgroundColor: "#202731", color: "#8F9CA9", border: "none" },
  ".cm-activeLine": { backgroundColor: "#26303A" },
  ".cm-activeLineGutter": { backgroundColor: "#303B46" },
  ".cm-content": { caretColor: "#D9A0B7" },
  ".cm-cursor": { borderLeftColor: "#D9A0B7" },
  ".cm-selectionBackground": { backgroundColor: "#526274 !important" },
}, { dark: true });
function currentEditorText() { return editorView?.state.doc.toString() ?? ""; }
function editorMode(filename) {
  const ext = filename.split(".").pop().toLowerCase();
  if (ext === "yml" || ext === "yaml") return [yaml()];
  if (ext === "md") return [markdown()];
  return [];
}
async function openEditor(filename) {
  const file = await request("/api/workspace/" + encodeURIComponent(filename));
  editingName = filename;
  savedValue = file.content;
  editorTitle.textContent = filename;
  editorStatus.textContent = "UTF-8 · do 64 KiB · Ctrl+S zapis · Esc zamyka";
  editor.hidden = false;
  editorOpen = true;
  editorView?.destroy();
  editorView = new EditorView({
    state: EditorState.create({
      doc: file.content,
      extensions: [
        basicSetup, editorMode(filename), syntaxHighlighting(defaultHighlightStyle),
        editorTheme,
        keymap.of([
          { key: "Mod-s", run: () => { void saveEditor(); return true; } },
          { key: "Escape", run: () => { closeEditor(); return true; } },
        ]),
      ],
    }),
    parent: editorMount,
  });
  editorView.focus();
}
function closeEditor() {
  if (currentEditorText() !== savedValue && !confirm("Odrzucić niezapisane zmiany?")) return false;
  editorView?.destroy();
  editorView = null;
  editor.hidden = true;
  editorOpen = false;
  editingName = null;
  drawPrompt();
  requestAnimationFrame(refit);
  return true;
}
document.getElementById("editor-close").addEventListener("click", closeEditor);
async function saveEditor() {
  if (!editingName) return;
  const content = currentEditorText();
  editorStatus.textContent = "Zapisywanie...";
  try {
    await request("/api/workspace/" + encodeURIComponent(editingName), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content }),
    });
    savedValue = content;
    editorStatus.textContent = "Zapisano " + editingName;
  } catch (error) {
    editorStatus.textContent = error.message || "Nie udało się zapisać";
  }
}
document.getElementById("editor-save").addEventListener("click", saveEditor);
document.getElementById("editor-open").addEventListener("click", () => {
  terminal.write("\r\n");
  writeOutput("Edytor: edit notatki.md lub nano inventory.ini");
  drawPrompt();
});
// Repository configuration is shared with the restricted CLI via the backend API.
const dashboardView = document.getElementById("dashboard-view");
const repositoryView = document.getElementById("repository-view");
const navDashboard = document.getElementById("nav-dashboard");
const navRepositories = document.getElementById("nav-repositories");
const repoForm = document.getElementById("repo-form");
const repoList = document.getElementById("repo-list");
const repoStatus = document.getElementById("repo-status");
let repoLoading = false;
let repoRefreshQueued = false;
let repoSnapshot = null;
const FALLBACK_REFRESH_MS = 30000;
let repoEvents = null;
let repoEventsFallback = false;

function updateRepositoriesWhenVisible() {
  if (!repositoryView.hidden && document.visibilityState === "visible") {
    void refreshRepositories();
  }
}

function connectRepositoryEvents() {
  if (repoEvents || typeof EventSource === "undefined") return;
  repoEvents = new EventSource("/api/repositories/events");
  repoEvents.addEventListener("repositories-changed", updateRepositoriesWhenVisible);
  repoEvents.addEventListener("open", () => { repoEventsFallback = false; });
  repoEvents.addEventListener("error", () => { repoEventsFallback = true; });
}
function disconnectRepositoryEvents() {
  if (repoEvents) {
    repoEvents.close();
    repoEvents = null;
  }
  repoEventsFallback = false;
}
// Fallback only when SSE is unavailable or disconnected, and only on a visible view.
window.setInterval(() => {
  if (repoEventsFallback || typeof EventSource === "undefined") {
    updateRepositoriesWhenVisible();
  }
}, FALLBACK_REFRESH_MS);
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    if (!repositoryView.hidden) connectRepositoryEvents();
    updateRepositoriesWhenVisible();
  } else {
    disconnectRepositoryEvents();
  }
});
window.addEventListener("focus", updateRepositoriesWhenVisible);

function setView(view) {
  const showRepo = view === "repositories";
  dashboardView.hidden = showRepo;
  repositoryView.hidden = !showRepo;
  if (showRepo) {
    navRepositories.setAttribute("aria-current", "page");
    navDashboard.removeAttribute("aria-current");
    void refreshRepositories();
    if (document.visibilityState === "visible") connectRepositoryEvents();
  } else {
    disconnectRepositoryEvents();
    navDashboard.setAttribute("aria-current", "page");
    navRepositories.removeAttribute("aria-current");
  }
}
navDashboard.addEventListener("click", () => setView("dashboard"));
navRepositories.addEventListener("click", () => setView("repositories"));

function repoText(tag, text, className = "") {
  const element = document.createElement(tag);
  element.textContent = text;
  if (className) element.className = className;
  return element;
}
async function refreshRepositories() {
  if (repoLoading) {
    repoRefreshQueued = true;
    return;
  }
  repoLoading = true;
  try {
    const payload = await request("/api/repositories");
    const snapshot = JSON.stringify(payload.repositories);
    if (snapshot === repoSnapshot) return;
    repoSnapshot = snapshot;
    repoList.replaceChildren();
    if (!payload.repositories.length) {
      repoList.append(repoText("p", "Nie dodano jeszcze repozytoriów.", "repo-note"));
      return;
    }
    for (const repo of payload.repositories) {
      const item = document.createElement("div");
      item.className = "repo-row";
      const info = repoText("div", "", "repo-details");
      const title = repoText("strong", repo.name + " · " + repo.branch);
      const url = repoText("span", repo.url, "repo-url");
      const caption = repoText("span",
        repo.sync_status === "synced"
          ? "Ostatnio pobrano: " + repo.synced_at + " · " + repo.commit_sha.slice(0, 12)
          : "Nie synchronizowano · wymagany SSH deploy key",
        "repo-note");
      info.append(title, url, caption);
      const del = repoText("button", "Usuń", "repo-delete");
      del.type = "button";
      del.setAttribute("aria-label", "Usuń konfigurację " + repo.name);
      del.addEventListener("click", async () => {
        if (!confirm("Usunąć konfigurację repozytorium " + repo.name + "?")) return;
        del.disabled = true;
        try {
          await request("/api/repositories/" + encodeURIComponent(repo.name), { method: "DELETE" });
          repoStatus.textContent = "Usunięto konfigurację " + repo.name + ".";
          await refreshRepositories();
        } catch (error) {
          repoStatus.textContent = error.message || "Nie udało się usunąć.";
          del.disabled = false;
        }
      });
      const actions = document.createElement("div");
      actions.className = "repo-actions";
      const sync = repoText("button", "Synchronizuj", "repo-sync");
      sync.type = "button";
      sync.setAttribute("aria-label", "Synchronizuj " + repo.name);
      sync.addEventListener("click", async () => {
        sync.disabled = true;
        repoStatus.textContent = "Synchronizacja repozytorium " + repo.name + "...";
        try {
          await request("/api/repositories/" + encodeURIComponent(repo.name) + "/sync", { method: "POST" });
          repoStatus.textContent = "Pobrano repozytorium " + repo.name + ".";
          repoSnapshot = null;
          await refreshRepositories();
        } catch (error) {
          repoStatus.textContent = error.message || "Synchronizacja nie powiodła się.";
        } finally { sync.disabled = false; }
      });
      const showFiles = repoText("button", "Pliki", "repo-sync");
      showFiles.type = "button";
      showFiles.setAttribute("aria-label", "Pokaż inventory " + repo.name);
      showFiles.addEventListener("click", async () => {
        repoStatus.textContent = "Odczytywanie inventory...";
        try {
          const base = "/api/repositories/" + encodeURIComponent(repo.name) + "/files?kind=inventory";
          const payload = await request(base);
          repoStatus.textContent = payload.files.length
            ? "Inventory " + repo.name + ": " + payload.files.join(", ")
            : "Brak plików inventory.";
        } catch (error) { repoStatus.textContent = error.message || "Odczyt nie powiódł się."; }
      });
      actions.append(sync, showFiles, del);
      item.append(info, actions);
      repoList.append(item);
    }
  } catch (error) {
    repoSnapshot = null;
    repoList.replaceChildren(repoText("p", error.message || "Nie udało się odczytać repozytoriów.", "repo-note"));
  } finally {
    repoLoading = false;
    if (repoRefreshQueued) {
      repoRefreshQueued = false;
      void refreshRepositories();
    }
  }
}
repoForm.addEventListener("submit", async event => {
  event.preventDefault();
  const submit = document.getElementById("repo-submit");
  submit.disabled = true;
  repoStatus.textContent = "Zapisywanie konfiguracji...";
  try {
    const body = {
      name: document.getElementById("repo-name").value.trim(),
      url: document.getElementById("repo-url").value.trim(),
      branch: document.getElementById("repo-branch").value.trim(),
    };
    const saved = await request("/api/repositories", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    repoStatus.textContent = "Dodano " + saved.name + ". Synchronizacja Git nie jest jeszcze dostępna.";
    repoForm.reset();
    await refreshRepositories();
  } catch (error) {
    repoStatus.textContent = error.message || "Nie udało się dodać repozytorium.";
  } finally {
    submit.disabled = false;
  }
});
fetch("/api/me", {credentials:"same-origin"}).then(r => r.ok ? r.json() : null)
  .then(data => { if(data?.username) document.getElementById("username").textContent = data.username; }).catch(()=>{});
if (new URLSearchParams(location.search).get("terminal") === "1") {
  openTerminal(); window.history.replaceState(null, "", "/");
}

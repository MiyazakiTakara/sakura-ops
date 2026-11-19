# Console and text editor (v0.2 development preview)

## Scope
The floating terminal now uses the locally bundled `@xterm/xterm` emulator, but it is **not a Linux shell or PTY**. User input is parsed by `backend/app/commands.py`. Commands must be explicitly registered there. Unsupported commands, pipes, redirection and system shell commands are rejected.

## Build on WSL

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/floating-terminal-ui
source .venv/bin/activate
cd frontend
npm install
npm run build
cd ..
python -m pytest -q tests/test_ui_shell.py
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload
```

Do not commit `node_modules/` or generated browser bundles; the build places them in `backend/app/static/generated/`.

## Basic commands
- `?`, `help`, `Tab`: contextual assistance/completion.
- `show version`, `show status`, `show servers`, `show inventory`.
- `configure terminal` enters configuration mode. `exit` and `end` change mode.
- `server <name>` in configuration mode currently selects a *demonstration context only*. It does not change inventory.
- `files`: list local SakuraOps workspace files.
- `cat notes.md`: show contents of one workspace file.
- `edit notes.md` or `nano notes.md`: open SakuraOps's built-in text editor.
- `history`, `clear`, `Ctrl+L`, arrow up/down.
- Editor: `Ctrl+S` saves; `Esc` returns to the terminal. The editor is a GUI text area, **not GNU nano or Vim**.

## File editing boundaries
- Files exist only under `data/workspace`. This folder is ignored by Git.
- Single-component filenames only, max 80 characters, and extensions `.txt`, `.md`, `.ini`, `.yml`, `.yaml`.
- No parent traversal, absolute paths, symlinks or arbitrary host filesystem access.
- 64 KiB max file contents. Read operations require login and CSRF; saving requires the admin role.
- This is an initial local editor, not a git playbook editor. Do not put passwords, private keys or production inventory with secrets here.

## Manual acceptance
1. Test local login and logout, then verify direct `/api/console` cannot be used without login.
2. Open Terminal from the left capsule. Drag, resize, minimize/restore and maximize.
3. Execute `?`, `show status`, `configure terminal`, `exit`, `end`, Tab completion and history arrows.
4. Run `edit notes.md`, type text and save with Ctrl+S. Reopen and confirm contents. Use `files` and `cat notes.md`.
5. Verify `edit ../secret.txt` and arbitrary commands do not grant filesystem/shell access.
6. Verify window on narrow viewport.
7. Do not merge PR until build and functional checks pass.

## Future
Native `skr` Linux client and full optional SSO, Ansible integration, Git-backed playbook editor and richer keyboard editor modes are separate tasks.

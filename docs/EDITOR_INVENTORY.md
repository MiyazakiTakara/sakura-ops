# v0.3 preview: Editor and read-only Ansible checkout

This PR builds on the merged floating terminal (PR #7). It does not deploy to production or execute Ansible.

## Editor

The editor reached by `edit notes.md` or `nano notes.md` now uses CodeMirror 6 inside the same floating terminal window. It supports line numbers, active-line highlighting, undo/redo, keyboard shortcuts, Markdown and YAML highlighting. `Ctrl+S` writes through the existing restricted SakuraOps workspace API; `Esc` exits after a prompt for unsaved changes. `nano` remains an alias, not GNU nano.

## Ansible Git checkout: file listing only

`show inventory` and `show playbooks` discover file **names**, without printing contents or executing playbooks, from an explicitly configured local Git checkout. Nothing is cloned automatically.

On the machine running SakuraOps, clone the already-authorized Forgejo repository to a non-web-accessible folder using your normal Git credentials, then configure:

```bash
export SAKURAOPS_ANSIBLE_REPO_PATH="$HOME/ansible-checkout"
```

Restart the app after setting the environment variable. The checkout must have a `.git` entry and subfolders named `inventory/` or `inventories/`, and/or `playbooks/` or `playbook/`. Only `.ini`, `.yaml`, `.yml` file names are listed, up to 200 per category; no file content or secrets are transmitted by these commands. The feature requires an authenticated admin user.

This is **not yet** a connection to the live Forgejo remote. No fetch/pull, parsing of inventory hosts, write-back, commits, or playbook runs are implemented. It may need adjustments after inspecting the real repository layout.

## Local verification

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/editor-inventory-foundation
source .venv/bin/activate
cd frontend
npm install
npm run build
cd ..
python -m pytest -q tests/test_ui_shell.py tests/test_editor_inventory.py
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload
```

Test `edit demo.yml`, line numbers, YAML highlighting, saving/re-opening and `show inventory` without a checkout (should report not configured).

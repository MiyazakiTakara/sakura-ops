# Manual HTTPS and SSH Git synchronization (development preview)

**No production Ansible actions.** This implementation fetches Git objects into a
**bare** local cache and displays names of inventory/playbook files. It never
checks out a working tree, executes playbooks, applies roles or synchronizes
the Semaphore service.

## Configuration through GUI or `skr`

First add the remote through the **Repozytoria** view, or in SakuraOps terminal. Both HTTPS and SSH are supported:

```text
SakuraOps# configure terminal
SakuraOps(config)# repository add ansible https://git.amuz.wroc.pl/patryk.nowicki/semaphore-ansible main
SakuraOps(config)# end
SakuraOps# show repositories
```

Then, once the host is allowlisted (and credentials are configured if private), synchronize manually:

```text
SakuraOps# repository sync ansible
SakuraOps# show repository ansible inventory
SakuraOps# show repository ansible playbooks
```

The panel offers **Synchronizuj** and **Pliki** buttons for the same operations.
Both use the same authenticated backend functions and database.
`show inventory` (without repo name) retains its older independent local-checkout
behavior for now.

## HTTPS — public and private repositories

HTTPS synchronization uses the regular `https://host/owner/repo` URL stored in
SakuraOps. The host must be explicitly allowed by its service administrator:

```bash
export SAKURAOPS_GIT_ALLOWED_HOSTS=git.amuz.wroc.pl
```

**Public HTTPS Git:** no credentials required.

**Private HTTPS Git:** create a read-only Forgejo access token (repository read
permission only), store it in a file readable exclusively by the SakuraOps
service user, and set the file's path as a service environment variable.
For a repo named `ansible`, the example is:

```bash
export SAKURAOPS_GIT_TOKEN_FILE_ANSIBLE=/secure/path/ansible-git-token
export SAKURAOPS_GIT_HTTPS_USER=oauth2
```

Set mode 0600 on the token file, owner SakuraOps. The username may depend on
the Git provider. The token is passed to Git via a short-lived child-process
environment variable (not a URL, CLI command argument, or log message).
Never paste a token into a `repository add` command or commit it to Git.

HTTPS TLS certificates are verified by Git's normal CA trust; HTTP redirects are
disabled. No insecure TLS option is provided.

## SSH provisioning (one-time administrator task)

This step is intentionally **not exposed in the web terminal or GUI yet**:
the initial implementation never accepts private keys, passphrases or tokens
as command arguments, avoiding leakage to the browser history or audit logs.
The SakuraOps host administrator must provide a dedicated read-only deploy key.

1. Generate a separate SSH keypair for SakuraOps on its own host with mode 0600
   for the private key and restrict its read access to the SakuraOps service account.
   Register only the **public** key as a **read-only deploy key** in the Forgejo
   repository. Never put the private key into GitHub, the terminal, or the GUI.
2. Verify the Forgejo server's SSH host key fingerprint independently against
   a trusted value provided by your Forgejo administrator. Pin that exact key
   in a private `known_hosts` file on the SakuraOps host. Do not use automatic
   `ssh-keyscan` output without independent verification.
3. Set these environment variables in the SakuraOps service environment before
   starting FastAPI:

```bash
export SAKURAOPS_GIT_ALLOWED_HOSTS=git.amuz.wroc.pl
export SAKURAOPS_GIT_SSH_KEY=/path/to/service-owned-deploy-key
export SAKURAOPS_GIT_KNOWN_HOSTS=/path/to/verified-known-hosts
```

Use existing paths on the host. These are **examples, not generated credentials**.
The SSH host allowlist is a defense against arbitrary network destinations.
The private key must have mode 0600 and known_hosts must exist.
SSH accepts only a verified known host, noninteractive authentication and the
configured key. No arbitrary shell commands, submodules or hooks run in SakuraOps.

## Limits and next improvements

- Manual synchronization only. No automatic pull, no write/push, no playbook run.
- HTTPS URLs can synchronize without a token for public repos, or with a separately
  provisioned token file for private repos. SSH continues to use pinned host keys.
- Git objects are cached under ignored `data/git_mirrors/`. Entries are indexed
  by `refs/sakuraops/synced`; last commit and timestamp are in SQLite.
- Synchronization allows one operation per repository, times out after 50 seconds
  per Git subprocess, and suppresses remote Git stderr to avoid leaking details.
- Filenames are shown from committed trees, limited to 200 per category and known
  extensions. **File contents cannot yet be edited or executed.**
- This first integration has no background worker, aggregate storage quota,
  artifact integrity policy, or audit log for privileged changes. Those must be
  addressed before broad deployment.
- The UI should eventually allow a secure workflow to provision/revoke deploy
  keys and approve verified host fingerprints, without displaying private key
  contents. Both GUI and CLI can already configure repos and initiate syncs.

## Acceptance test

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/manual-git-sync
source .venv/bin/activate
cd frontend && npm install && npm run build && cd ..
python -m pytest -q tests/test_repository_settings.py tests/test_ui_shell.py tests/test_editor_inventory.py
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload
```

Without a host allowlist, HTTPS and SSH synchronization must fail safely.
Private HTTPS repositories require a separate token file; SSH repositories need
an approved deploy key. Both should fetch and list files without modifying the
remote or running Ansible.

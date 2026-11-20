# SakuraOps PR #17 — Ansible builtin ping (oddzielna tożsamość)

Ten etap **nie uruchamia playbooków z Forgejo**. Wykonuje dokładnie
\`ansible -i ADRES_IP, all -m ansible.builtin.ping -u sakuraops-ping\`,
na jednym zatwierdzonym IP, w odrębnej kolejce z logami i możliwością zatrzymania.

**Uwaga:** Ansible ping NIE jest ICMP. Wymaga SSH, zdalnego uruchomienia
Pythona i tworzy tymczasowe pliki modułu w katalogu domowym użytkownika.
Nie zmienia konfiguracji usług, ale wymaga uprawnień do wykonywania
programów na hoście — więcej niż dotychczasowy test SSH z wymuszonym \`true\`.

## Rozdzielenie kont/kluczy

Zachowaj istniejące konto i klucz \`sakuraops-diag\` w niezmienionej postaci
z opcją \`restrict,command="/usr/bin/true"\`. **Nie usuwać ani nie rozluźniać
tego ograniczenia.**

Do modułów Ansible użyj osobnego konta \`sakuraops-ping\`, które:
- nie jest w \`sudo\`, nie ma \`NOPASSWD\`, \`become\` ani uprzywilejowanych grup;
- ma zablokowane hasło, logowanie wyłącznie dedykowanym kluczem;
- posiada tylko dostęp do nieuprzywilejowanego wykonania Pythona;
- nie otrzymuje dostępu do danych/kluczy kont administratorów.
- ma wpis \`authorized_keys\` z opcją \`restrict\` (bez \`command=\`,
  bo Ansible musi uruchomić własny kod zdalny).

Nie używaj tego konta/klucza do wykonywania playbooków produkcyjnych.
Jeżeli potrzeba silnej izolacji zdalnego wykonania, użyj odrębnej VM
lub kontenera zamiast współdzielenia serwera Semaphore.

## Konfiguracja laptopa WSL

Przygotuj na laptopie **nowy klucz**, odrębny od
\`~/.ssh/sakuraops_diag_ed25519\`:

\`\`\`bash
ssh-keygen -t ed25519 -f "$HOME/.ssh/sakuraops_ping_ed25519" -N "" \
  -C "sakuraops-ping-laptop"
chmod 600 "$HOME/.ssh/sakuraops_ping_ed25519"
ssh-keygen -lf "$HOME/.ssh/sakuraops_ping_ed25519.pub"
\`\`\`

Dodaj tylko nowy klucz publiczny do
\`/home/sakuraops-ping/.ssh/authorized_keys\` na Semaphore jako:

\`\`\`text
restrict ssh-ed25519 <KLUCZ_PUBLICZNY> sakuraops-ping-laptop
\`\`\`

Zachowaj istniejący wcześniej zweryfikowany \`known_hosts\`.
Fingerprints klucza hosta muszą zgadzać się ze sprawdzonym kluczem
Semaphore. Nie akceptuj kluczy w ciemno.

Uruchamianie w sesji WSL deweloperskiego SakuraOps:

\`\`\`bash
cd ~/sakura-ops
git fetch origin
git switch feat/ansible-ping-diagnostics
source .venv/bin/activate
python -m pip install ansible-core
export SAKURAOPS_SSH_DIAG_TARGETS="semaphore=172.24.3.181"
export SAKURAOPS_ANSIBLE_PING_ENABLED=true
export SAKURAOPS_ANSIBLE_PING_KEY="$HOME/.ssh/sakuraops_ping_ed25519"
export SAKURAOPS_ANSIBLE_PING_KNOWN_HOSTS="$HOME/.ssh/sakuraops_known_hosts"
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 \
  --reload --reload-dir backend/app
\`\`\`

Nowe zmienne \`SAKURAOPS_ANSIBLE_PING_*\` są niezależne od flagi
\`SAKURAOPS_SSH_DIAG_ENABLED\`, ale współdzielą walidowaną listę
aliasów \`SAKURAOPS_SSH_DIAG_TARGETS\`.

W panelu **Zadania → Diagnostyka Ansible — ping** wybierz Semaphore
i potwierdź uruchomienie. Sukces oznacza wyjście 0 i odpowiedź \`pong\`.

## Mechanizmy ochrony

- Flaga jest domyślnie wyłączona; brak konta/klucza/hosta wyłącza UI.
- Admin + sesja + CSRF; SSE wyłącznie dla zalogowanego admina.
- Docelowy host wyłącznie z allowlisty jawnych IP; kontrola ponownie
  przed rozpoczęciem, żadnego inventory z Git i żadnych hostvars.
- Osobny klucz SSH prywatny 0600, osobny \`known_hosts\` i wymagana
  weryfikacja klucza hosta. Zablokowane SSH forwarding, agent, multiplexing.
- Jedyny dozwolony moduł \`ansible.builtin.ping\`, bez parametrów użytkownika
  i bez plików playbooków.
- Dedykowany katalog tymczasowy HOME/ansible.cfg (0600) i izolowane env.
- Jedno wykonanie równocześnie w osobnym workerze, do 10 oczekujących,
  limit 45 sekund, stan persistent w SQLite, anulowanie, log do 16 KiB.

**Ograniczenie:** to nie jest piaskownica ani ochrona przed
kompromitacją użytkownika \`sakuraops-ping\`. Zdalny account ma zdolność
uruchamiania dowolnych procesów w swojej przestrzeni użytkownika, jeśli
klucz zostanie przejęty. Testuj najpierw na Semaphore, bez sudo i poza
wrażliwymi katalogami. Nie skaluj tego mechanizmu na produkcję bez dalszego
etapu izolacji.

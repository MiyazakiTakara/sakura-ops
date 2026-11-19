# SakuraOps — normalny Git, ręczna synchronizacja

SakuraOps korzysta z **systemowego Gita**, jego konfiguracji, standardowych
credential helperów oraz SSH agent/known_hosts użytkownika, pod którym działa
FastAPI. Nie przechowuje własnych tokenów ani kluczy w bazie.

## Konfiguracja

Repozytorium można dodać z panelu Repozytoria lub komendą:

```text
SakuraOps# configure terminal
SakuraOps(config)# repository add ansible https://git.amuz.wroc.pl/patryk.nowicki/semaphore-ansible main
SakuraOps(config)# end
SakuraOps# repository sync ansible
SakuraOps# show repository ansible inventory
SakuraOps# show repository ansible playbooks
```

Ten sam przycisk **Synchronizuj** w GUI wywołuje tę samą funkcję backendu.
Adres HTTPS pozostaje bez zmian — nie trzeba przechodzić na SSH.

## Jak działa `repository sync`

1. Pierwsza synchronizacja: `git clone --single-branch --branch main` do
   `data/git_worktrees/ansible`.
2. Kolejne synchronizacje: sprawdzenie `origin`, bieżącej gałęzi i czystości
   katalogu, następnie `git fetch origin main` oraz `git merge --ff-only FETCH_HEAD`.
3. Jeśli istnieją niezapisane zmiany (w tym untracked files), SakuraOps kończy
   z błędem 409 i **nie nadpisuje plików**. Nie wykonuje `git reset --hard`.
4. `show repository ansible inventory|playbooks` wyświetla nazwy plików
   śledzonych przez Git, bez wykonywania ich zawartości.

Repozytoria są klonowane do wydzielonego katalogu SakuraOps; ten etap nie
ingeruje w istniejący katalog Semaphore. Nie wykonujemy playbooków Ansible.

## HTTPS i SSH — standardowa konfiguracja Git

SakuraOps uruchamia Git w środowisku użytkownika usługi. Dla prywatnego
Forgejo skonfiguruj u tego użytkownika odpowiedni Git credential helper
(HTTPS z tokenem odczytu) lub SSH key/agent oraz zweryfikowane `known_hosts`.
Jeśli już można wykonać zwykłe `git ls-remote` jako ten użytkownik, SakuraOps
powinien móc pobrać to samo repozytorium.

Uwaga: komendy webowe działają bez terminala interaktywnego
(`GIT_TERMINAL_PROMPT=0`). Jeżeli credential helper wymaga za każdym razem
interakcji z przeglądarką/terminalem, może być konieczna jednorazowa autoryzacja
użytkownika usługi albo odpowiedni nieinteraktywny credential helper.

Opcjonalnie można ograniczyć dozwolone serwery:
```bash
export SAKURAOPS_GIT_ALLOWED_HOSTS=git.amuz.wroc.pl
```
To dodatkowe zabezpieczenie przed synchronizacją z nieautoryzowanych hostów.
Zarówno URL w GUI, jak i w CLI nie może zawierać hasła, tokenu czy parametrów.
HTTPS zachowuje standardową weryfikację TLS; SSH korzysta z normalnej
weryfikacji klucza serwera.

## Granice i plan dalszy

- Git jest uruchamiany z listą argumentów (bez powłoki) i limitem 90 sekund
  na operację; nie wykonujemy `git push` ani `git reset --hard`.
- Wyłączone są hooki Git, pobieranie submodułów i niebezpieczne protokoły.
  **Globalna konfiguracja Git i zaufane repozytorium należą do obowiązków
  administratora użytkownika usługi**. Git checkout może uruchamiać dodatkowe
  filtry skonfigurowane przez tego użytkownika. Nie traktuj repozytoriów
  od niezaufanych nadawców jako bezpiecznych.
- Wpis repozytorium w SakuraOps można zarządzać z GUI i CLI. Konfiguracja
  poświadczeń Git z GUI nie jest jeszcze zaimplementowana: na tym etapie Git
  korzysta z istniejących poświadczeń użytkownika usługi.
- Aktualny etap wymaga dalszego audytu pod kątem zasobów dyskowych,
  synchronizacji zewnętrznej, usuwania checkoutów i uprawnień, zanim zostanie
  udostępniony w sieci produkcyjnej.

## Lokalny test

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/manual-git-sync
source .venv/bin/activate
cd frontend && npm install && npm run build && cd ..
python -m pytest -q
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload
```

Sprawdź, że `repository sync ansible` działa jako użytkownik WSL mający
dostęp do Forgejo. Na prywatnym repo bez skonfigurowanych poświadczeń należy
oczekiwać czytelnej odmowy synchronizacji zamiast promptu w aplikacji.

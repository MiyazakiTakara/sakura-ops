# Inventory i Playbooki — przeglądanie i edycja

SakuraOps pokazuje pliki z synchronizowanego checkoutu Git. Strony **Inventory**
i **Playbooki** pozwalają wybrać repozytorium, otworzyć plik w edytorze
CodeMirror i zapisać lokalnie. Nie wywołują Ansible.

## CLI

```text
SakuraOps# show repository ansible inventory
SakuraOps# show repository ansible playbooks
SakuraOps# show repository ansible file inventory/hosts.ini
SakuraOps# show repository ansible diff inventory/hosts.ini
SakuraOps# repository edit ansible playbooks/linux-inventory.yml
```

Ostatnia komenda przełącza panel na odpowiedni widok i otwiera plik
w graficznym edytorze. Przycisk **Pokaż różnice** pokazuje lokalny Git diff
zapisanego pliku względem commita (bez wykonywania commit/push).
GUI i CLI używają tych samych endpointów i zasad
uprawnień administratora.

## Ograniczenia

- Dozwolone są tylko **śledzone przez Git** pliki pod `inventory/`,
  `inventories/`, `playbook/` lub `playbooks/` z rozszerzeniami
  `.ini`, `.yml`, `.yaml`.
- Limit rozmiaru odczytu/zapisu: **64 KiB**, tylko UTF-8.
- Symlinki i ścieżki wychodzące poza checkout są odrzucane.
- Zapis wymaga wersji SHA-256 pliku odczytanej przy otwarciu; kiedy
  plik zmieni się pomiędzy odczytem i zapisem, API zwróci 409.
- Zapis jest atomowy w obrębie katalogu roboczego, ze wspólną blokadą
  względem synchronizacji Git SakuraOps.
- **Nie ma automatycznych commitów ani push**. Po zmianie pliku
  `repository sync ansible` odmówi aktualizacji do czasu świadomego
  rozstrzygnięcia lokalnych modyfikacji. Nie wolno wykonywać `git reset --hard`
  w ramach automatycznego procesu SakuraOps.
- To funkcja administracyjna. Inventory i playbooki mogą zawierać poufne
  konfiguracje; tylko zalogowany administrator może je odczytywać i zapisywać.

## Odbiór w WSL

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/git-file-browser
source .venv/bin/activate
python -m pytest -q
cd frontend && npm install && npm run build && cd ..
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload --reload-dir backend/app
```

Zalecany pierwszy test: otwórz `inventory/hosts.ini` i oba playbooki w
trybie podglądu. Zapis przetestuj na kontrolowanej zmianie testowego pliku,
ponieważ modyfikuje lokalny checkout. Sprawdź konflikt 409 przy zapisie
nieaktualnej zawartości. Nie uruchamiaj playbooków.

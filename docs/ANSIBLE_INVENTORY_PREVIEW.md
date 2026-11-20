# SakuraOps — hosty i grupy Ansible z inventory

Widok **Serwery** pozwala wybrać zsynchronizowane repozytorium Git oraz
śledzony plik w `inventory/` lub `inventories/`. Pokazuje liczbę hostów,
grupy i przypisania hostów (w tym grupy potomne).

Operacje są dostępne także w konsoli `skr`:

```text
SakuraOps# show hosts ansible inventory/hosts.ini
SakuraOps# show groups ansible inventory/hosts.ini
```

## Ograniczenia

To **statyczny podgląd** obsługujący inventory INI i YAML.

- Parser nie uruchamia `ansible-inventory`, playbooków, pluginów ani Jinja.
  Nie nawiązuje połączeń SSH ani nie pobiera facts.
- Zawartość zmiennych hostów i grup, w tym poświadczeń, **nie jest zwracana**
  do panelu ani konsoli. Wyświetlane są tylko nazwy hostów i grup.
- Pliki muszą być śledzone w lokalnym checkout Git; odczyt używa tych samych
  limitów i kontroli ścieżek co edytor.
- Maksymalnie 64 KiB na plik, 2000 hostów, 500 grup i 30 poziomów YAML.
  Cykliczne grupy oraz aliasy YAML są blokowane.
- Inventory dynamiczne, konstrukcje Jinja, pluginy i złożone warianty składni
  nie są interpretowane. W takich przypadkach wynik może różnić się od
  prawdziwej ewaluacji Ansible — nie używaj go jako autoryzacji wykonania.

## Zależności i odbiór

YAML wymaga `PyYAML` w środowisku Python:

```bash
cd ~/sakura-ops
git fetch origin
git switch feat/ansible-inventory-preview
source .venv/bin/activate
python -m pip install pyyaml
python -m pytest -q
cd frontend && npm install && npm run build && cd ..
python -m uvicorn backend.app.main:app --host 127.0.0.1 --port 8000 --reload --reload-dir backend/app
```

Po zalogowaniu otwórz **Serwery**, wybierz `ansible` oraz
`inventory/hosts.ini`. Zmiana pliku/źródła aktualizuje listy.
Testy CI sprawdzają, że nie są ujawniane zmienne oraz że pliki niepoprawne
i cykliczne są odrzucane.

## Podgląd zadań (bez wykonania)

Dostępny jest także widok **Zadania**. Wybierz repozytorium, inventory,
śledzony playbook i grupę docelową. SakuraOps zweryfikuje wybraną grupę i
pokaże statyczną listę hostów, które **mogłyby** zostać wybrane. Żadne
połączenie z serwerami nie następuje i nie jest uruchamiany Ansible.

Te same dane można uzyskać w konsoli:

```text
SakuraOps# task plan ansible inventory/hosts.ini playbooks/linux-inventory.yml all
```

Dostęp wymaga roli administratora. Plan nie jest zapisywany ani wykonywany;
nie można przekazać dowolnych argumentów procesu. Nie jest to pełna ewaluacja
`hosts:`, `when:` czy innych warunków playbooka.

## Następna faza: zadania

Do bezpiecznego uruchamiania playbooków potrzebna będzie **oddzielna**
autoryzacja akcji, jawne wybranie inventory, playbooka i limitu hostów,
walidacja składni, izolacja procesu, zablokowane niebezpieczne rozszerzenia,
limity czasu/zasobów, kolejka, dziennik audytowy i kontrolowany dostęp SSH.
Ten PR **nie uruchamia żadnych zadań Ansible**.

## Podpowiedzi Tab w konsoli

Konsola `skr` uzupełnia kolejne argumenty na podstawie dozwolonych
poleceń i aktualnej konfiguracji SakuraOps, również po wpisaniu samej
nazwy polecenia. Przykłady:

- `show hosts` + Tab — podpowiada repozytorium i plik inventory
- `show groups` + Tab — te same źródła inventory
- `task plan` + Tab — repozytorium, inventory, playbook oraz grupę
- `repository sync`, `repository edit`, `repository push` + Tab —
  odpowiednio repozytoria, śledzone pliki i aktualny SHA
- `show repository` + Tab — repozytorium, rodzaj danych i pliki
- `edit`, `nano` i `cat` + Tab — pliki z lokalnego workspace

Gdy następny argument ma jedną wartość, Tab może uzupełnić kilka kolejnych
argumentów. Gdy wartości jest wiele, kolejny Tab pokazuje wybór. Danych,
których nie można bezpiecznie wywnioskować (nowa nazwa repozytorium, URL,
opis commita), SakuraOps nie wymyśla — pokazuje wskazówkę składni.
Wyniki pochodzą wyłącznie z istniejącego, lokalnego stanu aplikacji;
podpowiedzi nie wykonują Ansible ani operacji zapisujących Git.

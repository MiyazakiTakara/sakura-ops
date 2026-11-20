# PR #20 — oddzielna usługa przyjmowania zleceń

**Zakres:** oddzielny proces/usługa działająca jako inny użytkownik systemowy, z gniazdem Unix, prostą kolejką potwierdzeń i rozdzieleniem od API FastAPI. **Ten PR nie jest gotowym wykonawcą Ansible.** Celowo nie przyjmuje playbooków, argumentów modułów ani poleceń. Stany `queued` i `validated` opisują wyłącznie odbiór/walidację metadanych. Nie ma połączeń SSH, zdalnych hostów, Ansible ani operacji na repozytorium Git.

## Granice zaufania

- Administrator musi wykonać preflight PR #19 i jawnie go zatwierdzić.
- API ponownie waliduje aktualny SHA, pliki, dozwolone hosty i fingerprint raportu.
- API wysyła do gniazda tylko: SHA, fingerprint, listę aliasów hostów i stałą operację `submit`.
- API nie uruchamia demona, subprocessów ani powłoki na żądanie użytkownika.
- Serwer gniazda sprawdza UID połączenia przez Linux `SO_PEERCRED`; klient niezależnie weryfikuje UID serwera. Obie tożsamości muszą być różne; root jest wykluczony po stronie demona.
- Demon przechowuje same metadane w odrębnej bazie SQLite właściciela wykonawcy. API nie ma bezpośredniego dostępu do tej bazy.
- Kolejka maksymalnie 10 potwierdzeń naraz; brak równoczesnych potwierdzeń na tych samych aliasach; osobne stany `cancelled`, `validated`, `interrupted`.
- Restart zmienia zaległe `queued` na `interrupted`. Nie ma automatycznego ponawiania ani wykonania.
- Endpointy API wymagają roli admin, sesji i nagłówka CSRF.

## Wariant laboratoryjny

To **instrukcja dla wydzielonej maszyny testowej**, nie do instalacji bezpośrednio na produkcyjnym Semaphore. Domena wdrożeniowa (np. `/opt/sakuraops`) musi być czytelna dla konta usługi, ale kod i pliki wykonawcze nie mogą być przez nie modyfikowalne.

Przykładowe przygotowanie administratora po instalacji aplikacji do `/opt/sakuraops`:

```bash
sudo groupadd --system sakuraops-ipc
sudo useradd --system --no-create-home --shell /usr/sbin/nologin --gid sakuraops-ipc sakuraops-exec
sudo install -d -m 0750 -o sakuraops-exec -g sakuraops-ipc /run/sakuraops-exec
sudo install -d -m 0700 -o sakuraops-exec -g sakuraops-ipc /var/lib/sakuraops-exec
```

Administrator musi osobno przydzielić **wyłącznie procesowi API** dostęp do grupy `sakuraops-ipc` (np. przez `SupplementaryGroups=` w jego jednostce systemd), bez umieszczania prywatnych kluczy wykonawcy we współdzielonym katalogu. Nie zmieniaj uprawnień bazy danych API.

Dla tej wersji wykonawca używa wyłącznie biblioteki standardowej Pythona.
Nie należy uruchamiać go z katalogu domowego konta dewelopera ani kopiować
całego repozytorium z sekretami. Wystarczy **jedno sprawdzone źródło**
`backend/app/executor_service.py` w katalogu systemowym, należącym do root:

```bash
sudo install -d -m 0755 -o root -g root /opt/sakuraops-executor
sudo install -m 0644 -o root -g root backend/app/executor_service.py /opt/sakuraops-executor/executor_service.py
```

Przykładowa usługa wydzielona (zapisz jako osobną jednostkę systemd,
dostosuj Python, UID i polityki do środowiska):

```ini
[Unit]
Description=SakuraOps admission-only isolated executor
After=network.target

[Service]
Type=simple
User=sakuraops-exec
Group=sakuraops-ipc
WorkingDirectory=/opt/sakuraops-executor
RuntimeDirectory=sakuraops-exec
RuntimeDirectoryMode=0750
StateDirectory=sakuraops-exec
StateDirectoryMode=0700
ExecStart=/usr/bin/python3 /opt/sakuraops-executor/executor_service.py --socket /run/sakuraops-exec/executor.sock --database /var/lib/sakuraops-exec/receipts.sqlite --api-uid API_UID
NoNewPrivileges=yes
PrivateTmp=yes
PrivateNetwork=yes
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/run/sakuraops-exec /var/lib/sakuraops-exec
RestrictAddressFamilies=AF_UNIX
SystemCallArchitectures=native
CapabilityBoundingSet=
UMask=0077
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Zastąp `API_UID` numerycznym UID użytkownika serwisu WWW. Dla usługi systemd `RuntimeDirectory` pozwala odtworzyć gniazdo po restarcie, bez automatycznego ufania pozostawionym plikom socketu. W przypadku ręcznego startu po awarii najpierw zweryfikuj, że poprzedni demon już nie działa; dopiero wtedy usuń osierocone gniazdo. Uwaga: `UMask=0077` daje bezpieczny plik bazy 0600, a serwer gniazda po utworzeniu jawnie ustawia uprawnienia 0660 (grupa). Oba katalogi i klucze pozostają poza repozytorium.

Proces API uruchamiasz z:

```bash
export SAKURAOPS_EXECUTOR_SOCKET=/run/sakuraops-exec/executor.sock
```

W WSL z jednym użytkownikiem gniazdo zostanie **celowo odrzucone**. Nie obchodź tego przez uruchamianie usługi jako root lub konto API.

## API (wyłącznie admin + CSRF)

- `GET /api/executor/health`: tryb `admission_only`, `exec_enabled: false`.
- `POST /api/executor/receipts?review_id=ID`: zlecenie metadanych zatwierdzonego, aktualnego preflightu.
- `GET /api/executor/receipts/ID`: stan potwierdzenia.
- `POST /api/executor/receipts/ID/cancel`: anulowanie oczekującego potwierdzenia.

Nie powstają zadania Ansible, nie ma CLI do dowolnych komend i nie są uruchamiane playbooki.

## Odbiór

1. CI: testy protokołu, odrzucenie dowolnych komend, brak uprawnień, walidacja SHA, blokada aliasów, anulowanie, restart i limity kolejki.
2. W laboratorium: uruchom usługę i API jako **różne konta nie-root**, sprawdź `GET /api/executor/health`.
3. Spróbuj podać niezatwierdzony/zmieniony preflight, złe UID, nieznane pole i nieznany typ operacji. Wszystkie muszą zostać odrzucone.
4. Zweryfikuj prawa socketu i bazy. Brak `ansible-playbook`, `ssh` i kluczy SSH w procesie wykonawcy.
5. Po prawidłowym odbiorze dopiero w PR #21 projektuj osobny tryb wykonania kontrolowanych playbooków diagnostycznych; nie uruchamiaj produkcji bez odrębnej autoryzacji.

**Ograniczenie:** w obecnym etapie brak jeszcze polityk pozwalających na realną pracę Ansible. Jest to warstwa separacji i dopuszczania metadanych, a nie kompletna piaskownica do wykonania niezaufanych playbooków.

## Ograniczenie bezpieczeństwa przed PR #21

Każdy proces z prawem połączenia do gniazda (ten sam UID API lub
członkostwo w grupie `sakuraops-ipc`) może wysłać własny `submit`
bez zatwierdzenia preflightu przez API. Demon waliduje jedynie schemat
metadanych i UID, **nie potwierdza samodzielnie podpisu ani uprawnień
administratora**. Dlatego `validated` nie stanowi dowodu autoryzacji.

**Blokada wdrożeniowa:** przed dodaniem jakiegokolwiek wykonania Ansible
należy usunąć tę możliwość obejścia, np. przez weryfikowalny, jednorazowy
bilet zatwierdzenia i niezależne sprawdzenie dopuszczonego profilu w demonie.
Do tego czasu `exec_enabled: false` musi pozostać niezmienne. Nie wolno
traktować obecnych rekordów `approved` ani `validated` jako pozwolenia
na uruchomienie polecenia lub playbooka.

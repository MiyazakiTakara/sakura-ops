# 04. Gotowe zadania i przykładowe użycie

## Model katalogu

Katalog wbudowany jest dostępny bez Git i pisania kodu. Zadanie odwołuje się do konkretnej wersji typowanej akcji agenta. GUI generuje formularz z walidowanego schema; agent niezależnie waliduje tę samą wersję i lokalną politykę. Nie budujemy własnego ogólnego języka skryptowego. MVP: Linux, Ubuntu/Debian i rodzina RHEL/Oracle Linux po potwierdzeniu macierzy wersji. Inne platformy i urządzenia sieciowe poza zakresem obecnej roadmapy.

Klasy D0 = diagnostyka nie-root; D1 = dodatkowy odczyt uprzywilejowany lub jawna zmiana cache (oznaczyć odrębnie); Z1 = ograniczona zmiana bez planowanego przestoju; Z2 = możliwy przestój/duży wpływ. D0 nie oznacza zerowych śladów: logi, połączenie i odczyty są normalne. D1/Z1/Z2 wyłączone do osobnego odbioru.

| ID | Nazwa | Klasa | Wynik / parametry | Etap |
| --- | --- | --- | --- | --- |
| agent.ping | Sprawdź połączenie | D0 | challenge/response, latency, agent version, last_seen | 3 |
| linux.facts | Informacje o systemie | D0 | OS, kernel, arch, uptime, boot ID; bez env | 3 |
| linux.resources | Sprawdź zasoby | D0 | RAM, swap, dyski, inode, load, jednostki i timestamp | 4 |
| linux.services.status | Sprawdź usługi | D0 | lista zatwierdzonych unitów, active/failed/unknown | 4 |
| linux.updates.audit | Sprawdź dostępne aktualizacje | D0 | cache status, lista wersji, security classification lub unknown | 4 |
| linux.packages.info | Sprawdź wersje pakietów | D0 | dozwolone nazwy, installed/not_installed, wersja | 4 |
| linux.reboot.status | Sprawdź potrzebę restartu | D0/D1 | wskaźnik zależny od OS, unknown jeśli brak metody | 4 |
| linux.certificates.audit | Sprawdź ważność certyfikatów | D0 | zatwierdzone publiczne certyfikaty, expiry, dni, błędy | 4 |
| linux.updates.refresh | Odśwież listę aktualizacji | D1 | jawny refresh cache i ruch do repo, bez instalacji | 7 |
| linux.updates.security | Zainstaluj aktualizacje bezpieczeństwa | Z2 | plan pakietów, canary, backup, okno, postcheck | 7 |
| linux.updates.selected | Zaktualizuj wybrane pakiety | Z2 | zatwierdzone pakiety i wersje, zależności | 7 |
| linux.services.reload | Przeładuj konfigurację usługi | Z1/Z2 | allowlist, config validate, health | 7 |
| linux.services.restart | Uruchom ponownie usługę | Z2 | jedna dozwolona jednostka, postcheck | 7 |
| linux.services.start_stop | Uruchom lub zatrzymaj usługę | Z2 | action enum, jednostka, zależności | 7 |
| linux.reboot | Uruchom ponownie serwer | Z2 | okno, reason, boot ID i health po powrocie | 8 |
| linux.config.deploy | Wdróż zatwierdzoną konfigurację | Z2 | artefakt, backup, validate, atomowa podmiana, postcheck | 8 |
| agent.version | Sprawdź wersję agenta | D0 | build/version, protocol compatibility | 3 |
| agent.upgrade | Zaktualizuj agenta | Z2 | podpisany release, zgodność, canary, rollback binarki | 8 |

## Semantyka i adaptery

**Agent ping:** nie ICMP ani test SSH. Wysyłamy podpisany challenge do właściwej tożsamości, weryfikujemy odpowiedź. Brak odpowiedzi to agent unreachable, nie dowód wyłączenia VM. Rozróżnić offline, revoked, incompatible, timeout.

**Fakty i zasoby:** odczyt z bezpiecznych API OS/proc/sysfs z allowlistą, nie env, hasła, pliki użytkowników. Rozróżnić available/free RAM, inode, mountpoint, rozmiar i jednostki. Konfigurowalne progi dysku, przykładowo 80% warning i 90% critical; to proponowane progi produktu, nie pomiar. Nie usuwać danych automatycznie. Testy na braku mounta, dużych liczbach, locale i błędach dostępu.

**Usługi:** systemd adapter, wybór z zatwierdzonego wykazu, bez dowolnej jednostki z formularza. Wynik active/inactive/failed/unknown i czas. Nie czytać dowolnych journal logów w MVP. Restart/reload wymaga osobnego helpera, stałej operacji, pre/post health, timeout. Nie fallback do restartu po nieudanym reload. Nie restartować sieci, SSH, huba, DB/klastra bez dedykowanej polityki. Dla usługi Nginx przykładowy postcheck: z góry zatwierdzony test konfiguracji, reload, potwierdzenie active i aplikacyjnego health. Nie uznawać active za dowód pełnego działania aplikacji.

**Aktualizacje:** Debian/Ubuntu adapter APT, RHEL-family adapter DNF/YUM; dokładna semantyka i supported versions w labie. Odczyt istniejących metadanych bez wymuszania sieci/refresh; jeśli backend tego nie gwarantuje, oznaczyć refresh wymagający zgody. RHEL-family może zwracać kod 100 dla dostępnych aktualizacji, nie traktować go jako zwykłego błędu bez testu konkretnego backendu. Rozróżnić cache stale/missing, brak advisory metadata i faktyczne zero. Nie klasyfikować security po nazwie pakietu. D1 odświeża cache jawnie i nie instaluje. Z2: zweryfikowane źródło, podgląd wersji i zależności/usunięć/restartów, odmowa rozbieżnego planu, lock managera, wolne miejsce, repo health, maintenance, canary. Nie obchodzić GPG/SSL, brak domyślnego dist-upgrade i usunięć. Pakiety mogą zrestartować usługę nawet bez reboot. Postcheck i raport wersji, restart required, błędy. Downgrade nie zawsze możliwy: backup/snapshot zgodny z usługą i sprawdzone odtworzenie, nie obiecywać rollbacku.

**Certyfikaty:** tylko zatwierdzone ścieżki publicznych certyfikatów, parsing daty, timezone, allowlist ścieżek, symlink checks, nigdy klucz prywatny ani zawartość sekretu. Wygasanie wyświetlane z datą; brak automatycznej wymiany.

**Reboot:** odrębna akcja Z2, explicit target i okno, wykluczenia i zależności, canary, potwierdzenie nowego boot ID oraz health po reconnect. Timeout = unknown, nie automatyczny kolejny restart.

**Config deploy:** tylko zaufany, wersjonowany artefakt i zatwierdzona ścieżka; diff z redakcją, kopia, validate, atomic swap, postcheck, jawnie przetestowane odtworzenie. Brak dowolnych plików i shell. Własne zadania: najpierw kompozycja z dozwolonych kroków (sprawdź, warunek, jawna operacja, postcheck), bounded DAG i schema; nie dowolne skrypty ani upload pluginu. Nowy typ akcji wymaga przeglądu kodu i release agenta. Oddzielny ADR dla ewentualnego modelu zaufanego autora kodu po MVP.

## Schemat i przykłady

TaskVersion przechowuje znormalizowaną definicję Sakura Task v1, wersję, digest oraz wyliczone ryzyko, efekty, wymagane capabilities i politykę. Akcje i ich parametry/output mają osobny zaufany rejestr kontraktów dostarczany z release agenta. Schemat i semantyka: [14-workflow-engine.md](14-workflow-engine.md). Wersja zamrożona w planie i harmonogramie; zmiana wymaga ponownej akceptacji. Wszystkie 18 powyższych pozycji ma deklarację w builtin/tasks w tym samym formacie co zadania własne. examples/task-catalog.yaml jest wyłącznie przykładowym indeksem, nie drugim formatem wykonawczym. Definicje nie są jeszcze wykonywalne; aktywacja wymaga implementacji i odbioru akcji.

1. Poranny audyt: grupa lab-linux, zadanie Sprawdź dostępne aktualizacje, 07:00, powiadomienie przy failed lub stale. Bez instalacji.
2. Nowy host lab-web-01: onboarding, ping, fakty, dysk, dodanie do grupy web-test; tylko D0.
3. Aktualizacja: wybór security, konkretne serwery, podgląd zależności, drugi akceptujący, canary, health, kontynuacja po sukcesie. Reboot osobno.
4. Awaria usługi: Sprawdź status, operator stwierdza przyczynę, zatwierdza reload po validate, health. Brak cichej eskalacji do restartu.
5. Problem wymagający ręcznej diagnostyki: osobna karta SSH w terminalu WWW, dozwolony SSH, normalne konto administratora, audyt rozpoczęcia/końca. Nie zamieniać sesji w zadanie automatyczne.

Każde zadanie ma test pozytywny, unsupported OS, błędne parametry, offline, timeout, brak uprawnień, replay, brak sekretów w logu; dla zmian dodatkowo partial failure, cancellation/unknown, idempotency, exact effects, recovery. Dla każdego adaptera matryca wspieranych OS/version i fixtures. Niewspierane zadanie oznaczyć w GUI, nie uruchamiać alternatywnej komendy na chybił trafił.

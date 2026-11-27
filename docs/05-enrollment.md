# 05. Podłączanie nowego serwera: skrypt i tożsamość

## Cel

Jedno zaproszenie w panelu i jedno uruchomienie zweryfikowanego skryptu na hoście Linux przez administratora. Bez SSH key do automatyzacji, bez Ansible, bez ręcznego YAML, bez nowych portów nasłuchu agenta. Host musi móc dotrzeć do zatwierdzonego kontrolera po HTTPS. Agent jest jawnie instalowaną usługą, widoczną w systemd, z lokalnymi logami i procedurą usunięcia. Podstawowy profil diagnostyczny nie-root. Zdalny SSH w przeglądarce włączany odrębnie. Instalator agenta tylko dla Linux; nie instaluje klienta operatora.

## Kontrakt skryptu (jeszcze nie zaimplementowany)

Nazwa: sakuraops-enroll.sh. Dystrybucja z wersjonowanego release, panel podaje URL, digest i podpis z kluczem publicznym zaufanym niezależnie od pobieranego pliku. Zamiast curl | sudo bash pobierz, zweryfikuj, uruchom. Fikcyjna domena .example:

    curl --fail --show-error --proto '=https' --tlsv1.2 \
      -o sakuraops-enroll.sh https://ops.example/releases/sakuraops-enroll.sh
    # Zweryfikuj podpis i digest zgodnie z instrukcją wydania.
    sudo bash ./sakuraops-enroll.sh --controller https://ops.example --invite-id INVITE_ID

Skrypt prosi o jednorazowy token bez echa. Dla automatyzacji --token-file (0600 root, nie symlink), token nie w URL/argv/env, sprzątanie przez dostarczający proces. --check wyłącznie odczyt i preflight, --non-interactive wymaga jawnego potwierdzenia parametrów i token-file, --resume dla próby z bezpiecznym lokalnym stanem, --uninstall z informacją o centralnym revoke. --help, --version, stabilne kody: 0 lokalna instalacja/zgłoszenie (nie gwarantuje akceptacji), 2 wejście, 3 wymagania, 4 TLS/API, 5 konflikt, 6 odrzucone zaproszenie, 7 rollback niepełny, 8 podpis/release. Nie przyjmuje arbitralnych komend, opcji systemd, dowolnego URL payloadu ani redirect do innego origin. Pokazuje plan zmian i pyta o potwierdzenie.

## Etapy rejestracji

1. Admin tworzy invite: expected alias, środowisko, grupa, dozwolony profil, expiry proponowane 15 minut, scope tylko enrollment. Token losowy min. 256 bitów, hash w DB, wyświetlany raz, nie logowany. Rate limit i atomowe claim, jeden invite = jeden host. Zaproszenie nie jest uprawnieniem root ani zdalną sesją.
2. Skrypt sprawdza root, OS/version/arch, systemd, curl, verifier, dostęp do zaufanego CA, DNS/TLS kontrolera, wolne miejsce i konflikty. Nie źródłuje niezweryfikowanej treści /etc/os-release. Nie instaluje zależności ani nie zmienia globalnego firewall/sshd. Jeżeli brak wymagań, wypisuje konkretne kroki i kończy. W MVP brak auto-install innych pakietów.
3. Pobiera właściwy artefakt agenta i manifest dla systemu/arch, weryfikuje podpis i digest z niezależnym zaufanym kluczem, sprawdza wersję i anti-downgrade. Nie wykonuje danych z API. Pakiet/binarka i jednostka root-owned, odrębne konto systemowe sakuraops-agent bez loginu interaktywnego i bez sudo, state dir 0700, pliki klucza 0600. Nie instalować opcjonalnego helpera ani brokera zdalnych sesji w profilu podstawowym.
4. Agent generuje lokalnie własny klucz i CSR; prywatny klucz nigdy nie opuszcza hosta. Skrypt wysyła claim przez HTTPS (token Authorization, strict schema, timeout, brak redirect, limit body). Serwer atomowo zużywa token, wiąże public key/CSR z invite i host ID. Odpowiedź tylko deklaratywne dane, attempt ID i wąski credential do status/resume. Nie ufa nazwie lub IP jako dowodowi tożsamości.
5. Lokalna konsola/skrypt pokazuje fingerprint klucza, panel pokazuje zgłoszenie pending_verification. Admin potwierdza porównanie przez niezależny kanał (np. konsola VM), nie sam fakt odebrania fingerprint przez enrollment API. Mismatch blokuje. Jeżeli odpowiedź claim utracona, procedura resume albo nowy invite, nie powtórne zużycie starego tokenu.
6. Po akceptacji issuer wydaje certyfikat clientAuth dla przypisanego agent ID. Agent pobiera go przez krótkotrwałe attempt credential i proof-of-possession; expiry i revoke sprawdzane transakcyjnie. Przypina kontroler i zaufane klucze podpisu z zatwierdzonej konfiguracji. Uruchamia normalny jawny systemd service, nawiązuje mTLS i raportuje heartbeat. Hub sprawdza certyfikat, serial/revoke, ID, kompatybilność i challenge. Tylko po sukcesie status active; bootstrap nie oznacza pełnych praw.
7. Panel pokazuje system/wersję agenta i pierwsze bezpieczne sprawdzenie. Żadnego automatycznego update, root helpera, zdalnej sesji ani harmonogramu.

## Lokalny układ i idempotencja

Proponowane ścieżki: /usr/lib/sakuraops/agent (root-owned binarka), /etc/sakuraops/agent.toml (root-owned, bez tokenu), /var/lib/sakuraops-agent/identity i journal (właściciel usługi, 0700), /var/lib/sakuraops-enroll (stan instalatora root-only), sakuraops-agent.service. Ostateczny layout w ADR dystrybucji. Umask 077, flock, bezpieczny mktemp/trap, zakaz set -x/eval, atomic rename, sprawdzenie właściciela/symlink, brak nadpisania niezależnej instalacji. Zapis managed marker z wersją, instance ID i host ID bez sekretu invite. Ponowne uruchomienie rozpoznaje istniejący poprawny agent i nie duplikuje usługi ani tożsamości. Inny kontroler/klucz = konflikt i jawne de-enroll, nie automatyczne przejęcie. Zmiana wersji tylko przez proces upgrade. SELinux: prawidłowe context i polityka, nie wyłączać enforcing.

Na błędzie przed instalacją zero zmian; po częściowej instalacji usuń tylko własne artefakty tej transakcji, jeśli bezpieczne. Jeżeli rollback niepewny, pokaż pozostałości i oznacz cleanup_required. Nie usuwać istniejących kont/plików niezwiązanych z instalatorem. Proof-of-possession i minimalny state do idempotentnego status report, bez utrzymywania reusable enrollment tokenu.

## API i stany

issued -> claimed -> prepared -> pending_verification -> approved -> connecting -> active; alternatywy expired, revoked, rejected, connection_failed, cleanup_required. Jednorazowy claim atomowo, attempt credential krótki i przypisany do jednego CSR, nie pozwala tworzyć hostów. Wyścig approve/revoke rozwiązywany w transakcji, ponowne sprawdzenie przed cert issuance i connect. Przekroczony expiry lub revoke nie aktywuje hosta. Agent cert nie jest tokenem użytkownika. Każde przejście audytowane bez sekretów. Przywracanie DB po awarii nie może przywrócić zaufania do wcześniej wycofanych certyfikatów bez reconciliacji.

## Aktualizacja, rotacja i odłączenie

Rotacja certyfikatu przed wygaśnięciem: mTLS aktualnej tożsamości, CSR/proof, nowy cert, potwierdzenie nowego połączenia, unieważnienie starego, audit. Odrębna rotacja kluczy podpisu kontrolera: zaufany bundle podpisany poprzednim kluczem lub ręczny proces recovery, overlap, wersja i anti-downgrade. Cert mismatch = blokada; brak automatycznego insecure fallback.

Agent upgrade: zweryfikowany pakiet z release, odrębna rola/zgoda, wersja docelowa, kompatybilność, canary, drain, health po restarcie i znany poprzedni podpisany artefakt do odtworzenia. Nie dawać zwykłemu agentowi możliwości samodzielnej podmiany root-owned binarki. Na początek lokalny upgrade przez administratora; centralny upgrade dopiero po osobnej bramce helpera.

Odłączenie: admin suspend/retire, unieważnia certyfikaty i grants, zamyka połączenia i sesje, blokuje kolejkę i harmonogramy, rozlicza running/unknown. Lokalny uninstall wyłącza i usuwa tylko zarządzaną usługę/artefakty/konto po sprawdzeniu zależności, usuwa lokalny klucz zgodnie z procedurą, opcjonalnie zachowuje sanitized log. Jeśli host niedostępny, stan „dostęp centralnie odwołany, usunięcie lokalne niepotwierdzone”. Centralny revoke nie oznacza fizycznego usunięcia. Ponowne enrollment = nowy klucz, nowy invite, świadome powiązanie z historią, żadnego automatycznego odzyskania praw.

## Testy i odbiór

ShellCheck i Bats dla parsera, --check, kodów, uprawnień, konfliktów, symlink, ponownego wykonania, TLS/CA, złego podpisu/digest, błędnej arch, przerwania i cleanup. Test token nie pojawia się w ps/log/history/URL. Integracja API: expiry, replay, równoczesny claim, zły CSR, revoke podczas approve, lost response, oversized JSON, redirect, zły fingerprint, cert serial revoked, poświadczenie innego hosta, klon VM. VM lab Ubuntu/Debian i Oracle Linux/RHEL-family: SELinux enforcing, brak nowego portu, outbound TLS, nie-root, brak sudo/helpera, reboot hosta, reconnect, downgrade rejection, uninstall/rotation.

Odbiór UX: świeża wspierana VM, jedno zaproszenie i uruchomienie skryptu plus niezależne potwierdzenie fingerprint, host online i zadanie D0 bez ręcznego YAML. Proponowany cel <=5 minut po spełnieniu wymagań sieci i pobraniu release, nie zmierzony czas. Implementacja skryptu wymaga działającego release/issuer/API i jest zaplanowana w etapie 3; nie umieszczamy pozornie działającego instalatora w gałęzi dokumentacji.

## Profil hosta

Profil centralny Linux Basic (21-profiles.md) jest osobny od lokalnego diagnostic capability. Wybór profilu i grupy przed enrollment nie nadaje roota ani uprawnień SSH, nie włącza harmonogramów. Po niezależnym potwierdzeniu i aktywacji pokazujemy effective config, źródła i ograniczenia. Zmiana profilu wymaga revision i recheck planów.

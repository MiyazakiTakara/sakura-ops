# 06. Zdalny dostęp SSH w terminalu WWW

## Zakres i granice

Wyłącznie Linux, bez oddzielnej aplikacji. Terminal skr z poprzedniej iteracji jest podstawowym interfejsem administracyjnym i działa zanim powstanie prawdziwe SSH. Późniejsza funkcja Połącz SSH otwiera osobną, jednoznacznie oznaczoną kartę w tym samym pływającym oknie. Nie ma zmiany interpretera skr na Bash, dowolnej powłoki kontrolera ani wywoływania shell poprzez agenta automatyzacji. Nie budujemy własnego protokołu kryptograficznego. Korzystamy z dojrzałego SSH i emulatora terminala w przeglądarce.

## Transport A: gateway SSH w sieci administracyjnej (pierwszy etap)

Przeglądarka komunikuje się przez uwierzytelniony WSS same-origin z wydzielonym gateway. Gateway jako odrębny nieuprzywilejowany proces realizuje protokół SSH do istniejącego sshd na uprzednio dopuszczonym serwerze w sieci administracyjnej. Przeglądarka nie otwiera raw TCP ani nie przechowuje prywatnych kluczy w localStorage. Gateway jest zaufanym punktem końcowym SSH i widzi interaktywny strumień w pamięci, nie nazywamy tego szyfrowaniem end-to-end między przeglądarką a hostem. Nie zapisuje treści sesji, haseł ani kluczy prywatnych w logach, telemetryce lub bazie. OS authentication nie może zostać zastąpione samą sesją panelu.

Host ID i dozwolona usługa SSH pochodzą z zatwierdzonego rejestru; nie przyjmować dowolnego IP/portu/SSH options z przeglądarki. Pin klucza hosta lub zaufanej CA SSH ustalony niezależnym kanałem, mismatch blokuje sesję. Egress gateway ograniczony do konkretnych dopuszczonych endpointów; DNS rebind/SSRF, loopback kontrolera i sieci wewnętrzne poza allowlistą blokowane. Bez automatycznej zmiany sshd, firewall i ACL. Jeżeli sieć nie pozwala na gateway do hosta, funkcja jest niedostępna. Nie obchodzić segmentacji.

## Transport B: opcjonalny access connector (po osobnym review)

Jeżeli właściciel zdecyduje o potrzebie połączeń przez sieci bez bezpośredniej trasy z gateway, osobny jawnie włączany connector na hoście może udostępniać wyłącznie zatwierdzoną lokalną usługę sshd poprzez uwierzytelniony kanał wychodzący. Osobna tożsamość i polityka od agenta zadań; instalacja domyślnie go nie włącza. Żadnego SOCKS, dowolnego TCP/port-forward, proxy do LAN, arbitralnego exec ani ruchu między hostami. Broker i connector niezależnie egzekwują host ID/service ID/expiry i lokalny cel. Normalne SSH nadal weryfikuje tożsamość hosta i konto użytkownika. Osobny security review, testy pivot i jawna zgoda właściciela sieci. Nie jest potrzebny do MVP.

## Przepływ sesji

1. Użytkownik zalogowany lokalnie lub opcjonalnym Keycloak wybiera host i indywidualne konto Linux. Uprawnienie remote_access odrębne od operatora zadań. Scope hosta, aktualny stan, powód i dodatkowa zgoda zgodnie z polityką. Brak root login.
2. API tworzy niezmienny plan sesji i jednorazowy grant związany z aktorem, sesją WWW, host ID, kontem OS, usługą SSH, polityką i expiry. Wstępnie 60 s do rozpoczęcia, 60 min limit całkowity, 10 min idle, do pomiaru. Grant nie w URL, nie w logach. Atomowe zużycie; Origin/CSRF i sprawdzanie autoryzacji WSS na starcie oraz w trakcie.
3. Gateway ponownie sprawdza pin, dozwolony cel, zgodę i limity; łączy się do SSH za pomocą sprawdzonej biblioteki. Preferowane indywidualne, krótkotrwałe certyfikaty SSH ze wspieranego issuer i normalnej konfiguracji OS, po oddzielnym ADR. Alternatywny standardowy keyboard-interactive/SSH prompt tylko po review, bez trwałego przechowywania hasła. Brak wspólnego konta i automatycznego sudo. Nie omijać PAM/MFA ani OS ACL.
4. Po udanym handshake powstaje PTY wyłącznie w sesji SSH, obsługa resize, Unicode i bounded input/output/backpressure. W karcie widoczne host, konto, stan, czas i Rozłącz. Zablokować agent forwarding, X11, port forwarding i file transfer w MVP; enforce również po stronie sshd. Terminal skr nadal działa niezależnie.
5. Rozłącz, wygaśnięcie, revoke, utrata sieci lub logout zamyka transport. Zmiana uprawnień, zawieszenie hosta lub revoke zgody ma zakończyć istniejącą sesję gateway. Zamknięcie transportu nie gwarantuje zabicia procesów odłączonych wcześniej przez użytkownika na hoście (np. tmux). Reconnect wymaga nowego grantu; brak automatycznego ponownego logowania.

## Audyt, prywatność i operacje

Metadane: kto, host, konto OS, powód, zgoda, identyfikator sesji, start/koniec, wynik, przyczyna zamknięcia. Brak automatycznego nagrywania treści terminala, keyloggingu, przechwytywania schowka lub zrzutów ekranu. Bufory ograniczone, wyczyszczone po sesji, brak sekretów w crash dump. Uprawnienia konta na Linux pozostają niezależne od panelu. UI pokazuje, że jest to prawdziwe SSH i działania mogą mieć skutki; nie obiecuje cofania poleceń. Gateway nie kontroluje niezależnych połączeń SSH poza SakuraOps.

## Odbiór

Lab: normalny indywidualny login, różne hosty, host key mismatch, błędna tożsamość, expired/replayed grant, cross-user i cross-host, zmiana roli, logout/revoke, próba dowolnego portu, SSRF/DNS rebind, utrata sieci, limit sesji i idle, nieuprawniony WSS origin, resize/Unicode, bounded log/output, bez sekretów. Próba wstrzyknięcia komendy skr nie uruchamia PTY. Test rozdzielenia sekretów i tożsamości agenta od gateway. Zamknięcie i przełączenie zakładek zgodnie z UX. B dopiero po osobnym review i testach. Żadnych połączeń do produkcji przed odrębną zgodą.

Przesyłanie plików przez SFTP, jeśli potrzebne, jako osobna jawna funkcja po review scope, ścieżek, uprawnień i logowania metadanych; nie część MVP. Nie planujemy klienta desktopowego ani zdalnego pulpitu.

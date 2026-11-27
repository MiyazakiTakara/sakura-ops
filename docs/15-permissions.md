# 15. Uprawnienia i zasady zgód

RBAC + scope grup + akcja/host + lokalna polityka agenta. Default deny. Rola to zbiór uprawnień, a nie hierarchia pozwalająca ominąć zatwierdzenia. Admin panelu nie ma automatycznie dostępu root ani SSH. Model jednej instancji organizacyjnej; multi-tenant poza zakresem.

| Operacja | Viewer | Operator | Approver | Admin | RemoteAccess |
| --- | --- | --- | --- | --- | --- |
| Odczyt hosta/runu w scope | tak | tak | tak | tak w scope | tylko jeśli ma odczyt |
| Plan/uruchom D0 | nie | tak | z Operator | z Operator | nie |
| Plan D1/Z1/Z2 | nie | tak po aktywacji | z Operator | z Operator | nie |
| Approve zmianę | nie | nie bez roli | tak w scope | z Approver | nie |
| Cancel własny run | nie | tak | z Operator | tak w scope | nie |
| Import draft zadania | nie | osobne task.write | osobne task.write | tak w scope | nie |
| Publish wersję custom | nie | osobne task.publish | osobne task.publish | tak | nie |
| Enrollment / revoke / profile policy / RBAC | nie | nie | nie | tak z reauth | nie |
| SSH WWW | nie | nie domyślnie | nie domyślnie | nie domyślnie | tak tylko konkretny host, konto i grant |
| Zarządzanie sekretami integracji | nie | nie | nie | osobne secret.manage, reauth | nie |

User może mieć kilka ról; scope jest listą grup lub jawnie nadanym globalnym uprawnieniem. Host w wielu grupach nie dziedziczy automatycznie uprawnień innego operatora; ocena po zasobie i akcji, deny ma pierwszeństwo. Widoczność list i agregatów filtrowana przed paginacją/liczeniem. Scope target group musi być dostępny, każdy host autoryzowany niezależnie, brak silent partial execution; nieautoryzowany target -> błąd bez ujawniania hosta. Zmiana członkostwa powoduje revision i unieważnia plany/granty; istniejące streamy SSE/SSH recheck i revoke. Uprawnienia w GUI i skr są identyczne, UI jedynie prezentuje wynik.

## Klasy akcji

D0 diagnostyka nie-root: operator uruchamia jawnie, harmonogram tylko po scoped grant. D1 dodatkowy odczyt uprzywilejowany/refresh cache: wymaga włączonego profilu i jawnej zgody zgodnie z polityką. Z1 ograniczona zmiana bez planowanego przestoju: explicit plan i approve. Z2 możliwy przestój/duży wpływ: drugi niezależny approver, okno, canary, backup/health i scope. Produkcja i ryzykowne operacje nie mogą używać self-approval; requester != approver, service account nie approver, zmiana aktora/target/parametrów/digest/okna/polityki = nowy plan. Domyślne instalacje nie mają D1/Z1/Z2 aktywnych; pojedynczy administrator w lab nie może fikcyjnie zatwierdzić ryzykownego zadania drugim kontem bez realnego odbioru. Break-glass lokalny dla odzyskania panelu nie omija polityki akcji.

## Sesje, tożsamość i sekrety

Cookie HttpOnly/Secure/SameSite, server-side sessions, rotation po login/role change, CSRF dla mutacji, logout/revoke, idle i absolute TTL ustalić w lab. Hasła Argon2id z parametrami benchmarkowanymi, throttle bez enumeracji kont. Keycloak opcjonalny OIDC PKCE z issuer/sub binding, brak automatycznego linkowania email; awaria IdP nie blokuje lokalnego loginu. CLI token hash, scope, expiry, revoke, last used; nie bearer w localStorage, URL lub historii. Agent cert nie jest user tokenem. Sekrety integracji w chronionych plikach/secret store, master key poza DB i backupem DB; UI write-only, pokaż stan i rotację, bez odzyskania wartości. SSH indywidualne konto OS i niezależny grant, nie agent ani wspólne hasło.

## Testy

Macierz każda rola × operacja × własny/obcy host × aktywny/revoked, brak sesji/CSRF, IDOR, zmiana grupy między plan/approve/start/dispatch, self-approval, service-account expiry, stale revisions, SSE reconnect i revoke, SSH grant i origin, audit redaction. Negatywny test ważniejszy od ukrytego przycisku. Wszystkie mutacje recheck w transakcji i audit. Policy decision ma code/reason i revision. Użytkownik bez scope nie widzi sekretów ani liczników obcych zasobów.

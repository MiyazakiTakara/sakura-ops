# 11. Zamknięty zakres MVP

Decyzja projektowa na podstawie ustaleń właściciela; odbiór funkcjonalny po NEXT-036. MVP jest wersją diagnostyczną lab, nie produkcyjną. Kolejne etapy zachowują plan z 08-roadmap.md.

## W zakresie

1. Fresh install jednej VM Linux, Compose, HTTPS, PostgreSQL, health/readiness, bootstrap lokalnego admina przez TTY, ustawienia instancji.
2. Lokalny login/logout/sesje/CSRF, podstawowy RBAC i scope grup, audyt, konta lokalne. Keycloak później.
3. Shell WWW, kompaktowa nawigacja, pływający skr, help/Tab/history/Ctrl+L/tryby i bezpieczny workspace CodeMirror. Polecenia działają przez te same usługi co GUI.
4. Rejestr hostów, grupy, tagi, środowisko lab/prod, jeden profil bazowy z konfiguracją. W MVP tylko Linux Basic, diagnostyczne profile innych typów jako nieaktywne propozycje. Brak harmonogramów przed etapem 5.
5. Agent nie-root, podpisane wydanie, instalator, zaproszenie i niezależny fingerprint, mTLS, heartbeat, cert renewal/revoke, suspend/retire, lokalny uninstall. Dwie rodziny OS w lab.
6. Wspólny Sakura Task v1, import i walidacja definicji, immutable version, plan, frozen targets, podpis i nonce, per-host/krok logi, anulowanie, historia. Wbudowane zadania D0 tylko po testach adaptera. Własne proste sekwencje D0 mogą używać tego samego parsera; zaawansowany wizualny kreator po MVP.
7. Fakty, zasoby, dysk, dozwolone usługi/pakiety/certyfikaty, audyt aktualizacji z cache i status reboot gdy możliwy bez eskalacji. Unsupported/unknown jawnie. D1 nie jest częścią MVP.
8. Testy jednostkowe/integracyjne/E2E/security, clean install, recovery i dokumentacja. Artefakt wydania lab, nie sam deployment produkcyjny.

## Poza MVP

Harmonogramy i SMTP (etap 5), SSH WWW i opcjonalny Keycloak (6), helper i zmiany pakietów/usług (7), reboot/config deploy/agent upgrade (8), zaawansowany kreator i Git (9). Docker/DB/Web profiles rozszerzane wraz z obsługą capabilities; Docker socket zakazany dla standardowego agenta. Brak Windows, desktop, Ansible runtime, dowolnego shell w zadaniach, HA/multi-site, pełnego NMS i autonomicznego naprawiania.

## Ścieżka odbioru

Na świeżej instancji z lab VM Ubuntu i Oracle Linux: utwórz admina, dodaj hosty z kreatora, zatwierdź fingerprints, zobacz heartbeat i system, utwórz grupę/profil, uruchom tę samą definicję audytu na obu hostach, sprawdź ujednolicone wyniki i przyczyny unsupported/stale, powtórz przez skr, anuluj odczyt, rozłącz host i sprawdź stan, przetestuj revoke. Viewer nie może zmieniać stanu; operator nie widzi hosta spoza scope. Negatywne testy replay, błędnego certyfikatu, dowolnego polecenia, fałszywego wyniku, restartu huba. Zero instalacji pakietów/restartów i zero połączeń produkcyjnych. Wszystkie wymagane gate mają dowody testów oraz owner review; nie zakładać PASS przed wykonaniem.

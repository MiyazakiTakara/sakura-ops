# 24. Zarządzanie aktualizacjami i oknami serwisowymi

MVP tylko audyt istniejących metadanych. D1 refresh, Z2 instalacja i restart to oddzielne bramki etapu 7/8. Wszystko przez Sakura Task v1, wspólne kontrakty akcji i adaptery APT/DNF, bez raw shell.

## Audyt

Widok host/pakiet, installed/candidate version, advisory/security known/unknown, cache state fresh/stale/missing, observed_at i adapter. Filtrowanie nie zamienia unknown w brak aktualizacji. Nie wywoływać sieci w D0; jeśli cache nieczytelny, pokaż instrukcję D1 dopiero gdy odbiór. D1 refresh cache nie instaluje pakietów, respektuje GPG/TLS, repo allowlist i locks. Błąd repo/expired key = blokada, nie disable verification. Nie importować repo keys z dowolnego URL.

## Plan zmiany

1. Wybierz hosty i typ security/selected, jawne pakiety i wersje. Preflight wylicza dokładne zależności, usunięcia, skutki restartu, free disk, lock, repo trust, kompatybilność, zależności usług, backup/snapshot i health. Nie zakładaj, że aktualizacja nie restartuje usług.
2. Pokaż diff efektów per host; zatwierdzona wersja i exact effect digest, frozen target, okno i rollback/recovery. Druga osoba Z2, canary 1, max parallel 1 domyślnie. Brak approval lub rozjazd cache/plan -> nowy plan, nie cichy fallback.
3. Lokalny helper tylko zatwierdzone operacje/adapter, ponownie sprawdza podpis, TTL, scope, local allowlist i plan. Nie używa szerokiego sudo ani bypass lock. Nie domyślny dist-upgrade, usunięcia, reboot ani restart krytycznej usługi. Jeśli nie można bezpiecznie przewidzieć efektu, odmowa.
4. Wynik: before/after package versions, status, changed, warnings, reboot_required/unknown, pre/post health. Stop batch na canary failure; nie kontynuuj kolejnych hostów po unknown. Retry mutacji tylko po nowym planie i ręcznej reconciliacji.
5. Recovery: snapshot/backup zgodny z usługą, test restore, jawny runbook; downgrade nie zawsze możliwy. Cancel nie rollback. Reboot osobna operacja z boot ID i health. Reload nie przechodzi automatycznie w restart.

## Okna i harmonogramy

IANA timezone, UTC zapis, start/end lokalne, definicja dni i cross-midnight, wyjątki/blackout, jawne next occurrence. DST gap skip (brak nadrabiania), fold tylko jedno wystąpienie wg uzgodnionej reguły wcześniejszego offsetu; testy. Global/env/profile/group/host ograniczenia przecinają się, konflikt blokuje. Rozpoczęcie i przewidywany czas muszą mieścić się w oknie; przed każdym hostem recheck. Jeśli okno kończy się podczas package transaction, nie kill w sposób grożący uszkodzeniem DB pakietów; stop nowych hostów, dokończ według adaptera, raport odchylenia i alert. Brak mutacji automatycznych w harmonogramach do osobnego etapu 8. D0 schedule pin task version i scope grant, brak overlap i missed skip. Notification throttled, no secrets.

## Odbiór

Disposable VM APT i DNF: locked cache, missing advisory, code 100, repository error, dependency drift, no space, failed canary, partial/unknown, reboot required, timeout, revoke, outside window, concurrent run, DST, rollback/restore i no arbitrary root. Każda akcja zmieniająca oddzielnie aktywowana dopiero po review. Produkcja osobno.

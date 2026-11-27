# ADR-0001: Własny agent i hub, Linux-only, browser-only

- Status: **Accepted** (właściciel, 2026-11-27, PR #45)
- Data: 2026-11-27
- Decydent: właściciel projektu
- Zakres NEXT: NEXT-001. Konsoliduje ADR-000 i ADR-001 z [01-audit.md](../01-audit.md).

## Zakres akceptacji

Właściciel zaakceptował 2026-11-27 kierunek architektoniczny i technologiczny. Spike P01 jest wyłącznie **dowodem wykonalności**, nie odbiorem produktu ani bramki wydania. Odroczone bramki weryfikacji pozostają **obowiązkowe** na swoich etapach i nie zostały uchylone; niepowodzenie którejś wymaga korekty lub nowego ADR. Akceptacja nie upoważnia do dostępu produkcyjnego, instalacji na rzeczywistych hostach, helpera uprzywilejowanego ani SSH.

## Kontekst

Stan wyjściowy i powody zmiany kierunku opisuje [01-audit.md](../01-audit.md). Poprzednia iteracja zapowiadała Ansible Runner, Keycloak i osobne ścieżki zadań, a część zapowiedzi nie była dostarczona. Potrzebny jest jeden model wykonania, widoczne onboarding i status hostów oraz granice zaufania możliwe do przetestowania.

## Decyzja

1. **Własny, jawnie instalowany agent** (usługa systemd) z wychodzącym kanałem mTLS do huba. Brak Ansible jako runtime wykonawczego; nie wdrażamy dwóch silników naraz.
2. **Hub** (API, worker, agent gateway, signer/issuer jako odrębne granice) wydaje plany i zlecenia. Rejestr hostów w DB jest źródłem prawdy. Protokół: [13-agent-protocol.md](../13-agent-protocol.md).
3. **Zarządzane hosty wyłącznie Linux** (macierz: [16-linux-compatibility.md](../16-linux-compatibility.md)). Brak Windows i innych systemów.
4. **Browser-only**: panel w zwykłej przeglądarce; brak aplikacji desktopowej i klienta natywnego. Opcjonalny tekstowy CLI `skr` dla Linuksa pozostaje poza zakresem podstawowym.
5. **Pływający terminal skr jest funkcją podstawową**, równorzędnym interfejsem tych samych usług co GUI. To kontrolowany interpreter, nigdy Bash ani powłoka kontrolera ([09-terminal.md](../09-terminal.md)).
6. **SSH w przeglądarce** (WSS) jest późniejszym, osobnym i opt-in modułem z własnym gate; nie jest ścieżką automatyzacji ([06-remote-access.md](../06-remote-access.md)).
7. Jeden format zadań Sakura Task v1 dla wszystkich zadań, także wbudowanych ([14-workflow-engine.md](../14-workflow-engine.md)).

## Rozważone alternatywy

- **Ansible push**: dojrzałe moduły, ale wymaga SSH, inventory i poświadczeń do hostów oraz przyjmuje dowolne playbooki, co zwiększa powierzchnię ataku i rozmywa model uprawnień. Odrzucone.
- **Własny kontroler SSH bez agenta**: mało instalacji, ale trwałe utrzymanie dostępu SSH do każdego hosta, rozbieżności systemów i brak stałego kanału statusu bez dodatkowej usługi. Odrzucone.

## Granice zaufania (skrót)

Pełny model: [20-threat-model.md](../20-threat-model.md), [03-architecture.md](../03-architecture.md).

- Agent wykonuje wyłącznie **typowane, skompilowane akcje** z zatwierdzonego, podpisanego zlecenia; brak surowej powłoki, dowolnego skryptu i pluginu z sieci.
- Domyślnie **D0**: nie-root, bez helpera, SSH, zmian, automatycznego restartu i harmonogramów modyfikujących. Rozszerzenia osobno za zgodą właściciela.
- **Signer/issuer** oddzielony od API i gateway; API nie ma klucza CA. Agent niezależnie egzekwuje **lokalną politykę** (allowlista), więc przejęcie huba samo nie daje dowolnego wykonania.
- Agent łączy się sam (outbound) przez mTLS z osobnej CA urządzeń; brak nowego portu nasłuchu agenta w profilu podstawowym.

## Poza zakresem

Aplikacja desktopowa, Windows i inne OS, Ansible jako runtime, dowolne skrypty i pluginy, obowiązkowy Git, Keycloak, Redis, Kubernetes lub chmura. Lokalny login działa niezależnie od opcjonalnego Keycloak.

## Konsekwencje i ryzyka

- Koszt utrzymania **adapterów** na wielu dystrybucjach oraz **dystrybucji i aktualizacji agenta** (podpisane artefakty, rotacja).
- Własny protokół i CA wymagają dojrzałych testów negatywnych (replay, klon, odwołanie) przed użyciem poza lab.
- Ryzyka resztkowe z [20-threat-model.md](../20-threat-model.md): root na hoście może przejąć tożsamość agenta i fałszować wyniki (panel nie jest systemem attestation); przejęcie signera może umożliwić podpisanie akcji, a lokalna polityka ogranicza zakres bez gwarancji; wykonanie na OS nie jest exactly-once przy crashu.

## Co daje akceptacja, a czego nie

Akceptacja potwierdza kierunek architektury i pozwala kontynuować pakiety implementacyjne w lab. **Nie upoważnia** do dostępu produkcyjnego, instalacji agenta na rzeczywistych hostach, helpera uprzywilejowanego, sesji SSH ani zmian na hostach; każde z nich wymaga odrębnej zgody właściciela i gate.

## Akceptacja

Zaakceptowany przez właściciela 2026-11-27 po przeglądzie tego ADR, [01-audit.md](../01-audit.md) i [20-threat-model.md](../20-threat-model.md). Zmiana decyzji: nowy ADR (patrz [README](README.md)).

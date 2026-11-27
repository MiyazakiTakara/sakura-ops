# SakuraOps Next

**Status: plan projektowy, 27.11.2026.** Nie jest działającą aplikacją ani instrukcją wdrożenia produkcyjnego.

SakuraOps Next to proponowany własny, self-hosted system zarządzania infrastrukturą z centralnym panelem WWW, lekkim agentem instalowanym na serwerach oraz wbudowanym terminalem WWW. Zakres całej obecnej roadmapy: wyłącznie zarządzane serwery Linux, bez aplikacji desktopowej. W tym wariancie rezygnujemy z Ansible jako zależności wykonawczej. Ansible nie jest potrzebny do konfiguracji, dodawania hostów ani uruchamiania zadań. Ewentualna integracja z istniejącymi playbookami to osobny temat po MVP.

## Proponowany stack v2

Go (hub, agent, worker, gateway, signer, opcjonalny CLI), Svelte 5 + TypeScript z Vite i statycznym SvelteKit, PostgreSQL + pgx/sqlc + Goose, gRPC/Protobuf/mTLS dla agentów, REST/OpenAPI i SSE dla GUI/CLI, WSS dla SSH. xterm.js i CodeMirror 6 zachowują poprzedni terminal i edytor. Docker Compose i systemd na hostach. Node.js wyłącznie w build/CI, nie jako usługa produkcyjna. Zmiana stacku jest propozycją do zatwierdzenia, szczegóły w [10-stack.md](docs/10-stack.md).

## Co ma potrafić administrator

- Zainstalować centralny system na jednej VM, zalogować się lokalnie i dodać pierwszy serwer.
- Wygenerować jednorazowe zaproszenie i uruchomić jeden zweryfikowany skrypt na zarządzanym Linuksie.
- Zobaczyć stan agentów, systemy, podstawowe zasoby, dostępne aktualizacje i problemy.
- Wybrać gotowe zadanie, serwery, zobaczyć jego skutki i uruchomić je bez pisania skryptu.
- Zaplanować cykliczne odczyty i kontrolowane czynności administracyjne.
- Otworzyć pływający terminal SakuraOps z promptem `skr >`, pomocą, historią, Tab i trybem konfiguracji, zgodnie z poprzednią iteracją. Terminal jest równorzędny z GUI, nie dodatkiem na koniec.
- Opcjonalnie otworzyć osobną kartę sesji SSH w tym samym oknie WWW po wdrożeniu i odbiorze modułu dostępu. SSH wymaga niezależnej autoryzacji.
- Znaleźć historię, wyniki i audyt; edytować i wersjonować definicje zadań w zaawansowanym widoku Git, bez obowiązku używania Git w codziennej obsłudze.

## Dokumentacja

**Obowiązkowe dla wykonawców:** [AGENTS.md](AGENTS.md) i [DESIGN.md](DESIGN.md). Zakres MVP w [11-mvp.md](docs/11-mvp.md).

| Dokument | Zakres |
| --- | --- |
| [01. Audyt](docs/01-audit.md) | Stan aktualnego main i PR #22, decyzje co zachować |
| [02. Produkt i UX](docs/02-product-ux.md) | Ekrany, kreatory, nawigacja, pływający terminal |
| [03. Architektura i bezpieczeństwo](docs/03-architecture.md) | Hub, agent, protokół, zadania, dane, granice zaufania |
| [04. Katalog zadań](docs/04-task-catalog.md) | Gotowe zadania, parametry, przykładowe przepływy |
| [05. Instalacja agenta i rejestracja](docs/05-enrollment.md) | Skrypt, certyfikaty, rejestracja, rotacja, usuwanie |
| [06. Zdalny dostęp](docs/06-remote-access.md) | Sesje SSH w przeglądarce, broker, uprawnienia |
| [07. Instalacja i eksploatacja](docs/07-operations.md) | Compose, konfiguracja, backup, aktualizacje, awarie |
| [08. Roadmapa i backlog](docs/08-roadmap.md) | Etapy, zadania, testy, bramki wydania |
| [09. Terminal i zgodność z poprzednią iteracją](docs/09-terminal.md) | skr, tryby, polecenia, edytor, Git, migracja i testy |
| [10. Stack i ADR](docs/10-stack.md) | Go, Svelte, kontrakty, wdrożenie i bramka technologiczna |
| [14. Sakura Task v1](docs/14-workflow-engine.md) | Jednolity YAML/JSON, adaptery i wbudowane zadania |
| [11. MVP](docs/11-mvp.md) | Zakres i odbiór pierwszej wersji |
| [12. Ekrany UI](docs/12-design-system.md) | Routing, makiety opisowe i stany |
| [13. Protokół agenta](docs/13-agent-protocol.md) | gRPC, tożsamość, podpisy, recovery |
| [15. Uprawnienia](docs/15-permissions.md) | RBAC, scope i zgody |
| [16. Linux](docs/16-linux-compatibility.md) | Macierz OS i adaptery |
| [17. Testy](docs/17-testing.md) | CI, lab i odbiór |
| [18. Rozwój](docs/18-development-rules.md) | PR, role agentów, DoD |
| [19. Release](docs/19-release-plan.md) | Wersjonowanie i wdrożenia |
| [20. Threat model](docs/20-threat-model.md) | Granice i zagrożenia |
| [21. Profile](docs/21-profiles.md) | Grupy, profile i effective policy |
| [22. Model danych](docs/22-data-model.md) | Tabele, indeksy i transakcje |
| [23. API](docs/23-api-contract.md) | REST/SSE, skr i endpointy |
| [24. Aktualizacje](docs/24-updates.md) | Audyt, zmiany, okna |
| [25. Backlog](docs/25-backlog.md) | Pakiety PR i zależności |
| [ADR](docs/adr/README.md) | Rejestr decyzji architektonicznych (ADR-0001, ADR-0002: Proposed) |
| [Dev setup](docs/dev/README.md) | Środowisko deweloperskie, `make check`, spike P01 (po angielsku) |
| [Schema v1](schemas/sakura-task-v1.schema.json) | Projekt walidacji struktury |
| [Wbudowane zadania](builtin/tasks/) | Deklaracje projektowe wszystkich 18 pozycji katalogu |
| [Przykładowy katalog](examples/task-catalog.yaml) | Makiety danych do projektowania, niewykonywalne |
| [Przykładowa konfiguracja](examples/config.example.yaml) | Proponowane ustawienia bez sekretów |

## Zasady

GUI zapewnia prostą obsługę, a terminal skr jest od początku równorzędnym interfejsem tych samych usług. Szczegóły techniczne są dostępne na żądanie. Nie powstaje aplikacja desktopowa, klient natywny, agent dla Windows ani obsługa innych systemów niż Linux w tej roadmapie. Zwykła przeglądarka na dowolnym systemie może korzystać z panelu. Osobny CLI skr pozostaje opcjonalny dla Linuksa, bez GUI. Agent nie jest uniwersalną powłoką na polecenie API, wykonuje ograniczone, typowane operacje. Automatyzacja i dostęp interaktywny mają osobne granice zaufania i uprawnienia. Domyślne wdrożenie jest odczytowe; zmiany, restart i zdalne sesje to odrębne możliwości wymagające autoryzacji. Agent jest jawnie instalowaną, widoczną usługą systemową z dokumentacją i możliwością usunięcia. Brak ukrytego dostępu, zbierania danych użytkowników, obchodzenia uwierzytelnienia i automatycznego otwierania portów.

Gałąź zawiera wyłącznie dokumentację i przykładowe dane, bez starego kodu. Commit zachowuje rodzica main dla identyfikowalności źródła audytu; nie jest orphan branch w sensie historii Git. Nie scalać mechanicznie z main, ponieważ usunęłoby to stare pliki. Decyzję o przejściu na Next podjąć po odbiorze. Repozytorium jest publiczne: nie publikować realnych adresów infrastruktury, danych klientów, kluczy, tokenów ani produkcyjnych konfiguracji.

Wszystkie zadania, zarówno wbudowane, jak i własne, używają tego samego formatu Sakura Task v1 i silnika. YAML i JSON są równoważne; zadania są przenośne między wspieranymi dystrybucjami Linux dzięki adapterom Go. Wbudowane definicje w builtin/tasks są na razie kontraktami projektowymi, nie aktywnymi zadaniami.

Plan obejmuje projekt UI, model danych, role, protokół, profile, aktualizacje, testy i wydania. Nie jest dowodem wykonania. Przed pierwszym kodem: review P01, spike i akceptacja kontraktów. Wszystkie niezweryfikowane szczegóły są projektowymi domyślnymi wartościami. Legacy main pozostaje niezależny.

# SakuraOps Next

**Status: plan projektowy, 27.11.2026.** Nie jest działającą aplikacją ani instrukcją wdrożenia produkcyjnego.

SakuraOps Next to proponowany własny, self-hosted system zarządzania infrastrukturą z centralnym panelem WWW, lekkim agentem instalowanym na serwerach oraz aplikacją administracyjną do nawiązywania zdalnych sesji. W tym wariancie rezygnujemy z Ansible jako zależności wykonawczej. Ansible nie jest potrzebny do konfiguracji, dodawania hostów ani uruchamiania zadań. Ewentualna integracja z istniejącymi playbookami to osobny temat po MVP.

## Co ma potrafić administrator

- Zainstalować centralny system na jednej VM, zalogować się lokalnie i dodać pierwszy serwer.
- Wygenerować jednorazowe zaproszenie i uruchomić jeden zweryfikowany skrypt na zarządzanym Linuksie.
- Zobaczyć stan agentów, systemy, podstawowe zasoby, dostępne aktualizacje i problemy.
- Wybrać gotowe zadanie, serwery, zobaczyć jego skutki i uruchomić je bez pisania skryptu.
- Zaplanować cykliczne odczyty i kontrolowane czynności administracyjne.
- Z aplikacji SakuraOps Connect otworzyć autoryzowaną sesję SSH, a w późniejszym etapie również RDP do wspieranych hostów. Nie budujemy własnego protokołu pulpitu zdalnego.
- Znaleźć historię, wyniki i audyt. Konsola skr jest opcjonalnym interfejsem tego samego API.

## Dokumentacja

| Dokument | Zakres |
| --- | --- |
| [01. Audyt](docs/01-audit.md) | Stan aktualnego main i PR #22, decyzje co zachować |
| [02. Produkt i UX](docs/02-product-ux.md) | Ekrany, kreatory, nawigacja, aplikacja Connect |
| [03. Architektura i bezpieczeństwo](docs/03-architecture.md) | Hub, agent, protokół, zadania, dane, granice zaufania |
| [04. Katalog zadań](docs/04-task-catalog.md) | Gotowe zadania, parametry, przykładowe przepływy |
| [05. Instalacja agenta i rejestracja](docs/05-enrollment.md) | Skrypt, certyfikaty, rejestracja, rotacja, usuwanie |
| [06. Zdalny dostęp](docs/06-remote-access.md) | Connect, SSH, RDP, broker sesji, uprawnienia |
| [07. Instalacja i eksploatacja](docs/07-operations.md) | Compose, konfiguracja, backup, aktualizacje, awarie |
| [08. Roadmapa i backlog](docs/08-roadmap.md) | Etapy, zadania, testy, bramki wydania |
| [Przykładowy katalog](examples/task-catalog.yaml) | Makiety danych do projektowania, niewykonywalne |
| [Przykładowa konfiguracja](examples/config.example.yaml) | Proponowane ustawienia bez sekretów |

## Zasady

GUI jest podstawowym interfejsem, a szczegóły techniczne dostępne na żądanie. Agent nie jest uniwersalną powłoką na polecenie API, wykonuje ograniczone, typowane operacje. Automatyzacja i dostęp interaktywny mają osobne granice zaufania i uprawnienia. Domyślne wdrożenie jest odczytowe; zmiany, restart i zdalne sesje to odrębne możliwości wymagające autoryzacji. Agent jest jawnie instalowaną, widoczną usługą systemową z dokumentacją i możliwością usunięcia. Brak ukrytego dostępu, zbierania danych użytkowników, obchodzenia uwierzytelnienia i automatycznego otwierania portów.

Gałąź zawiera wyłącznie dokumentację i przykładowe dane, bez starego kodu. Commit zachowuje rodzica main dla identyfikowalności źródła audytu; nie jest orphan branch w sensie historii Git. Nie scalać mechanicznie z main, ponieważ usunęłoby to stare pliki. Decyzję o przejściu na Next podjąć po odbiorze. Repozytorium jest publiczne: nie publikować realnych adresów infrastruktury, danych klientów, kluczy, tokenów ani produkcyjnych konfiguracji.

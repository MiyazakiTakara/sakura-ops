# Rejestr ADR

Architecture Decision Records dla SakuraOps Next. Rejestr zawiera decyzje, nie plany pracy; roadmapa jest w [08-roadmap.md](../08-roadmap.md), zasady pracy w [18-development-rules.md](../18-development-rules.md).

| ADR | Tytuł | Status |
| --- | --- | --- |
| [0001](0001-agent-hub-linux-browser-only.md) | Własny agent i hub, Linux-only, browser-only | Proposed |
| [0002](0002-stack-go-svelte-postgres.md) | Stack Go + Svelte + PostgreSQL | Proposed |

## Proces

- Statusy: **Proposed** → **Accepted** → **Superseded**. Status Accepted nadaje wyłącznie właściciel; wykonawca ani reviewer AI nie akceptuje ADR.
- Zmiana przyjętej decyzji wymaga nowego ADR, który wskazuje zastępowany, oraz zgody właściciela (AGENTS.md). Stary ADR dostaje status Superseded i link do następcy, treść zostaje.
- Numeracja kolejna, plik `NNNN-krotki-tytul.md`. ADR zawiera: status i datę, kontekst, decyzję, rozważone alternatywy, konsekwencje i ryzyka, kryteria akceptacji. Szczegóły techniczne zostają w dokumentach 01–25; ADR na nie wskazuje, nie kopiuje.
- ADR w statusie Proposed nie upoważnia do działania na infrastrukturze, produkcji, helperze ani SSH.

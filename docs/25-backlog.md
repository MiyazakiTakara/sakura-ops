# 25. Backlog implementacyjny

Źródło identyfikatorów: 08-roadmap.md (NEXT-001..077). Pakiety PR są logicznymi grupami podzadań; każde ma kryteria odbioru, zależności i testy. Nie są gotowymi funkcjami. Zasady AGENTS.md / 18-development-rules.md. Gałąź integracyjna v2/native-agent-blueprint, nie legacy main. Szczegółowa mapa GitHub Issues zostanie uzupełniona po ich utworzeniu i zweryfikowaniu. Bez automatycznego zamykania istniejących issues legacy.

## Kolejność

| Pakiet | NEXT | Cel | Zależność |
| --- | --- | --- | --- |
| P01 | 001–003 | ADR, stack spike, monorepo | owner review |
| P02 | 004–006 | Compose, CI i kontrakty | P01 |
| P03 | 007–008 | auth/RBAC | P02 |
| P04 | 009–012 | UI shell, skr, QA | P03 |
| P05 | 013–015 | hosts/profiles, Task v1, immutable plan | P04 |
| P06 | 016–017 | agent, gRPC/mTLS, issuer | P05 |
| P07 | 018–021 | worker, audit, lab security | P06 |
| P08 | 022–024 | fakty, enrollment, pakiet | P07 |
| P09 | 025–029 | installer, wizard, revoke, lab QA | P08 |
| P10 | 030–032 | adaptery D0, audyt | P09 |
| P11 | 033–036 | dashboard, katalog, skr, MVP QA | P10 |
| P12 | 037–041 | scheduler i SMTP | P11 |
| P13 | 042–045 | SSH WWW, oddzielny review | P12 |
| P14 | 046–051 | instalacja/restore/SSO/import/pilot | P13 |
| P15 | 052–054 | helper, okna, refresh | P14 |
| P16 | 055–059 | zmiany pakietów/usług i odbiór | P15 |
| P17 | 060–062 | opcjonalny relay (tylko po ADR) | P16 |
| P18 | 063–065 | reboot/config/agent upgrade | P16, P17 tylko gdy relay potrzebny |
| P19 | 066–069 | recovery i release readiness | P18 |
| P20 | 070–073 | kreator, Linux matrix, SFTP, terminal | P19 |
| P21 | 074–077 | integracje, Git, opcjonalny import, scale | P19, P20 wg zależności |

Pakiety nie są zgodą na automatyczny merge/production. Jeśli duży, dzielić na małe PR zachowując checklisty. Każdy P ma osobne gate z 08, 17 i 18. Po P11 odbiór MVP; P12+ poza MVP. P17 opcjonalny; nie blokuje rdzenia, jeśli owner odrzuci relay.

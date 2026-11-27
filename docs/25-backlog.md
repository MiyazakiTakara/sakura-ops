# 25. Backlog implementacyjny

Źródło identyfikatorów: 08-roadmap.md (NEXT-001..077). Pakiety PR są logicznymi grupami podzadań; każde ma kryteria odbioru, zależności i testy. Nie są gotowymi funkcjami. Zasady AGENTS.md / 18-development-rules.md. Gałąź integracyjna v2/native-agent-blueprint, nie legacy main. Utworzono i zweryfikowano 21 GitHub Issues (#23–#43), obejmujących wszystkie 77 podzadań NEXT. Każdy issue ma checklistę i kryteria odbioru. Bez automatycznego zamykania istniejących issues legacy.

## Kolejność

| Pakiet | NEXT | Cel | Zależność | GitHub Issue |
| --- | --- | --- | --- | --- |
| P01 | 001–003 | ADR, stack spike, monorepo | owner review | [#23](https://github.com/MiyazakiTakara/sakura-ops/issues/23) |
| P02 | 004–006 | Compose, CI i kontrakty | P01 | [#24](https://github.com/MiyazakiTakara/sakura-ops/issues/24) |
| P03 | 007–008 | auth/RBAC | P02 | [#25](https://github.com/MiyazakiTakara/sakura-ops/issues/25) |
| P04 | 009–012 | UI shell, skr, QA | P03 | [#26](https://github.com/MiyazakiTakara/sakura-ops/issues/26) |
| P05 | 013–015 | hosts/profiles, Task v1, immutable plan | P04 | [#27](https://github.com/MiyazakiTakara/sakura-ops/issues/27) |
| P06 | 016–017 | agent, gRPC/mTLS, issuer | P05 | [#28](https://github.com/MiyazakiTakara/sakura-ops/issues/28) |
| P07 | 018–021 | worker, audit, lab security | P06 | [#29](https://github.com/MiyazakiTakara/sakura-ops/issues/29) |
| P08 | 022–024 | fakty, enrollment, pakiet | P07 | [#30](https://github.com/MiyazakiTakara/sakura-ops/issues/30) |
| P09 | 025–029 | installer, wizard, revoke, lab QA | P08 | [#31](https://github.com/MiyazakiTakara/sakura-ops/issues/31) |
| P10 | 030–032 | adaptery D0, audyt | P09 | [#32](https://github.com/MiyazakiTakara/sakura-ops/issues/32) |
| P11 | 033–036 | dashboard, katalog, skr, MVP QA | P10 | [#33](https://github.com/MiyazakiTakara/sakura-ops/issues/33) |
| P12 | 037–041 | scheduler i SMTP | P11 | [#34](https://github.com/MiyazakiTakara/sakura-ops/issues/34) |
| P13 | 042–045 | SSH WWW, oddzielny review | P12 | [#35](https://github.com/MiyazakiTakara/sakura-ops/issues/35) |
| P14 | 046–051 | instalacja/restore/SSO/import/pilot | P13 | [#36](https://github.com/MiyazakiTakara/sakura-ops/issues/36) |
| P15 | 052–054 | helper, okna, refresh | P14 | [#37](https://github.com/MiyazakiTakara/sakura-ops/issues/37) |
| P16 | 055–059 | zmiany pakietów/usług i odbiór | P15 | [#38](https://github.com/MiyazakiTakara/sakura-ops/issues/38) |
| P17 | 060–062 | opcjonalny relay (tylko po ADR) | P16 | [#39](https://github.com/MiyazakiTakara/sakura-ops/issues/39) |
| P18 | 063–065 | reboot/config/agent upgrade | P16, P17 tylko gdy relay potrzebny | [#40](https://github.com/MiyazakiTakara/sakura-ops/issues/40) |
| P19 | 066–069 | recovery i release readiness | P18 | [#41](https://github.com/MiyazakiTakara/sakura-ops/issues/41) |
| P20 | 070–073 | kreator, Linux matrix, SFTP, terminal | P19 | [#42](https://github.com/MiyazakiTakara/sakura-ops/issues/42) |
| P21 | 074–077 | integracje, Git, opcjonalny import, scale | P19, P20 wg zależności | [#43](https://github.com/MiyazakiTakara/sakura-ops/issues/43) |

Pakiety nie są zgodą na automatyczny merge/production. Jeśli duży, dzielić na małe PR zachowując checklisty. Każdy P ma osobne gate z 08, 17 i 18. Po P11 odbiór MVP; P12+ poza MVP. P17 opcjonalny; nie blokuje rdzenia, jeśli owner odrzuci relay.

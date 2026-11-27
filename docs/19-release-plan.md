# 19. Wersjonowanie, wydania i operacje

Plan, nie uruchomiony pipeline. SemVer dla produktu, osobno protocol major/minor i action version. Manifest TaskVersion immutable niezależnie od release. Wersja testowa Next nie zastępuje starego main do akceptacji.

| Wydanie planowane | Gate | Zakres |
| --- | --- | --- |
| 0.1.0-alpha | 0–2 | hub/GUI/skr/auth, symulator, protokół i local lab |
| 0.2.0-alpha | 3 | enrollment i pierwszy agent Linux |
| 0.3.0-beta | 4 | MVP D0, kompatybilność lab i katalog |
| 0.4.0-beta | 5–6 | harmonogramy, opcjonalny SSO/SSH, read-only pilot po zgodzie |
| 0.5.0-rc | 7 | kontrolowane zmiany po osobnym odbiorze |
| 1.0.0-rc / 1.0.0 | 8 | operacyjna gotowość, restore, formalny odbiór |

Numery orientacyjne, żadnych tagów ani release dziś. Nie zwiększać wersji dla funkcji niedostarczonych. Minor backward compatible, breaking change = major lub nowy action version; N/N-1 tylko jeśli przetestowane. Nie nadpisywać publikowanych digestów. Release notes: zmiany, security, breaking, kompatybilność OS/agent/protocol, migracje i recovery.

## Artefakty i podpis

Go + statyczny Svelte build, obrazy przypięte digest, osobne binarki boundary components, agent podpisany pakiet deb/rpm lub sprawdzony installer/manifest, checksum, SBOM, provenance. amd64/arm64 tylko po testach. Release klucze odrębne od CA i command signer. Installer weryfikuje podpis i arch przed modyfikacją. Protected release/manual approval, żadnego deploy z PR; bez secrets w publicznych build logs. Reproducible na opisanych toolchains, skan CVE/licencji. Ograniczony dostęp do signing key, rotacja i awaryjne odwołanie.

## Upgrade/recovery

Backup przed zmianą: PostgreSQL + config + bezpieczne oddzielne klucze; test restore w izolacji, reconcile revoked identities, stale grants i nonce; wstrzymanie nowych runów i drain, migrator Goose, sprawdź health, lab agent, następnie canary. Rollback binary tylko gdy schema zgodna, inaczej przetestowany restore (jawne RPO). Agent upgrade manualnie podpisanym artefaktem do etapu 8; centralny upgrade tylko odrębny helper i canary, nie automatyczny na wszystkie hosty. Bez downgrade niezaufanym artefaktem. Przerwane mutacje unknown, bez ponowienia. Restore nie może reaktywować revoke. Runbook i RPO/RTO w 07.

## Przejście z legacy

Zostawić legacy main i PR #22 bez zmian. Nowa gałąź nie jest PR do starego main. Pilot równoległy w odczycie, brak podwójnych zmian; statyczny import hostów bez sekretów i autoaktywacji, każdy agent oddzielnie enroll. Po formalnym owner review: decyzja o tagu legacy, default branch i wydaniu; nie robić automatycznie. Produkcja odrębna autoryzacja.

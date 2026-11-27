# 20. Model zagrożeń

Projekt STRIDE/abuse-case dla własnego agenta i huba. Ryzyko opisane jakościowo; nie twierdzimy, że zabezpieczenia są już zaimplementowane.

## Aktywa i granice

Tożsamość użytkownika, cert agent, klucze CA/signer/release, plany/zgody, lokalny helper, host OS, DB/audit, task definitions, gateway SSH, backup. Internet/przeglądarka -> reverse proxy/API; API -> store/worker/signer; agent gateway -> mTLS agent; agent -> lokalny helper; SSH gateway -> jawny sshd; CI/release -> instalator. Każde przejście uwierzytelnione i ograniczone; nie ufać danym dlatego, że są z sieci wewnętrznej.

| Zagrożenie | Kontrola / dowód do odbioru |
| --- | --- |
| Kradzież invite / fałszywy agent | hash/TTL, atomic claim, CSR, niezależny fingerprint, revoke, test race |
| MITM / fake hub / spoof cert header | pinned trust, mTLS, proxy isolation, zła CA i DNS, brak insecure |
| Kompromitacja API/operatora | RBAC/scope, policy, odrębny signer/issuer, lokalna allowlista, explicit approval, deny default |
| Replay, TOCTOU, zmiana planu | podpis canonical, nonce journal, frozen targets i revision, recheck, stale tests |
| Arbitrary code/privilege escalation | compiled action contracts, no shell/plugin, non-root UID, helper peer creds i lokalna polityka, param tests |
| Malicious YAML/JSON | strict parser, bounded depth/size, duplicate reject, no remote ref, fuzz |
| Klon/kradzież klucza agent | unique identity, duplicate connection block, rotation, revoke i quarantine |
| SSRF/pivot i SSH abuse | host/service allowlist, verified pin, egress, no arbitrary port/forward, scoped WSS grant, revoke, OS auth |
| XSS/CSRF/session theft | escaping output/terminal, CSP, secure cookie, CSRF/origin, session revoke, no secret in localStorage |
| Dostęp do cudzych hostów | resource auth przed list/count, IDOR tests, scope recheck po zmianie grupy |
| Log/secret leakage | typed output allowlist, redaction, bounded buffers, write-only secrets, brak session recording |
| Supply chain | lockfile/pinned tools, signature verification, SBOM, private keys outside repo, release review |
| DoS / reconnect storm | bounded queues/frames, rate limits, jitter, timeout, backpressure, load test |
| Crash/backup rollback | durable journal/outbox/fencing, unknown state, no mutation retry, revoke reconcile |
| Privileged operator misuse | separation of duties, second person Z2, maintenance, canary, audit and review |
| Kompromitacja hosta/CA/signer | isolate/revoke, rotate trust, stop dispatch, incident review, restore gate |

Resztkowe ryzyko: root na hoście może przejąć tożsamość i sfałszować wyniki; panel nie jest systemem attestation. Przejęcie signera może umożliwić podpisanie akcji, lokalna polityka ogranicza zakres, nie daje gwarancji. Gateway widzi SSH strumień w pamięci, nie zapisuje treści. DB admin może zmieniać lokalny audit; zewnętrzny eksport później. Odtworzenie DB wymaga niezależnej reconciliacji revoke. Wykonanie na OS nie jest exactly-once przy crash. Agent outbound nie jest zgodą na ominięcie segmentacji.

## Reakcja i gate

Suspect host -> suspend, revoke cert/stream, block runs/SSH, analiza i ręczna rejestracja po oczyszczeniu; nie odzyskiwać dostępu automatycznie. Compromised hub -> stop scheduler/dispatcher/signer, suspend agenty lokalną procedurą, rotate credentials/trust, sprawdź audyt, restore w izolacji i zatwierdź ponownie scope. Klucze release odwołać zgodnie z polityką. Nie publikować realnych sekretów/IoC prywatnej sieci w publicznym repo. Każdy boundary ma test negatywny w 17; przed D1/Z/SSH i produkcją osobny security review. Nie akceptować ryzyka przez przemilczenie, udokumentować właściciela i decyzję.

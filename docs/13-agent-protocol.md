# 13. Protokół agent–hub v1 (projekt normatywny)

gRPC bidirectional streaming po HTTP/2 i mTLS. Osobny listener urządzeń; rejestracja i CSR przez HTTPS z 05-enrollment.md. Protobuf nie jest formatem podpisu ani językiem zadań. Kontrakt do spike w NEXT-002/006/016/017; konkretna implementacja i fixtures dopiero po review.

## Tożsamość i handshake

Agent generuje klucz prywatny lokalnie; cert clientAuth wiąże immutable agent UUID, serial i host UUID z rejestrem huba. Gateway weryfikuje zaufane CA, purpose, validity, serial/revoke, identity, nie ufa samemu polu host_id. Kontroler z pinned trust i weryfikacją DNS, brak insecure. Po TLS klient otwiera Connect(stream AgentMessage) returns (stream HubMessage); pierwszy frame Hello w 10 s (propozycja). Hello: agent_id, protocol major/minor, agent version, build digest, boot_id, connection_nonce, obsługiwane action/version/adapter capabilities, resume summary ograniczone. ServerHello: negotiated major/minor, connection_epoch, heartbeat interval, max bytes, server_time, status. Niekompatybilny = reject i czytelna przyczyna w rejestrze. Dwa równoczesne streamy dla jednego certu -> zamrożenie nowych zadań i review (nie wypieranie starego na ślepo). Epoch/fencing zapobiega delivery z nieaktualnego kanału.

## Komunikaty (pola wymagane, draft)

| Kierunek | Typ | Pola / semantyka |
| --- | --- | --- |
| A->H | Hello | identity, wersje, boot_id, nonce, capabilities |
| H->A | ServerHello | epoch, wersja, limity, server_time |
| A->H | Heartbeat | epoch, seq, boot_id, active_command_ids (bounded), sent_at |
| H->A | CommandOffer | envelope canonical bytes, signature, key_id, epoch |
| A->H | CommandAck | command_id, accepted/rejected/duplicate, code; nie wynik |
| A->H | CommandEvent | command_id, event_seq, kind, bounded typed metadata |
| A->H | CommandResult | command_id, event_seq, outcome, typed result, changed/unknown, observed_at |
| H->A | CancelRequest | podpisane scope command_id, TTL/nonce, reason code; best effort |
| H->A | ResultAck | command_id, highest_committed_seq; dopiero po durable zapisie |
| H->A | Drain | brak nowych offerów, dokończ zgodnie z polityką |
| obie | ProtocolError | stable code, correlation_id, bez sekretów |

Message outer envelope ma version i oneof; unknown typ w negocjowanej wersji fail closed. Max frame 256 KiB, bounded queues, liczby i stringi walidowane. Limit wyników/logów na akcję, event_seq monotonic per command, powtórzone eventy dedup unique(command_id, seq). UTC, daty w protobuf Timestamp lub uzgodnionym RFC3339; TTL i clock skew max proponowane 30 s, niesynchronizowany czas blokuje wykonanie. Heartbeat 30 s z jitter, offline po 90 s od uwierzytelnionego kontaktu; reconnect 2–60 s jitter. Zmiana wersji i polityki unieważnia plan.

## Podpisane zlecenie

Signed bytes: canonical JSON wg kontraktu z 10/14, ograniczone typy liczb i dokładna wersja; SHA256 digest, podpis asymetryczny ze standardowej biblioteki (proponowane Ed25519; FIPS/środowisko może wymagać innego ADR), key_id i domain separator. Nie używać wspólnego HMAC klucza na wszystkich hostach. Signer odrębny od API/issuer; agent posiada wyłącznie przypięte klucze publiczne. Pola: protocol_version, command_id, run_id, step_id, agent_id, host_id, action_id/version, adapter_id/version, params, params_digest, plan_digest, policy_revision, issued_at, expires_at, nonce, approval_class, timeout_seconds. Dla mutacji dokładny effect_plan_digest i zatwierdzone ograniczenia. Agent ponownie waliduje wszystkie pola, capabilities, lokalną politykę i signature, nie wykonuje dowolnego kodu. Dane podpisane nie mogą mieć dodatkowych pól interpretowanych przez helper. Cancel podpisany odrębnie, nie pozwala uruchomić nowej akcji.

## Journal, retry i awarie

Hub tworzy trwały run/step/command z unikalnym ID i lease. Agent przed ack accepted atomowo utrwala ID/nonce/manifest hash oraz state accepted. Ta sama komenda + hash -> zwróć zapamiętany stan/wynik; identyczne ID z innym hashem -> reject tamper. Expiry przed startem = expired; po starcie deadline i lokalny timeout. Utrwal running przed efektem, końcowy wynik przed wysłaniem. Crash w trakcie mutacji = unknown, blokada kolejnych zmian do reconciliation; nie claimować exactly-once na OS. Ponów ten sam command ID tylko jako dostarczenie/odczyt statusu, nie nowe wykonanie. Nowa próba D0 wymaga odrębnego planu/polityki retry. ResultAck zwalnia bounded spool po durable commit, tombstone i nonce zostają co najmniej przez maks. okres akceptacji podpisu i recovery. Nie usuwaj niepotwierdzonych wyników po cichu. Wygaśnięcie/revoke/wymiana certyfikatu zamyka stream, nie ufa starym epochs; brak heartbeat nie dowodzi wyłączenia VM.

## Testy

Fixtures dobrych/złych canonical bytes, zła CA/serial/ID, expiry/clock skew, replay, ten sam ID inny hash, reconnect i result ack lost, journal crash w każdym przejściu, dwa streamy, fencing, oversized frame, backpressure, nieznany typ, niespójny adapter, cancel/timeout, odwołanie cert i rotacja kluczy. Test protokołu na dwóch niezależnych procesach, potem VM; brak produkcyjnych hostów. Go race/fuzz parsera i protokołu. Protobuf field numbers i wire spec zatwierdzić i wygenerować w etapie 006, nie traktować tabeli jako gotowego .proto.

# 22. Model danych i transakcje (PostgreSQL)

Projekt przed migracjami Goose/sqlc. UUID dla zasobów, timestamptz UTC, bigint dla event seq/revisions, text/check constraints dla stanów, JSONB tylko dla wersjonowanych, walidowanych payloadów. Niesekretne identyfikatory case-normalized z unikalnością. Nie używać JSONB zamiast relacji bezpieczeństwa. Foreign keys, indeksy i constraints są obowiązkowe; DB role ograniczone do procesu. Usunięcia to retire/revoke, nie cascade historii.

| Tabela | Pola istotne / klucze |
| --- | --- |
| users | id, username_norm unique, password_hash nullable dla OIDC, active, auth_rev, created_at |
| external_identities | issuer, subject unique razem, user_id FK, explicit linked_at |
| roles, user_role_scopes | role, user_id, scope group/global, permission, rev; jawne global |
| sessions, api_tokens | hash identyfikatora, user_id, scope, expiry, revoked_at, auth_rev; token plaintext nigdy |
| hosts | id, alias_norm unique, env, state, description, metadata_rev, policy_rev, profile_version_id FK, retired_at |
| groups, host_groups, host_tags | group_id/host_id unique, tags normalized; membership revision |
| profiles, profile_versions | slug unique, immutable version + digest unique, config/policy/task refs; pinned |
| agent_identities, agent_certificates | agent_id/host_id, key fingerprint, serial unique, validity, status/revoked_at, last_seen, protocol, epoch, boot_id |
| enrollment_invites, enrollment_attempts | token hash unique, intended host/profile, expiry, state, attempt CSR fingerprint/claim, actor; atomic claim |
| host_observations | host_id, source_run/step, kind, observed_at, ttl, payload_schema/version, validated payload, status; latest index |
| tasks, task_versions | slug unique, source builtin/custom, immutable version unique(task_id,version), definition digest, canonical JSON, contract refs, published_at |
| plans, plan_targets | task_version, inputs digest, actor, policy_rev, expiry, status, risk; target host/agent/profile/adapter/identity snapshot unique(plan,host) |
| approvals | plan_id, actor, class, timestamp, exact digest, expiry/revoked, unique(plan,actor,class), requester separation in transaction |
| runs, host_runs, step_runs | plan_id, idempotency key scoped actor unique, state, timestamps, host/step IDs, unique(run,host)/unique(host_run,step) |
| commands, command_events | command_id unique, step_run, signed digest, nonce unique, status, lease/fence, deadlines; unique(command_id,seq) |
| host_locks, outbox | host_id unique, lease/fence; durable delivery rows/status, next_attempt, bounded retry |
| schedules, occurrences, grants | rule/timezone, task version, scope, revision, owner, expiry/revoke; unique(schedule_id,scheduled_at) |
| ssh_sessions, ssh_session_events | actor, host/account/service, grant hash/TTL/revoke, policy_rev, start/end/state, no transcript/password |
| audit_events | id, seq, timestamp, actor/type, action, resource ID, request ID, result, sanitized details; append-only for API |
| instance_settings, secret_refs | singleton/versioned typed config, reference to external protected secret; no plaintext key material |

Indexes: hosts state/alias; membership both directions; observations(host,kind,observed_at desc); task/run created; run state/queued_at; command next_attempt/lease; cert serial/revoke/expiry; schedule next_at; audit actor/resource/time. Cursor pagination with stable (timestamp,id), scope before count. Partition audit/events only after measured need, retention respects active runs/nonce/recovery.

## Transactions and invariants

Invite claim locks row; unused + not expired + matching CSR, exactly one attempt. Revoke/approve races resolved by same lock and recheck before issuance; cert created only for approved attempt and matching agent. No implicit resurrection. Plan creation snapshots permissions, membership, capabilities, task digest and policy. Approval binds exact plan and second actor. Run creation validates expiry/revisions/scope, unique idempotency; same key/different body = 409. Outbox and run transition commit together. Worker claims queued row with row lock/lease and increments fence; late worker cannot overwrite newer fence. Per-host lock blocks overlapping changes (MVP conservative single active run per host); durable results and event seq dedup. Unknown state never becomes success based on heartbeat. Cancel request distinct from final state. Schedule occurrence unique, no missed mutation catchup.

Policy/profile/membership change increments policy revision and invalidates affected plans/grants/SSH; update endpoint If-Match for editable entities. Audit writes same transaction as mutation when possible; failure in audit for sensitive mutation fails closed. No task version update after publish. Retention does not delete references needed for audit or recovery. Migration role separate; fresh and upgrade tests, no destructive migration without backup/restore plan. Schema SQL and codegen in NEXT-004/013/014/015/018; no DB created now.

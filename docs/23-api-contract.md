# 23. REST/OpenAPI contract v1 (draft)

Base /api/v1, JSON UTF-8, strict request schemas and bounded sizes. REST for GUI and skr, SSE for live events, gRPC agent separate, WSS only SSH. No raw shell route. Concrete OpenAPI YAML and generated clients during NEXT-006; table below is normative planning scope, not running endpoints.

## Conventions

Opaque UUID, RFC3339 UTC, integer revisions, cursor pagination `{items,next_cursor}`, sort stable. Error `{code,message,request_id,field_errors:[{path,code,message}]}`; no traceback or cross-scope data. 400 malformed, 401 unauthenticated, 403 forbidden (404 for hidden objects), 409 stale/idempotency conflict, 422 invalid/unsupported, 429 limited, 503 dependency unavailable. Mutations check auth/scope/CSRF for browser, scoped token for CLI, audit; Idempotency-Key for run/create/claim where specified, same key and payload returns existing resource, mismatch 409. ETag/If-Match for mutable resources, immutable published versions. Return 202 only for accepted async job, never success of operation.

| Method / path | Contract / response |
| --- | --- |
| POST /auth/login | username/password, generic invalid, session cookie + CSRF |
| POST /auth/logout; GET /auth/me | revoke; identity/scopes/available features |
| GET /system/status; GET/PATCH /settings | redacted status; validated config+revision |
| GET/POST /users; PATCH /users/:id; roles/tokens | admin scoped, auth revision and revoke |
| GET/POST /hosts; GET/PATCH /hosts/:id | filter/paginate; metadata/env/profile ref, If-Match |
| POST /hosts/:id/suspend, /retire | explicit reason, revoke effects; no history delete |
| GET /hosts/:id/observations | kind, observed_at, ttl, source and status |
| GET/POST /groups; PATCH /groups/:id | membership preview and revision; scope recheck |
| GET/POST /profiles; POST /profiles/:id/versions | draft and immutable publish, effective diff |
| POST /enrollments; GET /enrollments/:id | intended host+diagnostic profile, token shown once, expiry |
| POST /enrollments/:id/claim, /approve, /revoke | claim restricted token/CSR, approval user/fingerprint, revoke |
| POST /enrollments/:id/renew | device authenticated; approved identity only |
| GET /tasks; GET /tasks/:id/versions/:version | catalog/status/inputs/contract refs |
| POST /tasks/validate; POST /tasks/drafts | YAML/JSON content, normalized draft errors, no run |
| POST /tasks/:id/versions | publish authorized immutable version/digest |
| POST /plans; GET /plans/:id | task ref, inputs, explicit hosts/group; resolved target snapshot, risk/blockers/expiry |
| POST /plans/:id/approve | exact plan digest, reason, reauth, approval object |
| POST /runs; GET /runs; GET /runs/:id | plan_id + idempotency, 202; history/detail |
| POST /runs/:id/cancel | request cancel, 202, eventual outcome |
| GET /runs/:id/events | SSE with id and Last-Event-ID, scope recheck; replay from DB or resync code |
| GET /runs/:id/results | per-host/step typed results, paging |
| GET/POST/PATCH /schedules; /pause,/resume | after MVP; task/version/grant/rule/timezone |
| POST /console/execute; /console/complete | bounded command/mode, result and allowed suggestions; same services, no PTY |
| GET/PUT /workspace/:file_id | safe workspace IDs/ETag, max bytes, no arbitrary filesystem |
| GET /audit | scoped redacted history |
| POST /sessions/plan, /:id/approve, /:id/connect, /:id/revoke; GET /sessions | later SSH, grant bound to user/host/account, WSS same-origin |
 
## Przykład planu (fikcyjny)

POST /plans: task_id=system-health, version=1, inputs={}, targets={host_ids:[UUID]}. Group selector mutually exclusive with host_ids. Response plan_id, digest, state=ready/blocked/pending_approval, expires_at, risk, effects, targets [{host_id, alias, capability_status, profile_version, adapter_versions, blockers}], approval_required. Nie przyjmować role/risk/sudo/adapter from user. Zmiana target/inputs wymaga nowego planu. POST /runs z plan_id nie zmienia payloadu.

SSE: event id persisted, event type run.status/host.status/step.status/step.result, JSON sanitized; reconnect Last-Event-ID, brak gwarancji dostawy dokładnie raz, UI dedup. Heartbeat comments, bounds, reauth/revoke, if gap beyond retention request snapshot. Nie streamować sekretów. Status offline jest obserwacją kontaktu agenta, nie awarią OS.

## Kontrakty bezpieczeństwa

API i skr używają tych samych services, nie duplikują reguł. Parser komendy nie jest uprawnieniem. CORS same-origin; no CSRF bypass przez content type. Enrollment i agent mają osobne auth i listener, user nie podaje cert identity jako header. SSH grant jednorazowy i bez URL/token logs, autoryzacja host key i OS osobno. Każdy endpoint ma test pozytywny i cross-scope, 401/403/409/422, schema/limits i audit. Nie wystawiać endpointów niezaimplementowanych.

Implemented subset as OpenAPI: `api/openapi/sakuraops.v1.yaml` (see docs/dev/contracts.md).

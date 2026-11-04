# SDD ledger — plan: docs/superpowers/plans/2026-11-03-hasura-kestra-demo.md
Ruling: No Git exists; implement directly in authorized demo/ and keep ledger here rather than Git-dependent helpers/worktrees. No commits possible.
Pre-flight: Tasks 1→2 share JWT identity and schema; Tasks 2→3 share persisted outbox event; Tasks 3→4 share webhook and execution status. Use DB outbox as source of truth for callback.
Task 1: auth tests RED undefined Auth/Identity → GREEN. Config and schema created, service startup in progress.
Task 2: store tests RED undefined Store → compiled, DB tests pending; HTTP tests RED undefined New → GREEN.
Task 1 Ruling: PostgreSQL healthcheck must use TCP. Socket pg_isready sees temporary init server and allowed Hasura to start before final DB restart; changed -h 127.0.0.1 and added Hasura restart policy. Evidence: connection refused at initial start, then successful psql and restart.
Ruling: host Go build needs -buildvcs=false because this checkout has no usable VCS metadata; container build has no .git.
Task 3 Ruling: Kestra 2.0.8 requires Basic Auth; configure local credentials and scoped client authentication (test RED missing credentials). Secret_* values require base64; encode in container entrypoint from plain env, keeping source YAML secret() references. Official docs checked.
Task 2: complete — docker compose run --rm test passed including PostgreSQL tests (concurrent delivery, stale version, workspace and draft constraints).
Task 3 Ruling: Kestra /flows/import only accepts multipart; YAML was routed as namespace named import and rejected. Use POST /flows/dxos.demo?delete=false with YAML for idempotent upsert. Regression test RED wrong endpoint → fix; no namespace renaming needed.
Task 3: workflows imported, bootstrap repeated successfully. Kestra 2.0.8 validator confirms options.timeout.readIdleTimeout and typed options.retry; fixed configuration to actual schema.
Review: no critical/important findings; fixed draft-only replay instruction and stale fixture race noted by reviewer.
Task 4: smoke initially caught Hasura ping before connection_ack; regression test RED, implementing handshake ping/pong handling.
Task 4 Ruling: Runtime-role test exposed catalog invoker permissions after Hasura trigger installation. Bootstrap now applies SECURITY DEFINER + fixed search_path to the exact generated trigger, not broad catalog privileges for API. Store event serialization uses transaction advisory lock instead of outbox FOR UPDATE to avoid granting UPDATE on immutable outbox. Runtime regression test exercises real api_runtime role.
Task 4 Ruling supersedes SECURITY DEFINER approach: pg_run_sql reload recreated Hasura trigger and cleared function settings (confirmed pg_proc.prosecdef=false). Stable minimal grant is schema USAGE + INSERT on five event_log columns used by Hasura insert_event_log; no catalog read/update/delete or metadata rights. Kept advisory event lock so no outbox UPDATE needed.
Task 1: complete — Hasura query/mutation/permissions and WebSocket authenticated subscription verified via real engine.
Task 3: complete — repeat bootstrap, real webhook, HTTP retry success/failure, callback and replay all verified.
Task 4: complete — DB runtime suite, host race/vet/build, smoke current + fresh volumes pass. Expected retry10 and API-stopped execution fail; replay after API restart succeeds. Links/LF/SPDX checked. See demo/VERIFICATION.md.
Final ruling: no Git means no merge/push/worktree finish menu applies; leave authorized files in demo/ and user-facing stack running, remove only separately created fresh test stack.

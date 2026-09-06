# Reusable v0 admission

This feature replaces the documentation-only placeholder with the first honest
durable implementation. `workflow.toml` is the state authority. The admission
audit and first independent review found implementation defects that are
repaired with exact-source evidence. The second independent review found and
prompted correction of a false runtime lock-cardinality claim. The third
independent review found Claim reconciliation-value loss and heartbeat teardown
liveness defects; both are repaired with exact-source race and PostgreSQL
evidence. The fourth independent review found an idempotency snapshot race,
prevalidation payload allocation, and Worker reconciliation-value loss; all
three are repaired with exact-source race, PostgreSQL, external-consumer, and
performance evidence. The fifth independent review found heartbeat payload
amplification, ineffective key-update retention under a partial unique index,
and double/unbounded row payload copying; all three are repaired with
exact-source race, PostgreSQL, allocation, external-consumer, fuzz, and
performance evidence. The sixth independent review found acknowledgement
reconciliation loss, an unenforced pgx result-mode dependency, an unsafe
Worker renewal interval,
unsupported caller transaction isolation, and incomplete stored-row
validation. All five are repaired at exact source `1fc2a7b` with unit/race,
PostgreSQL 17 mode and interleaving, allocation, fuzz, external-consumer, and
performance evidence. The first database run also caught an incomplete
binary-result OID map; that attempt is recorded as failed and superseded by the
final exact-source run. Admission remains active until two fresh
orchestrator-owned reviews admit the final candidate.

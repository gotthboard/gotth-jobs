# Reusable v0 admission

This feature replaces the documentation-only placeholder with the first honest
durable implementation. `workflow.toml` is the state authority. The admission
audit and first independent review found implementation defects that are
repaired with exact-source evidence. The second independent review found and
prompted correction of a false runtime lock-cardinality claim. The third
independent review found Claim reconciliation-value loss and heartbeat teardown
liveness defects; both are repaired with exact-source race and PostgreSQL
evidence. Admission remains active until two fresh orchestrator-owned reviews
pass on the final candidate.

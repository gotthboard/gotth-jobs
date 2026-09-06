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
Judge 6 repair's exact-source run. Admission remains active until two fresh
orchestrator-owned reviews admit the final candidate.

The seventh independent review found that cancellation could still erase an
unknown Heartbeat outcome, non-payload row fields allocated before validation,
the integration reset trusted any PostgreSQL 17 URL, nullable text presence
was collapsed, and the Judge 6 workflow event was absent. All five are
repaired at exact source `9711e2b` with deterministic heartbeat races, exact
borrowed-source bounds, fail-closed server-verified reset identity, complete
lease/nullability shape checks, and corrected append-only workflow history.
Fresh final reviews remain orchestrator-owned.

The eighth independent review rejected candidate `2486b47`: scalar lease
classification allocated an untrusted state before validation and treated
unknown state as lease loss, `Counts` omitted unknown or NULL states, and the
prior evidence did not retain literal commands or external-consumer source.
The implementation defects are repaired at `4ec1970` with bounded
borrowed-state scanning, exact five-state classification, count completeness,
malformed PostgreSQL/allocation tests, and a hashed exact-source `set -x`
transcript plus retained consumer source. Admission remains active pending two
fresh orchestrator-owned reviews of the final candidate.

The ninth independent review rejected candidate `8213b6d` because Worker
normalized and redacted the complete handler error string before enforcing its
4 KiB persistence limit. The repair at exact source `b54c0fc` calls `Error()`
once, caps the source first, bounds invalid UTF-8/NUL expansion, and reserves
the ellipsis within the final limit. Expected-red and exact-source allocation,
race, coverage, fuzz, focused performance, and graph evidence are retained;
PostgreSQL was not rerun because no database path changed. Fresh final reviews
remain orchestrator-owned.

The tenth independent review rejected candidate `a6900a1` because legacy
`panicnil=1` behavior allowed `panic(nil)` to be completed as success and a
custom Store could induce source-sized allocation by returning a large unknown
state. The repair at exact source `389915b` uses an explicit normal-completion
flag for every panic unwind and constant classified text for unknown state.
Expected-red, exact-source race, repeat, allocation, and coverage evidence are
retained. Fresh final reviews remain orchestrator-owned.

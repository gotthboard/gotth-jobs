# Coverage map

| Requirement | Design/spec | Implementation | Tests | Status |
| --- | --- | --- | --- | --- |
| JOB-001 | architecture/data model | `migrations`, values, bounded borrowed-byte scanners and row validation | exact field edges, NULL/presence/state shapes, ownership/allocation, PostgreSQL modes | covered |
| JOB-002 | architecture/enqueue | Read Committed-only `EnqueueTx` | isolation unit/PostgreSQL and rollback integration | covered |
| JOB-003 | spec/enqueue | one-snapshot fingerprint and non-partial unique key | delete/update replacement interleavings, NULL keys, sequential/concurrent PostgreSQL | covered |
| JOB-004 | architecture/claim | `Claim` | claim unit and concurrent PostgreSQL | covered |
| JOB-005 | architecture/fencing | token generation and claim | entropy unit, PostgreSQL lease replacement | covered |
| JOB-006 | architecture/fencing | scalar heartbeat/complete/fail and bounded lease-state classification | response shape/allocation, all five states, malformed state, stale-token PostgreSQL | covered |
| JOB-007 | state machine | retry policy and `Fail` | limits, retry, permanent, exhaustion | covered |
| JOB-008 | state machine | `Cancel` | idempotence, state conflict, PostgreSQL | covered |
| JOB-009 | operations | get/counts/list/redrive with complete known-state totals | unit, malformed unknown/NULL totals, pagination, PostgreSQL | covered |
| JOB-010 | worker | `Worker.Run` | half-lease boundary, success, retry, panic, cancellation, heartbeat, reconciliation, PostgreSQL | covered |
| JOB-011 | failure outcomes | `transact`, typed Claim/lease reconciliation errors | begin/body/rollback/commit and all Worker outcomes unit/external | covered |
| JOB-012 | trust boundary | API shape and docs | external-package compile and rollback | covered |
| JOB-013 | distribution | `LICENSE` and policy docs | license inventory | covered |

Statement coverage is 97.4% for both the unit and PostgreSQL integration
suites at exact implementation source `4ec1970`. Both bounded borrowed-byte
scanner methods, every shared job scan/validation entry point,
`classifyLease`, and `Counts` are 100% covered. Exact unrelated residual
blocks are retained in `coverage-gaps.log`; coverage is iteration evidence,
not the admission oracle.

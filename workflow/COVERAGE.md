# Coverage map

| Requirement | Design/spec | Implementation | Tests | Status |
| --- | --- | --- | --- | --- |
| JOB-001 | architecture/data model | `migrations`, values, bounded bytea scanner and row validation | values, NULL/timestamp boundaries, one-copy allocation, PostgreSQL modes | covered |
| JOB-002 | architecture/enqueue | Read Committed-only `EnqueueTx` | isolation unit/PostgreSQL and rollback integration | covered |
| JOB-003 | spec/enqueue | one-snapshot fingerprint and non-partial unique key | delete/update replacement interleavings, NULL keys, sequential/concurrent PostgreSQL | covered |
| JOB-004 | architecture/claim | `Claim` | claim unit and concurrent PostgreSQL | covered |
| JOB-005 | architecture/fencing | token generation and claim | entropy unit, PostgreSQL lease replacement | covered |
| JOB-006 | architecture/fencing | scalar heartbeat/complete/fail | response shape/allocation, failure paths, stale-token PostgreSQL | covered |
| JOB-007 | state machine | retry policy and `Fail` | limits, retry, permanent, exhaustion | covered |
| JOB-008 | state machine | `Cancel` | idempotence, state conflict, PostgreSQL | covered |
| JOB-009 | operations | get/counts/list/redrive | unit, pagination, PostgreSQL | covered |
| JOB-010 | worker | `Worker.Run` | half-lease boundary, success, retry, panic, cancellation, heartbeat, reconciliation, PostgreSQL | covered |
| JOB-011 | failure outcomes | `transact`, typed Claim/lease reconciliation errors | begin/body/rollback/commit and all Worker outcomes unit/external | covered |
| JOB-012 | trust boundary | API shape and docs | external-package compile and rollback | covered |
| JOB-013 | distribution | `LICENSE` and policy docs | license inventory | covered |

Statement coverage is 97.3% for both the clean-clone unit suite and PostgreSQL
integration at repair source `1fc2a7b`. EnqueueTx isolation, query-option
forcing, the bounded payload scanner, every shared job scan/validation entry
point, and all typed reconciliation methods are 100% covered. Every new Worker
acknowledgement branch is covered; `runAttempt` remains 97.8% because its
preexisting invalid retry-policy branch is unreachable through validated
`Worker.Run`. Exact residual blocks are recorded in verification evidence;
coverage is iteration evidence, not the admission oracle.

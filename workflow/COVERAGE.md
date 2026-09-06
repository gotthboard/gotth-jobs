# Coverage map

| Requirement | Design/spec | Implementation | Tests | Status |
| --- | --- | --- | --- | --- |
| JOB-001 | architecture/data model | `migrations`, values, bounded bytea scanner | values, migration, one-copy allocation, PostgreSQL | covered |
| JOB-002 | architecture/enqueue | `EnqueueTx` | rollback integration, public API | covered |
| JOB-003 | spec/enqueue | one-snapshot fingerprint and non-partial unique key | delete/update replacement interleavings, NULL keys, sequential/concurrent PostgreSQL | covered |
| JOB-004 | architecture/claim | `Claim` | claim unit and concurrent PostgreSQL | covered |
| JOB-005 | architecture/fencing | token generation and claim | entropy unit, PostgreSQL lease replacement | covered |
| JOB-006 | architecture/fencing | scalar heartbeat/complete/fail | response shape/allocation, failure paths, stale-token PostgreSQL | covered |
| JOB-007 | state machine | retry policy and `Fail` | limits, retry, permanent, exhaustion | covered |
| JOB-008 | state machine | `Cancel` | idempotence, state conflict, PostgreSQL | covered |
| JOB-009 | operations | get/counts/list/redrive | unit, pagination, PostgreSQL | covered |
| JOB-010 | worker | `Worker.Run` | success, retry, panic, cancellation, heartbeat, reconciliation, PostgreSQL | covered |
| JOB-011 | failure outcomes | `transact`, `ClaimReconciliationError` | begin/body/rollback/commit and Worker reconciliation unit/external | covered |
| JOB-012 | trust boundary | API shape and docs | external-package compile and rollback | covered |
| JOB-013 | distribution | `LICENSE` and policy docs | license inventory | covered |

Statement coverage is 97.0% for both the clean-clone unit suite and PostgreSQL
integration at current repair source `53cf140`. Heartbeat, its Worker caller,
the bounded payload scanner, and every shared job scan entry point are 100%
covered. Exact residual branches are recorded in `docs/verification.md`;
coverage is iteration evidence, not the admission oracle.

# Coverage map

| Requirement | Design/spec | Implementation | Tests | Status |
| --- | --- | --- | --- | --- |
| JOB-001 | architecture/data model | `migrations`, values, rows | values, migration, PostgreSQL | covered |
| JOB-002 | architecture/enqueue | `EnqueueTx` | rollback integration, public API | covered |
| JOB-003 | spec/enqueue | one-snapshot fingerprint and enqueue | replacement interleaving, row retention, sequential/concurrent PostgreSQL | covered |
| JOB-004 | architecture/claim | `Claim` | claim unit and concurrent PostgreSQL | covered |
| JOB-005 | architecture/fencing | token generation and claim | entropy unit, PostgreSQL lease replacement | covered |
| JOB-006 | architecture/fencing | heartbeat/complete/fail | failure paths and stale-token PostgreSQL | covered |
| JOB-007 | state machine | retry policy and `Fail` | limits, retry, permanent, exhaustion | covered |
| JOB-008 | state machine | `Cancel` | idempotence, state conflict, PostgreSQL | covered |
| JOB-009 | operations | get/counts/list/redrive | unit, pagination, PostgreSQL | covered |
| JOB-010 | worker | `Worker.Run` | success, retry, panic, cancellation, heartbeat, reconciliation, PostgreSQL | covered |
| JOB-011 | failure outcomes | `transact`, `ClaimReconciliationError` | begin/body/rollback/commit and Worker reconciliation unit/external | covered |
| JOB-012 | trust boundary | API shape and docs | external-package compile and rollback | covered |
| JOB-013 | distribution | `LICENSE` and policy docs | license inventory | covered |

Statement coverage is 96.9% for both the clean-clone unit suite and PostgreSQL
integration at current repair source `62d565a`. Enqueue's two public entry
points, preparation, combined fingerprint/job scanners, and every new Worker
reconciliation-error method are 100% covered. The shared PostgreSQL timestamp
predicate remains 100% covered. Exact residual branches are recorded in
`docs/verification.md`; coverage is iteration evidence, not the admission
oracle.

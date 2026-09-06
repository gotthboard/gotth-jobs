# Coverage map

| Requirement | Design/spec | Implementation | Tests | Status |
| --- | --- | --- | --- | --- |
| JOB-001 | architecture/data model | `migrations`, values, rows | values, migration, PostgreSQL | covered |
| JOB-002 | architecture/enqueue | `EnqueueTx` | rollback integration, public API | covered |
| JOB-003 | spec/enqueue | fingerprint and enqueue | unit, sequential and concurrent PostgreSQL | covered |
| JOB-004 | architecture/claim | `Claim` | claim unit and concurrent PostgreSQL | covered |
| JOB-005 | architecture/fencing | token generation and claim | entropy unit, PostgreSQL lease replacement | covered |
| JOB-006 | architecture/fencing | heartbeat/complete/fail | failure paths and stale-token PostgreSQL | covered |
| JOB-007 | state machine | retry policy and `Fail` | limits, retry, permanent, exhaustion | covered |
| JOB-008 | state machine | `Cancel` | idempotence, state conflict, PostgreSQL | covered |
| JOB-009 | operations | get/counts/list/redrive | unit, pagination, PostgreSQL | covered |
| JOB-010 | worker | `Worker.Run` | success, retry, panic, cancellation, heartbeat, PostgreSQL | covered |
| JOB-011 | failure outcomes | `transact` | begin/body/rollback/commit failure unit | covered |
| JOB-012 | trust boundary | API shape and docs | external-package compile and rollback | covered |
| JOB-013 | distribution | `LICENSE` and policy docs | license inventory | covered |

Statement coverage is 96.3% for the clean-clone unit suite and 96.5% with
PostgreSQL integration at repair source `72c6223`. Both repaired defect paths
have direct boundary tests. Exact residual branches are recorded in
`docs/verification.md`; coverage is iteration evidence, not the admission
oracle.

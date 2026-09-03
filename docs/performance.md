# Performance admission

## Decision

No optimization was introduced and no speedup is claimed. The direct
PostgreSQL mechanism is admitted provisionally because its measured behavior
is bounded enough for the first consumer and its cost remains visible: one
short transaction per mutation, one returned payload copy, and an indexed
`FOR UPDATE SKIP LOCKED` scan whose work grows with eligible and locked rows.

Because there is no baseline/candidate optimization comparison, hotspot share
`P`, hotspot speedup `S_hotspot`, and the Amdahl prediction
`1 / ((1-P) + P/S_hotspot)` are N/A. Inventing numeric substitutions without a
candidate would be dishonest. A future optimization must supply matched
baseline and candidate results before making any overall speedup claim.

## Environment

- Host: designated `development` validation host, Linux amd64.
- PostgreSQL: 17.10 from pinned image digest
  `sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d`.
- Docker Engine: 29.7.1.
- Remote integration compiler: Go 1.26.5-X:nodwarf5; the canonical local
  format/vet/unit/race/coverage gates use Go 1.26.6-X:nodwarf5.
- Connection: loopback published disposable container port, no network backend
  and no live database.

## Results

Claim latency measures only `Claim`. Throughput uses the full measured loop,
including `Complete` for nonempty workloads. Fixtures and enqueue setup are
outside the measurement.

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 554.037 us | 937.473 us | 1.120165 ms | 1667.38 ops/s |
| 100-job small backlog, empty payload | 50 | 3.642659 ms | 4.017174 ms | 5.475084 ms | 142.42 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 4.254707 ms | 7.664852 ms | 7.838505 ms | 119.50 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 6.120212 ms | 8.223411 ms | 8.876519 ms | 75.58 ops/s |
| 100-row locked prefix | 1 | 21.387997 ms | N/A | N/A | N/A |

The pathological sample proves that `SKIP LOCKED` can walk past a locked
prefix; it is not a distribution and supports no percentile claim. CPU, I/O,
allocation, and execution-plan attribution were not separately profiled
because no optimization is proposed. The returned payload copy and database
round trips are the expected visible costs. Re-profile when a real consumer
supplies representative payloads, concurrency, retention, and service-level
objectives.

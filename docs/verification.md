# Verification status

Current Judge 6 repair verification:

- Independent Judge 6 rejected candidate
  `6126a751cfc70d47b53449927c1d1552f604ba4d` for Worker acknowledgement
  reconciliation loss, a pgx text-result allocation path, insufficient Worker
  renewal margin, unsupported `EnqueueTx` isolation, and incomplete stored-row
  payload/timestamp validation.
- Exact implementation repair source
  `1fc2a7b3db6edd354e7efa4154f87032866cb090` adds a secret-safe
  `LeaseReconciliationError`, forces described binary job results, reserves a
  half-lease renewal budget, requires Read Committed for `EnqueueTx`, and
  rejects NULL payloads and invalid stored timestamps.
- Expected-red unit evidence failed every new defect assertion. A pre-fix real
  pgx run showed payload-sized text decoding under Exec/SimpleProtocol and
  accepted Repeatable Read/Serializable `EnqueueTx` calls.
- The first post-repair PostgreSQL attempt failed at the maximum finite
  timestamp because an incomplete result-format map made unmapped timestamp
  columns text. That attempt is superseded, not claimed. The final source
  requests binary for every job-column OID and passes the maximum timestamp.
- Focused local tests passed 10 repetitions, followed by the complete package,
  vet, and integration-tag compilation with `GOMAXPROCS=2` and `-p=1`.
- Exact detached clean source on `development` passed format, vet, unit,
  build, full race, 50 affected race repeats, two 10-second fuzz targets, and
  coverage. Unit statement coverage is 97.3%.
- PostgreSQL 17.10 race and coverage integration passed at the pinned image
  digest. Integration coverage is 97.3%. Three real-pgx runs passed in all
  five supported default modes for maximum and oversized payloads. Ten
  race-instrumented repeats passed for caller isolation and both key-retention
  interleavings.
- The standalone external consumer test/build and complete performance matrix
  passed against exact source. Performance records the intentional cost of
  two DescribeExec protocol round trips per job-returning statement.
- EnqueueTx isolation, query option forcing, bounded payload scanning, shared
  row scan/validation, and typed reconciliation methods are 100% covered. All
  new Worker acknowledgement branches are covered. The exact remaining
  `runAttempt` gap is the preexisting invalid retry-policy branch unreachable
  through validated `Worker.Run`; other residual blocks are unrelated
  preexisting input/error branches.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

Exact commands, hashes, allocation results, setup details, and residual blocks
are recorded under
`workflow/features/reusable-v0-admission/evidence/verification.md`.

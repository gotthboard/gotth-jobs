# Verification status

Current Judge 7 repair verification:

- Independent Judge 7 rejected candidate
  `f7afde19020efdb201591e71156a8e35816cd43f` for Heartbeat unknown-outcome
  precedence, unbounded non-payload row scans, unsafe destructive integration
  reset, nullable-text presence loss, and missing Judge 6 workflow history.
- Exact implementation repair source
  `9711e2b00dc95ae3070090745d611b65eca686f3` gives unknown Heartbeat commits
  immediate post-join precedence, bounds every stored variable-width field
  through pgx borrowed bytes, preserves nullable presence and full Lease
  shape, and requires explicit plus server-verified destructive-test identity.
- Expected-red focused tests reproduced both cancellation races, all three
  present-empty nullable cases, a surviving non-running `Lease.JobID`, invalid
  fingerprint lengths, and direct pgx scan destinations. The reset guard test
  initially failed to compile because the required mechanism did not exist.
- Focused local package and integration-tag guard tests passed with
  `GOMAXPROCS=2` and `go test -p=1`; vet, formatting, integration compilation,
  and `git diff --check` passed.
- Exact detached clean source on `development` passed format, vet, unit,
  build, full race, 50 affected race repeats, two 10-second fuzz targets, and
  coverage. Unit statement coverage is 97.4%.
- PostgreSQL 17.10 race and coverage passed in a disposable database named
  exactly `gotth_jobs_test` with the required server-side comment and explicit
  opt-in. Integration coverage is 97.4%.
- Three real-pgx runs passed maximum/oversized payload plus oversized text and
  fingerprint allocation checks under all five supported default modes. Ten
  race repeats passed reset rejection, isolation, and both key-retention
  interleavings.
- Both scanner methods, `scanJob`, `scanJobWithFingerprint`, `scanJobRow`, and
  `validateStoredJob` are 100% covered. Both new Worker precedence races are
  covered. The exact remaining `runAttempt` gap is the preexisting invalid
  retry-policy branch unreachable through validated `Worker.Run`; other gaps
  are unrelated preexisting error/input branches.
- The standalone external consumer and full performance matrix passed against
  exact source. The disposable container was removed and the detached source
  remained clean.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

The integration runner deliberately requires all three values before DDL:

```text
GOTTH_JOBS_TEST_DATABASE_URL=<URL whose current_database() is gotth_jobs_test>
GOTTH_JOBS_ALLOW_DESTRUCTIVE_TEST_DATABASE_RESET=true
COMMENT ON DATABASE gotth_jobs_test IS 'gotth-jobs:dedicated-destructive-integration-test-v1';
```

The opt-in is separate from the URL. The test process queries PostgreSQL for
both the current database name and comment before reset; no URL spelling or
environment-variable name is treated as authorization.

Exact commands, hashes, allocation results, setup details, and residual blocks
are recorded under
`workflow/features/reusable-v0-admission/evidence/verification.md`.

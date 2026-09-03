CREATE TABLE public.gotth_jobs (
    id text PRIMARY KEY,
    queue text COLLATE "C" NOT NULL,
    kind text COLLATE "C" NOT NULL,
    payload bytea NOT NULL,
    idempotency_key text COLLATE "C",
    request_fingerprint bytea NOT NULL,
    state text NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL,
    available_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    lease_token text,
    lease_owner text,
    lease_until timestamptz,
    last_error text NOT NULL DEFAULT '',
    finished_at timestamptz,
    CONSTRAINT gotth_jobs_id_format CHECK (id ~ '^[0-9a-f]{32}$'),
    CONSTRAINT gotth_jobs_queue_length CHECK (octet_length(queue) BETWEEN 1 AND 128),
    CONSTRAINT gotth_jobs_kind_length CHECK (octet_length(kind) BETWEEN 1 AND 128),
    CONSTRAINT gotth_jobs_payload_length CHECK (octet_length(payload) <= 1048576),
    CONSTRAINT gotth_jobs_key_length CHECK (idempotency_key IS NULL OR octet_length(idempotency_key) BETWEEN 1 AND 256),
    CONSTRAINT gotth_jobs_fingerprint_length CHECK (octet_length(request_fingerprint) = 32),
    CONSTRAINT gotth_jobs_state CHECK (state IN ('pending', 'running', 'succeeded', 'dead', 'canceled')),
    CONSTRAINT gotth_jobs_attempts CHECK (attempts BETWEEN 0 AND 100),
    CONSTRAINT gotth_jobs_max_attempts CHECK (max_attempts BETWEEN 1 AND 100 AND attempts <= max_attempts),
    CONSTRAINT gotth_jobs_lease_shape CHECK (
        (state = 'running' AND lease_token IS NOT NULL AND lease_owner IS NOT NULL AND lease_until IS NOT NULL)
        OR
        (state <> 'running' AND lease_token IS NULL AND lease_owner IS NULL AND lease_until IS NULL)
    ),
    CONSTRAINT gotth_jobs_lease_token_length CHECK (lease_token IS NULL OR lease_token ~ '^[0-9a-f]{64}$'),
    CONSTRAINT gotth_jobs_worker_length CHECK (lease_owner IS NULL OR octet_length(lease_owner) BETWEEN 1 AND 256),
    CONSTRAINT gotth_jobs_failure_length CHECK (octet_length(last_error) <= 4096),
    CONSTRAINT gotth_jobs_finished_shape CHECK (
        (state IN ('succeeded', 'dead', 'canceled') AND finished_at IS NOT NULL)
        OR
        (state IN ('pending', 'running') AND finished_at IS NULL)
    )
);

CREATE UNIQUE INDEX gotth_jobs_idempotency
    ON public.gotth_jobs (queue, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX gotth_jobs_claim
    ON public.gotth_jobs (queue, available_at, created_at, id)
    WHERE state = 'pending';

CREATE INDEX gotth_jobs_expired_lease
    ON public.gotth_jobs (queue, lease_until, created_at, id)
    WHERE state = 'running';

CREATE INDEX gotth_jobs_dead
    ON public.gotth_jobs (queue, finished_at, id)
    WHERE state = 'dead';

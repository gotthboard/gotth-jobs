//go:build integration

package jobs_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gotthboard/gotth-jobs/pkg/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
)

func integrationRepository(t *testing.T) (*pgxpool.Pool, *jobs.PostgreSQL) {
	t.Helper()
	databaseURL := os.Getenv("GOTTH_JOBS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("GOTTH_JOBS_TEST_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	var versionText string
	if err := pool.QueryRow(ctx, "SHOW server_version_num").Scan(&versionText); err != nil {
		t.Fatalf("read PostgreSQL version: %v", err)
	}
	version, err := strconv.Atoi(versionText)
	if err != nil || version/10000 != 17 {
		t.Fatalf("PostgreSQL version = %q, %v; want 17.x", versionText, err)
	}
	var encoding string
	if err := pool.QueryRow(ctx, "SHOW server_encoding").Scan(&encoding); err != nil || encoding != "UTF8" {
		t.Fatalf("PostgreSQL encoding = %q, %v; want UTF8", encoding, err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS public.gotth_job_consumer; DROP TABLE IF EXISTS public.gotth_jobs"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	migration, err := fs.ReadFile(jobs.Migrations(), "000001_jobs.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	repository, err := jobs.NewPostgreSQL(pool)
	if err != nil {
		t.Fatalf("construct repository: %v", err)
	}
	return pool, repository
}

func TestPostgreSQLEnqueueIdempotencyAndConsumerRollback(t *testing.T) {
	pool, repository := integrationRepository(t)
	ctx := context.Background()
	request := jobs.EnqueueRequest{Queue: "default", Kind: "send", Payload: []byte("payload"), IdempotencyKey: "message-1", MaxAttempts: 3}
	created, wasCreated, err := repository.Enqueue(ctx, request)
	if err != nil || !wasCreated {
		t.Fatalf("Enqueue(create) = (%+v, %t, %v)", created, wasCreated, err)
	}
	duplicate, wasCreated, err := repository.Enqueue(ctx, request)
	if err != nil || wasCreated || duplicate.ID != created.ID {
		t.Fatalf("Enqueue(duplicate) = (%+v, %t, %v)", duplicate, wasCreated, err)
	}
	changed := request
	changed.Payload = []byte("different")
	if _, _, err := repository.Enqueue(ctx, changed); !errors.Is(err, jobs.ErrIdempotencyConflict) {
		t.Fatalf("Enqueue(conflict) = %v", err)
	}

	if _, err := pool.Exec(ctx, "CREATE TABLE public.gotth_job_consumer (id integer PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Exec(ctx, "INSERT INTO public.gotth_job_consumer (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	transactional := jobs.EnqueueRequest{Queue: "default", Kind: "transactional", MaxAttempts: 1}
	job, wasCreated, err := repository.EnqueueTx(ctx, transaction, transactional)
	if err != nil || !wasCreated || job.ID == "" {
		t.Fatalf("EnqueueTx() = (%+v, %t, %v)", job, wasCreated, err)
	}
	if err := transaction.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var markerCount, jobCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.gotth_job_consumer").Scan(&markerCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM public.gotth_jobs WHERE id = $1", job.ID).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if markerCount != 0 || jobCount != 0 {
		t.Fatalf("rollback left marker=%d job=%d", markerCount, jobCount)
	}
}

func TestPostgreSQLConcurrentIdempotentEnqueueCreatesOneJob(t *testing.T) {
	_, repository := integrationRepository(t)
	ctx := context.Background()
	request := jobs.EnqueueRequest{Queue: "default", Kind: "concurrent", Payload: []byte("same"), IdempotencyKey: "same-key", MaxAttempts: 1}
	start := make(chan struct{})
	type result struct {
		job     jobs.Job
		created bool
		err     error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			job, created, err := repository.Enqueue(ctx, request)
			results <- result{job: job, created: created, err: err}
		}()
	}
	close(start)
	var id string
	createdCount := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent enqueue: %v", result.err)
		}
		if id == "" {
			id = result.job.ID
		} else if result.job.ID != id {
			t.Fatalf("concurrent IDs = %q and %q", id, result.job.ID)
		}
		if result.created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created count = %d, want 1", createdCount)
	}
}

func TestPostgreSQLConcurrentClaimLeaseFencingRetryAndExhaustion(t *testing.T) {
	pool, repository := integrationRepository(t)
	ctx := context.Background()
	enqueued, _, err := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan struct {
		job jobs.Job
		err error
	}, 2)
	for worker := range 2 {
		go func() {
			<-start
			job, claimErr := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker-" + string(rune('1'+worker)), LeaseDuration: time.Second})
			results <- struct {
				job jobs.Job
				err error
			}{job: job, err: claimErr}
		}()
	}
	close(start)
	var first jobs.Job
	claimed, empty := 0, 0
	for range 2 {
		result := <-results
		switch {
		case result.err == nil:
			claimed++
			first = result.job
		case errors.Is(result.err, jobs.ErrNoJob):
			empty++
		default:
			t.Fatalf("concurrent claim error: %v", result.err)
		}
	}
	if claimed != 1 || empty != 1 || first.ID != enqueued.ID || first.Attempts != 1 {
		t.Fatalf("claim oracle claimed=%d empty=%d job=%+v", claimed, empty, first)
	}

	if _, err := pool.Exec(ctx, "UPDATE public.gotth_jobs SET lease_until = clock_timestamp() - interval '1 second' WHERE id = $1", first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker-2", LeaseDuration: time.Second})
	if err != nil || second.Attempts != 2 || second.Lease.Token == first.Lease.Token {
		t.Fatalf("reclaim = (%+v, %v)", second, err)
	}
	if second.LastError != "lease expired before acknowledgement" {
		t.Fatalf("reclaim last error = %q", second.LastError)
	}
	if _, err := repository.Complete(ctx, first.Lease); !errors.Is(err, jobs.ErrLeaseLost) {
		t.Fatalf("stale complete = %v", err)
	}
	if _, err := repository.Heartbeat(ctx, second.Lease, time.Second); err != nil {
		t.Fatalf("heartbeat = %v", err)
	}
	if retried, err := repository.Fail(ctx, second.Lease, jobs.Failure{Message: "temporary"}); err != nil || retried.State != jobs.StatePending {
		t.Fatalf("retry failure = (%+v, %v)", retried, err)
	}
	third, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker-3", LeaseDuration: time.Second})
	if err != nil || third.Attempts != 3 {
		t.Fatalf("third claim = (%+v, %v)", third, err)
	}
	dead, err := repository.Fail(ctx, third.Lease, jobs.Failure{Message: "final"})
	if err != nil || dead.State != jobs.StateDead || dead.Attempts != 3 {
		t.Fatalf("exhausted failure = (%+v, %v)", dead, err)
	}
}

func TestPostgreSQLCancellationDeadListingCountsAndRedrive(t *testing.T) {
	pool, repository := integrationRepository(t)
	ctx := context.Background()
	pending, _, _ := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "default", Kind: "pending", MaxAttempts: 1})
	canceled, err := repository.Cancel(ctx, pending.ID)
	if err != nil || canceled.State != jobs.StateCanceled {
		t.Fatalf("cancel pending = (%+v, %v)", canceled, err)
	}
	if again, err := repository.Cancel(ctx, pending.ID); err != nil || again.State != jobs.StateCanceled {
		t.Fatalf("cancel idempotent = (%+v, %v)", again, err)
	}

	var deadIDs []string
	for index := range 2 {
		job, _, enqueueErr := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "default", Kind: "dead", Payload: []byte{byte(index)}, MaxAttempts: 1})
		if enqueueErr != nil {
			t.Fatal(enqueueErr)
		}
		claimed, claimErr := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: time.Second})
		if claimErr != nil || claimed.ID != job.ID {
			t.Fatalf("claim dead candidate = (%+v, %v), want %s", claimed, claimErr, job.ID)
		}
		dead, failErr := repository.Fail(ctx, claimed.Lease, jobs.Failure{Message: "permanent", Permanent: true})
		if failErr != nil || dead.State != jobs.StateDead {
			t.Fatalf("dead failure = (%+v, %v)", dead, failErr)
		}
		deadIDs = append(deadIDs, dead.ID)
	}
	page, err := repository.ListDead(ctx, "default", nil, 1)
	if err != nil || len(page) != 1 {
		t.Fatalf("ListDead(first) = (%+v, %v)", page, err)
	}
	cursor := &jobs.DeadCursor{FinishedAt: page[0].FinishedAt, ID: page[0].ID}
	next, err := repository.ListDead(ctx, "default", cursor, 2)
	if err != nil || len(next) != 1 || next[0].ID == page[0].ID {
		t.Fatalf("ListDead(next) = (%+v, %v)", next, err)
	}
	redriven, err := repository.Redrive(ctx, deadIDs[0])
	if err != nil || redriven.State != jobs.StatePending || redriven.Attempts != 0 {
		t.Fatalf("Redrive() = (%+v, %v)", redriven, err)
	}
	counts, err := repository.Counts(ctx, "default")
	if err != nil || counts.Pending != 1 || counts.Dead != 1 || counts.Canceled != 1 {
		t.Fatalf("Counts() = (%+v, %v)", counts, err)
	}

	if _, err := pool.Exec(ctx, "UPDATE public.gotth_jobs SET available_at = clock_timestamp() + interval '1 hour' WHERE id = $1", redriven.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: time.Second}); !errors.Is(err, jobs.ErrNoJob) {
		t.Fatalf("future job claim = %v", err)
	}
}

type cancelingStore struct {
	*jobs.PostgreSQL
	cancel context.CancelFunc
}

func (store cancelingStore) Complete(ctx context.Context, lease jobs.Lease) (jobs.Job, error) {
	job, err := store.PostgreSQL.Complete(ctx, lease)
	if err == nil {
		store.cancel()
	}
	return job, err
}

func TestPostgreSQLWorkerRetriesThenCompletes(t *testing.T) {
	_, repository := integrationRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "default", Kind: "worker", MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	worker := jobs.Worker{
		Store: cancelingStore{PostgreSQL: repository, cancel: cancel},
		Queue: "default", WorkerID: "worker", LeaseDuration: time.Second,
		HeartbeatInterval: 10 * time.Millisecond, PollInterval: time.Millisecond,
		RetryPolicy: jobs.RetryPolicy{},
		Handler: func(context.Context, jobs.Job) error {
			if calls.Add(1) == 1 {
				return errors.New("retry")
			}
			return nil
		},
	}
	if err := worker.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Worker.Run() = %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("handler calls = %d", calls.Load())
	}
}

func TestPostgreSQLExhaustedExpiredLeaseBecomesDead(t *testing.T) {
	pool, repository := integrationRepository(t)
	ctx := context.Background()
	job, _, _ := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "default", Kind: "expire", MaxAttempts: 1})
	claimed, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE public.gotth_jobs SET lease_until = clock_timestamp() - interval '1 second' WHERE id = $1", claimed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: time.Second}); !errors.Is(err, jobs.ErrNoJob) {
		t.Fatalf("Claim(exhausted) = %v", err)
	}
	got, err := repository.Get(ctx, job.ID)
	if err != nil || got.State != jobs.StateDead || got.FinishedAt.IsZero() {
		t.Fatalf("expired job = (%+v, %v)", got, err)
	}
}

var _ jobs.Store = cancelingStore{}

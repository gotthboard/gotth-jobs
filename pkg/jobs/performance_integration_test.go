//go:build integration && performance

package jobs_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/gotthboard/gotth-jobs/pkg/jobs"
)

func TestPostgreSQLPerformanceAdmission(t *testing.T) {
	pool, repository := integrationRepository(t)
	ctx := context.Background()
	type workload struct {
		name    string
		backlog int
		claims  int
		payload int
		empty   bool
	}
	workloads := []workload{
		{name: "empty", claims: 100, empty: true},
		{name: "small", backlog: 100, claims: 50},
		{name: "typical", backlog: 500, claims: 200, payload: 1024},
		{name: "large-payload-boundary", backlog: 20, claims: 20, payload: jobs.MaxPayloadBytes},
	}
	for _, workload := range workloads {
		t.Run(workload.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "TRUNCATE public.gotth_jobs"); err != nil {
				t.Fatal(err)
			}
			for index := range workload.backlog {
				if _, _, err := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "perf", Kind: "claim", Payload: make([]byte, workload.payload), MaxAttempts: 1}); err != nil {
					t.Fatalf("enqueue fixture %d: %v", index, err)
				}
			}
			durations := make([]time.Duration, 0, workload.claims)
			started := time.Now()
			for index := range workload.claims {
				before := time.Now()
				job, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "perf", Worker: "benchmark", LeaseDuration: time.Second})
				durations = append(durations, time.Since(before))
				if workload.empty {
					if !errors.Is(err, jobs.ErrNoJob) {
						t.Fatalf("empty claim %d = %v", index, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("claim %d: %v", index, err)
				}
				if _, err := repository.Complete(ctx, job.Lease); err != nil {
					t.Fatalf("complete %d: %v", index, err)
				}
			}
			wall := time.Since(started)
			sort.Slice(durations, func(left, right int) bool { return durations[left] < durations[right] })
			t.Logf("workload=%s samples=%d p50=%s p95=%s p99=%s throughput=%.2f_ops_s wall=%s", workload.name, len(durations), percentile(durations, 50), percentile(durations, 95), percentile(durations, 99), float64(len(durations))/wall.Seconds(), wall)
		})
	}

	t.Run("pathological-locked-prefix", func(t *testing.T) {
		if _, err := pool.Exec(ctx, "TRUNCATE public.gotth_jobs"); err != nil {
			t.Fatal(err)
		}
		for index := range 101 {
			if _, _, err := repository.Enqueue(ctx, jobs.EnqueueRequest{Queue: "perf", Kind: "locked", Payload: []byte{byte(index)}, MaxAttempts: 1}); err != nil {
				t.Fatal(err)
			}
		}
		locker, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer locker.Rollback(ctx)
		rows, err := locker.Query(ctx, `SELECT id FROM public.gotth_jobs
            WHERE queue = 'perf' AND state = 'pending'
            ORDER BY available_at, created_at, id
            LIMIT 100 FOR UPDATE`)
		if err != nil {
			t.Fatal(err)
		}
		locked := 0
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			locked++
		}
		rows.Close()
		if err := rows.Err(); err != nil || locked != 100 {
			t.Fatalf("locked rows = %d, %v", locked, err)
		}
		started := time.Now()
		job, err := repository.Claim(ctx, jobs.ClaimRequest{Queue: "perf", Worker: "benchmark", LeaseDuration: time.Second})
		duration := time.Since(started)
		if err != nil || job.ID == "" {
			t.Fatalf("claim behind locked prefix = (%+v, %v)", job, err)
		}
		t.Logf("workload=pathological-locked-prefix locked=100 samples=1 latency=%s", duration)
	})
}

func percentile(sorted []time.Duration, percent int) time.Duration {
	index := (len(sorted)*percent + 99) / 100
	if index < 1 {
		index = 1
	}
	return sorted[index-1]
}

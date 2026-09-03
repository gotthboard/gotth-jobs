package jobs_test

import (
	"context"
	"io/fs"
	"time"

	"github.com/gotthboard/gotth-jobs/pkg/jobs"
	"github.com/jackc/pgx/v5"
)

func compilePublicAPI(ctx context.Context, database jobs.Database, transaction pgx.Tx) error {
	repository, err := jobs.NewPostgreSQL(database)
	if err != nil {
		return err
	}
	request := jobs.EnqueueRequest{Queue: "default", Kind: "send", MaxAttempts: jobs.MaxAttempts}
	job, _, _ := repository.Enqueue(ctx, request)
	_, _, _ = repository.EnqueueTx(ctx, transaction, request)
	claimed, _ := repository.Claim(ctx, jobs.ClaimRequest{Queue: "default", Worker: "worker", LeaseDuration: jobs.MinLeaseDuration})
	_, _ = repository.Heartbeat(ctx, claimed.Lease, jobs.MaxLeaseDuration)
	_, _ = repository.Complete(ctx, claimed.Lease)
	_, _ = repository.Fail(ctx, claimed.Lease, jobs.Failure{Permanent: true})
	_, _ = repository.Get(ctx, job.ID)
	_, _ = repository.Counts(ctx, "default")
	dead, _ := repository.ListDead(ctx, "default", &jobs.DeadCursor{FinishedAt: time.Now().UTC(), ID: job.ID}, jobs.MaxDeadPage)
	_, _ = repository.Cancel(ctx, job.ID)
	_, _ = repository.Redrive(ctx, job.ID)
	_, _ = jobs.RetryPolicy{Initial: time.Second, Maximum: jobs.MaxRetryDelay}.Delay(1)
	_ = jobs.Permanent(err)
	_ = jobs.IsPermanent(err)
	var migrations fs.FS = jobs.Migrations()
	_ = migrations
	_ = dead
	return nil
}

var _ jobs.Store = (*jobs.PostgreSQL)(nil)
var _ = compilePublicAPI

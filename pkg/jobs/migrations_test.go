package jobs_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/gotthboard/gotth-jobs/pkg/jobs"
)

func TestMigrationsExposeOneImmutableSchema(t *testing.T) {
	filesystem := jobs.Migrations()
	entries, err := fs.ReadDir(filesystem, ".")
	if err != nil {
		t.Fatalf("ReadDir() = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "000001_jobs.sql" {
		t.Fatalf("migration entries = %+v", entries)
	}
	body, err := fs.ReadFile(filesystem, "000001_jobs.sql")
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	for _, required := range []string{
		"CREATE TABLE public.gotth_jobs",
		"gotth_jobs_idempotency",
		"gotth_jobs_claim",
		"CHECK",
		"lease_token",
		"finished_at",
	} {
		if !strings.Contains(string(body), required) {
			t.Errorf("migration missing %q", required)
		}
	}
	if _, err := fs.ReadFile(filesystem, "../go.mod"); err == nil {
		t.Fatal("migration filesystem escaped its subtree")
	}
}

func TestMigrationConstrainsStateAttemptCombinations(t *testing.T) {
	body, err := fs.ReadFile(jobs.Migrations(), "000001_jobs.sql")
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	for _, required := range []string{
		"CONSTRAINT gotth_jobs_state_attempt_shape CHECK",
		"(state = 'pending' AND attempts < max_attempts)",
		"(state IN ('running', 'succeeded', 'dead') AND attempts >= 1)",
		"state = 'canceled'",
	} {
		if !strings.Contains(string(body), required) {
			t.Errorf("migration missing state/attempt rule %q", required)
		}
	}
}

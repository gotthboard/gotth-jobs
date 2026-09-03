package jobs

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrations returns the immutable PostgreSQL migration files rooted at the
// first migration filename. The caller owns application and rollback policy.
//
// Complexity: time O(1), Omega(1), tight Theta(1); auxiliary space O(1),
// Omega(1), tight Theta(1). The embedded bytes are not copied.
func Migrations() fs.FS {
	files, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		panic("jobs: embedded migration subtree is missing: " + err.Error())
	}
	return files
}

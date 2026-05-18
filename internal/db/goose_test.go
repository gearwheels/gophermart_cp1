package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunMigrations_SQLiteDialect(t *testing.T) {
	dir := t.TempDir()
	mig := `-- +goose Up
CREATE TABLE t (id INTEGER PRIMARY KEY);
-- +goose Down
DROP TABLE IF EXISTS t;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "00001_init.sql"), []byte(mig), 0o600))

	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, runMigrations(sqlDB, dir, "sqlite3"))

	// sanity: table exists
	_, err = sqlDB.Exec("INSERT INTO t(id) VALUES (1)")
	require.NoError(t, err)
}

func TestRunMigrations_BadDialect(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	err = runMigrations(sqlDB, t.TempDir(), "definitely-not-a-dialect")
	require.Error(t, err)
}


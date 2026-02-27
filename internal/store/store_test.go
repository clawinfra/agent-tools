package store_test

import (
	"os"
	"testing"

	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_InMemory(t *testing.T) {
	db, err := store.Open(":memory:")
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.NoError(t, db.Close())
}

func TestOpen_FileDB(t *testing.T) {
	path := t.TempDir() + "/test.db"
	db, err := store.Open(path)
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.NoError(t, db.Close())
}

func TestOpen_DirCreationFailure(t *testing.T) {
	// Create a file where the dir should be, causing MkdirAll to fail
	tmp := t.TempDir()
	blockingFile := tmp + "/blocking"
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0o644))

	// Try to create DB inside a "directory" that is actually a file
	_, err := store.Open(blockingFile + "/agent-tools.db")
	assert.Error(t, err)
}

func TestOpen_IdempotentMigration(t *testing.T) {
	path := t.TempDir() + "/test.db"
	db, err := store.Open(path)
	require.NoError(t, err)
	db.Close()

	// Open again — migrations should run without error (IF NOT EXISTS guards)
	db2, err := store.Open(path)
	require.NoError(t, err)
	assert.NoError(t, db2.Close())
}

package cli_test

import (
	"os"
	"testing"

	"github.com/clawinfra/agent-tools/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeCmd_BadDBPath(t *testing.T) {
	// Create a file that blocks directory creation for the DB path.
	tmp := t.TempDir()
	blockingFile := tmp + "/blocking"
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0o600))

	root := cli.NewRootCmd()
	// Try to use a path inside the blocking file as a directory.
	root.SetArgs([]string{"serve", "--db", blockingFile + "/db/agent-tools.db"})
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "open store")
}

func TestServeCmd_Structure(t *testing.T) {
	root := cli.NewRootCmd()
	serveCmd, _, err := root.Find([]string{"serve"})
	require.NoError(t, err)
	require.NotNil(t, serveCmd)
	assert.Equal(t, "serve", serveCmd.Use)
	assert.NotNil(t, serveCmd.Flag("addr"))
	assert.NotNil(t, serveCmd.Flag("db"))
}

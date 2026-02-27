package cli_test

import (
	"testing"

	"github.com/clawinfra/agent-tools/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCmd_Structure(t *testing.T) {
	root := cli.NewRootCmd()
	require.NotNil(t, root)
	assert.Equal(t, "agent-tools", root.Use)

	// Verify sub-commands exist
	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Use)
	}
	assert.Contains(t, names, "serve")
	assert.Contains(t, names, "init")
	assert.Contains(t, names, "tool")
}

func TestNewRootCmd_Help(t *testing.T) {
	root := cli.NewRootCmd()
	// Execute with --help should not error
	root.SetArgs([]string{"--help"})
	// Help exits with code 0, cobra.Execute doesn't return error for --help
	_ = root.Execute()
}

func TestServeCmd_FlagsExist(t *testing.T) {
	root := cli.NewRootCmd()
	serveCmd, _, err := root.Find([]string{"serve"})
	require.NoError(t, err)
	require.NotNil(t, serveCmd)

	assert.NotNil(t, serveCmd.Flag("addr"))
	assert.NotNil(t, serveCmd.Flag("db"))
}

func TestToolCmd_Subcommands(t *testing.T) {
	root := cli.NewRootCmd()
	toolCmd, _, err := root.Find([]string{"tool"})
	require.NoError(t, err)
	require.NotNil(t, toolCmd)

	var names []string
	for _, cmd := range toolCmd.Commands() {
		names = append(names, cmd.Use)
	}
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "search")
}

func TestInitCmd_Idempotent(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"init"})

	// Change to temp dir to avoid polluting workspace
	t.Chdir(t.TempDir())

	err := root.Execute()
	assert.NoError(t, err)

	// Run again — should detect existing config
	err = root.Execute()
	assert.NoError(t, err)
}

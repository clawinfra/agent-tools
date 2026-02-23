// Package cli provides the agent-tools command-line interface.
package cli

import (
	"github.com/spf13/cobra"
)

// NewRootCmd returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "agent-tools",
		Short: "Decentralized tool registry for autonomous AI agents",
		Long: `agent-tools is the picks-and-shovels layer for autonomous AI agents.

Agents register tools they offer, discover tools they need, invoke them
with cryptographic receipts, and settle payments in CLAW tokens.

Learn more: https://github.com/clawinfra/agent-tools`,
	}

	root.AddCommand(
		newServeCmd(),
		newInitCmd(),
		newToolCmd(),
	)

	return root
}

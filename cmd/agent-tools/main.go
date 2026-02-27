// Package main is the entry point for the agent-tools registry server and CLI.
package main

import (
	"fmt"
	"os"

	"github.com/clawinfra/agent-tools/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

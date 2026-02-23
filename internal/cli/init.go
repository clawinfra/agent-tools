package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize agent-tools configuration",
		RunE: func(_ *cobra.Command, _ []string) error {
			dirs := []string{"data", "schemas"}
			for _, d := range dirs {
				if err := os.MkdirAll(d, 0o755); err != nil {
					return fmt.Errorf("create %s: %w", d, err)
				}
			}

			cfgPath := "agent-tools.toml"
			if _, err := os.Stat(cfgPath); err == nil {
				fmt.Printf("Config already exists at %s\n", cfgPath)
				return nil
			}

			cfg := `# agent-tools configuration
[server]
addr = ":8433"
db   = "./data/agent-tools.db"

[clawchain]
# ws_url = "ws://testnet.clawchain.win:9944"
`
			if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			abs, _ := filepath.Abs(cfgPath)
			fmt.Printf("✅ Initialized agent-tools at %s\n", abs)
			fmt.Println("   Run: agent-tools serve")
			return nil
		},
	}
}

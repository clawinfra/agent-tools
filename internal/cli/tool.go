package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/clawinfra/agent-tools/sdk/go/agenttools"
	"github.com/spf13/cobra"
)

func newToolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage tools in the registry",
	}

	cmd.AddCommand(
		newToolListCmd(),
		newToolSearchCmd(),
	)

	return cmd
}

func newToolListCmd() *cobra.Command {
	var registryURL string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tools in the registry",
		RunE: func(_ *cobra.Command, _ []string) error {
			client := agenttools.NewClient(registryURL)
			result, err := client.ListTools(nil, &agenttools.ListToolsRequest{Limit: 50})
			if err != nil {
				return err
			}

			if len(result.Tools) == 0 {
				fmt.Println("No tools registered.")
				return nil
			}

			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result.Tools)
		},
	}

	cmd.Flags().StringVar(&registryURL, "registry", "http://localhost:8433", "Registry URL")
	return cmd
}

func newToolSearchCmd() *cobra.Command {
	var (
		registryURL string
		query       string
		maxPrice    float64
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for tools by capability",
		RunE: func(_ *cobra.Command, _ []string) error {
			client := agenttools.NewClient(registryURL)
			opts := []agenttools.SearchOption{}
			if maxPrice > 0 {
				opts = append(opts, agenttools.WithMaxPrice(maxPrice))
			}

			result, err := client.SearchTools(nil, query, opts...)
			if err != nil {
				return err
			}

			if len(result.Tools) == 0 {
				fmt.Printf("No tools found for query: %q\n", query)
				return nil
			}

			fmt.Printf("Found %d tools:\n\n", len(result.Tools))
			for _, t := range result.Tools {
				fmt.Printf("  %s @ %s\n", t.Name, t.Version)
				fmt.Printf("    ID: %s\n", t.ID)
				fmt.Printf("    %s\n", t.Description)
				if t.Pricing != nil {
					fmt.Printf("    Price: %s\n", t.Pricing.String())
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&registryURL, "registry", "http://localhost:8433", "Registry URL")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Search query")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price in CLAW")
	_ = cmd.MarkFlagRequired("query")

	return cmd
}

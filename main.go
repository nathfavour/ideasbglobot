package main

import (
	"fmt"
	"os"

	"github.com/nathfavour/ideasbglobot/cmd"
	"github.com/nathfavour/ideasbglobot/internal"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ideasbglobot",
		Short: "Automate Telegram bots, AI, and git from the CLI",
		Run: func(cmd *cobra.Command, args []string) {
			// Ensure all config/data/auto files are present and valid
			cfg, err := internal.EnsureConfigFile()
			if err != nil {
				fmt.Printf("Error initializing config: %v\n", err)
				os.Exit(1)
			}

			if err := internal.EnsureDatabase(); err != nil {
				fmt.Printf("Failed to initialize database: %v\n", err)
				os.Exit(1)
			}

			internal.RunDefaultBot(cfg)
		},
	}

	rootCmd.AddCommand(cmd.BotCmd)
	rootCmd.AddCommand(cmd.AiCmd)
	rootCmd.AddCommand(cmd.GitCmd)
	rootCmd.AddCommand(cmd.GhCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

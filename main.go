package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nathfavour/ideasbglobot/cmd"
	"github.com/nathfavour/ideasbglobot/internal"
	"github.com/nathfavour/ideasbglobot/internal/engine"
	"github.com/nathfavour/ideasbglobot/internal/platform"
	"github.com/nathfavour/ideasbglobot/internal/provider"
	"github.com/spf13/cobra"
)

func main() {
	// Initialize logger
	if err := internal.SetupLogger(); err != nil {
		fmt.Printf("Warning: Failed to setup logger: %v\n", err)
	}

	var allowIDs []int64

	rootCmd := &cobra.Command{
		Use:   "ideasbglobot",
		Short: "Automate Telegram bots, AI, and git from the CLI",
		Run: func(cmd *cobra.Command, args []string) {
			// Ensure all config/data/auto files are present and valid
			cfg, err := internal.EnsureConfigFile()
			if err != nil {
				log.Printf("Error initializing config: %v\n", err)
				os.Exit(1)
			}

			// Add allowed IDs from flags if any
			if len(allowIDs) > 0 {
				for _, id := range allowIDs {
					exists := false
					for _, existingID := range cfg.AllowedIDs {
						if existingID == id {
							exists = true
							break
						}
					}
					if !exists {
						cfg.AllowedIDs = append(cfg.AllowedIDs, id)
					}
				}
				path, _ := internal.GetConfigPath()
				internal.SaveConfig(path, cfg)
			}

			if err := internal.EnsureDatabase(); err != nil {
				log.Printf("Failed to initialize database: %v\n", err)
				os.Exit(1)
			}

			// Initialize New Modular Architecture
			aiProvider := provider.NewVibeAuraProvider("")
			eng := engine.NewEngine(cfg, aiProvider)

			// Setup Telegram Platform if token exists
			if cfg.DefaultBotID != "" {
				botCfg, ok := cfg.Bots[cfg.DefaultBotID]
				if ok && botCfg.Token != "" {
					tg, err := platform.NewTelegramPlatform(botCfg.Token)
					if err != nil {
						log.Printf("Failed to init Telegram: %v\n", err)
					} else {
						eng.AddPlatform(tg)
					}
				}
			}

			log.Println("Starting Ultra-Modular Bot Engine...")
			eng.Start()

			// Keep alive
			select {}
		},
	}

	rootCmd.AddCommand(cmd.BotCmd)
	rootCmd.AddCommand(cmd.AiCmd)
	rootCmd.AddCommand(cmd.GitCmd)
	rootCmd.AddCommand(cmd.GhCmd)

	rootCmd.PersistentFlags().Int64SliceVar(&allowIDs, "allow", []int64{}, "Allow specific user or group IDs")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

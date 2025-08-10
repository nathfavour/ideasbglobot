package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nathfavour/ideasbglobot/internal"
	"github.com/spf13/cobra"
)

var BotCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start and control the Telegram bot",
	Run: func(cmd *cobra.Command, args []string) {
		token := os.Getenv("TELEGRAM_BOT_TOKEN")
		if token != "" {
			fmt.Println("Starting Telegram bot...")
			// Start bot logic here (see bot/telegram.go)
		} else {
			fmt.Println("TELEGRAM_BOT_TOKEN not set")
		}
	},
}

var botAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Telegram bot configuration",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter bot ID (unique name): ")
		id, _ := reader.ReadString('\n')
		id = strings.TrimSpace(id)
		fmt.Print("Enter bot token: ")
		token, _ := reader.ReadString('\n')
		token = strings.TrimSpace(token)

		// Load config
		cfgPath, err := internal.GetConfigPath()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		cfg, err := internal.EnsureConfigFile()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		if cfg.Bots == nil {
			cfg.Bots = map[string]internal.BotConfig{}
		}
		cfg.Bots[id] = internal.BotConfig{ID: id, Token: token}

		fmt.Print("Set this bot as default? (y/N): ")
		setDefault, _ := reader.ReadString('\n')
		setDefault = strings.TrimSpace(strings.ToLower(setDefault))
		if setDefault == "y" || setDefault == "yes" {
			cfg.DefaultBotID = id
		}

		// Save config
		if err := internal.SaveConfig(cfgPath, cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Println("Bot added successfully.")
	},
}

// getConfigPath is now in internal/config.go as GetConfigPath

// Use BotConfig and Configs from internal/config.go

func loadConfig(path string) (*internal.Configs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg internal.Configs
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *internal.Configs) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func init() {
	BotCmd.AddCommand(botAddCmd)
}

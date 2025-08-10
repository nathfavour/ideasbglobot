package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

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
		cfgPath, err := getConfigPath()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		cfg, err := loadConfig(cfgPath)
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		if cfg.Bots == nil {
			cfg.Bots = map[string]BotConfig{}
		}
		cfg.Bots[id] = BotConfig{ID: id, Token: token}

		fmt.Print("Set this bot as default? (y/N): ")
		setDefault, _ := reader.ReadString('\n')
		setDefault = strings.TrimSpace(strings.ToLower(setDefault))
		if setDefault == "y" || setDefault == "yes" {
			cfg.DefaultBotID = id
		}

		// Save config
		if err := saveConfig(cfgPath, cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Println("Bot added successfully.")
	},
}

func getConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(usr.HomeDir, ".ideasbglobe")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "configs.json"), nil
}

type BotConfig struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type Configs struct {
	DefaultBotID string               `json:"default_bot_id"`
	Bots         map[string]BotConfig `json:"bots"`
}

func loadConfig(path string) (*Configs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Configs
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *Configs) error {
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

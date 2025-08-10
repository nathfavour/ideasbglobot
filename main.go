package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nathfavour/ideasbglobot/cmd"
)

type BotConfig struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	// ...add more fields as needed...
}

type Configs struct {
	DefaultBotID string               `json:"default_bot_id"`
	Bots         map[string]BotConfig `json:"bots"`
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

func ensureConfigFile() (*Configs, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		defaultConfig := &Configs{
			DefaultBotID: "",
			Bots:         map[string]BotConfig{},
		}
		f, err := os.Create(configPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(defaultConfig); err != nil {
			return nil, err
		}
		return defaultConfig, nil
	}
	// Load config
	f, err := os.Open(configPath)
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

func runDefaultBot(cfg *Configs) {
	if cfg.DefaultBotID == "" {
		fmt.Println("No default bot configured. Please add a bot to ~/.ideasbglobe/configs.json and set default_bot_id.")
		return
	}
	bot, ok := cfg.Bots[cfg.DefaultBotID]
	if !ok {
		fmt.Printf("Default bot id '%s' not found in configs.\n", cfg.DefaultBotID)
		return
	}
	fmt.Printf("Starting default bot: %s\n", bot.ID)
	if bot.Token == "" {
		fmt.Println("Bot token empty; cannot start.")
		return
	}
	runTelegramBot(bot.Token)
}

func runTelegramBot(token string) {
	b, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		fmt.Printf("Telegram bot init error: %v\n", err)
		return
	}
	log.Printf("Authorized as @%s", b.Self.UserName)

	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel // placeholder for future graceful shutdown handling

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-updates:
			if !ok {
				fmt.Println("Update channel closed.")
				return
			}
			// Basic logging of incoming updates
			switch {
			case upd.Message != nil:
				from := ""
				if upd.Message.From != nil {
					from = upd.Message.From.UserName
				}
				text := upd.Message.Text
				if text == "" {
					text = "<non-text message>"
				}
				fmt.Printf("[msg] chat=%d from=%s text=%q\n", upd.Message.Chat.ID, from, text)

				// Simple command echo
				if upd.Message.IsCommand() {
					cmd := upd.Message.Command()
					args := upd.Message.CommandArguments()
					fmt.Printf("[cmd] /%s %s\n", cmd, args)
					reply := tgbotapi.NewMessage(upd.Message.Chat.ID, fmt.Sprintf("Command %s received.", cmd))
					_, _ = b.Send(reply)
				}
			case upd.CallbackQuery != nil:
				fmt.Printf("[callback] id=%s data=%q\n", upd.CallbackQuery.ID, upd.CallbackQuery.Data)
			default:
				fmt.Printf("[raw update] %+v\n", upd)
			}
		}
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "ideasbglobot",
		Short: "Automate Telegram bots, AI, and git from the CLI",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := ensureConfigFile()
			if err != nil {
				fmt.Printf("Error initializing config: %v\n", err)
				os.Exit(1)
			}
			runDefaultBot(cfg)
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

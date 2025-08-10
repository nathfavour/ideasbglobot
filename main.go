package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nathfavour/ideasbglobot/cmd"
	"github.com/spf13/cobra"
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
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Failed to create bot: %v", err)
		return
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutdown signal received")
		cancel()
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	log.Println("Bot started. Listening for messages...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Bot stopped")
			bot.StopReceivingUpdates()
			return

		case update := <-updates:
			if update.Message == nil {
				continue
			}

			username := update.Message.From.UserName
			if username == "" {
				username = update.Message.From.FirstName
			}

			log.Printf("[MESSAGE] Chat: %d, User: %s, Text: %s",
				update.Message.Chat.ID, username, update.Message.Text)

			if update.Message.IsCommand() {
				command := update.Message.Command()
				args := update.Message.CommandArguments()
				log.Printf("[COMMAND] /%s %s", command, args)

				// Example: run shell command if /run
				if command == "run" && args != "" {
					out, err := runShellCommand(args)
					resp := ""
					if err != nil {
						resp = fmt.Sprintf("❌ Error: %v", err)
					} else {
						resp = fmt.Sprintf("💻 Output:\n%s", out)
					}
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, resp)
					bot.Send(msg)
					continue
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					fmt.Sprintf("✅ Command received: /%s %s", command, args))
				bot.Send(msg)
			} else {
				// Use Ollama to understand and reply
				reply, err := ollamaChat(update.Message.Text)
				if err != nil {
					reply = fmt.Sprintf("Ollama error: %v", err)
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
				bot.Send(msg)
			}
		}
	}
}

// Call local Ollama API for chat completion
func ollamaChat(prompt string) (string, error) {
	ollamaURL := "http://localhost:11434/api/generate"
	payload := `{"model":"llama3","prompt":` + jsonString(prompt) + `,"stream":false}`
	req, err := http.NewRequest("POST", ollamaURL, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// Ollama returns JSON with a "response" field
	type ollamaResp struct {
		Response string `json:"response"`
	}
	var o ollamaResp
	if err := json.Unmarshal(body, &o); err != nil {
		return "", fmt.Errorf("ollama response: %s", string(body))
	}
	return strings.TrimSpace(o.Response), nil
}

// Helper to escape JSON string
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Run a shell command and return output
func runShellCommand(cmdline string) (string, error) {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return "", fmt.Errorf("no command provided")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
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

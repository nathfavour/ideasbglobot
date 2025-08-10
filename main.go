package main

import (
	"bytes"
	"context"
	"database/sql"
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
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
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

type Message struct {
	ID       int64     `json:"id"`
	ChatID   int64     `json:"chat_id"`
	UserID   int64     `json:"user_id"`
	Username string    `json:"username"`
	Text     string    `json:"text"`
	IsBot    bool      `json:"is_bot"`
	Type     string    `json:"type"` // "message", "command", "issue", "feature_request"
	Created  time.Time `json:"created"`
}

type AutoReply struct {
	ID       int    `json:"id"`
	Category string `json:"category"`
	Reply    string `json:"reply"`
	Context  string `json:"context"`
}

var db *sql.DB

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

func initDatabase() error {
	dbPath := filepath.Join(getAppDir(), "data.db")
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER,
			user_id INTEGER,
			username TEXT,
			text TEXT,
			is_bot BOOLEAN,
			type TEXT,
			created DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS auto_replies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category TEXT,
			reply TEXT,
			context TEXT
		)
	`)
	return err
}

func getAppDir() string {
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, ".ideasbglobe")
}

func saveMessage(msg Message) error {
	_, err := db.Exec(`
		INSERT INTO messages (chat_id, user_id, username, text, is_bot, type, created) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ChatID, msg.UserID, msg.Username, msg.Text, msg.IsBot, msg.Type, msg.Created)
	return err
}

func detectMessageType(text string) string {
	text = strings.ToLower(text)
	if strings.Contains(text, "issue") || strings.Contains(text, "bug") || strings.Contains(text, "error") || strings.Contains(text, "problem") {
		return "issue"
	}
	if strings.Contains(text, "feature") || strings.Contains(text, "enhancement") || strings.Contains(text, "request") || strings.Contains(text, "add") {
		return "feature_request"
	}
	if strings.Contains(text, "question") || strings.Contains(text, "how") || strings.Contains(text, "what") || strings.Contains(text, "?") {
		return "question"
	}
	return "message"
}

func shouldRespond(text string, chatID int64) bool {
	// Only respond if bot is mentioned or in DM or specific keywords
	botMentioned := strings.Contains(text, "@ideabglobe_bot") || strings.Contains(text, "@ideabglobe")
	isQuestion := strings.Contains(text, "?")
	isCommand := strings.HasPrefix(text, "/")
	isDM := chatID > 0 // DMs have positive chat IDs

	return botMentioned || isDM || isQuestion || isCommand
}

func getSmartReply(text string, msgType string) (string, error) {
	// Try to get AI response first
	if reply, err := ollamaChat(buildAIPrompt(text, msgType)); err == nil {
		return reply, nil
	}

	// Fallback to auto replies
	return getAutoReply(msgType), nil
}

func buildAIPrompt(text string, msgType string) string {
	context := fmt.Sprintf("You are a software engineering assistant bot in a Telegram group. Message type: %s. Be concise and helpful.", msgType)
	return fmt.Sprintf("%s\n\nUser message: %s", context, text)
}

func getAutoReply(category string) string {
	autoReplies := map[string][]string{
		"issue": {
			"🐛 I see you've mentioned an issue. Can you provide more details?",
			"📝 Please create a detailed issue report with steps to reproduce.",
			"🔍 Let me help you troubleshoot this problem.",
		},
		"feature_request": {
			"💡 Interesting feature idea! Let's discuss the requirements.",
			"🚀 That sounds like a useful enhancement. Can you elaborate?",
			"📋 I'll help you draft a proper feature request.",
		},
		"question": {
			"🤔 Good question! Let me think about this...",
			"📚 I can help you with that. Here's what I know:",
			"💭 Interesting question. Let me research that for you.",
		},
		"default": {
			"👍 Noted! I'm tracking this conversation.",
			"📊 I'm here to help with software engineering tasks.",
			"🤖 How can I assist with your development work?",
		},
	}

	replies, exists := autoReplies[category]
	if !exists {
		replies = autoReplies["default"]
	}
	return replies[len(replies)%3] // Simple rotation
}

func runTelegramBot(token string) {
	// Initialize database
	if err := initDatabase(); err != nil {
		log.Printf("Database init error: %v", err)
		return
	}
	defer db.Close()

	// Generate auto replies file
	generateAutoReplies()

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Failed to create bot: %v", err)
		return
	}

	bot.Debug = false // Reduce debug noise
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

			// Save message to database
			msgType := detectMessageType(update.Message.Text)
			msg := Message{
				ChatID:   update.Message.Chat.ID,
				UserID:   update.Message.From.ID,
				Username: username,
				Text:     update.Message.Text,
				IsBot:    update.Message.From.IsBot,
				Type:     msgType,
				Created:  time.Now(),
			}
			saveMessage(msg)

			log.Printf("[%s] Chat: %d, User: %s, Text: %s",
				strings.ToUpper(msgType), update.Message.Chat.ID, username, update.Message.Text)

			if update.Message.IsCommand() {
				command := update.Message.Command()
				args := update.Message.CommandArguments()
				log.Printf("[COMMAND] /%s %s", command, args)

				if command == "run" && args != "" {
					out, err := runShellCommand(args)
					resp := ""
					if err != nil {
						resp = fmt.Sprintf("❌ Error: %v", err)
					} else {
						resp = fmt.Sprintf("💻 Output:\n%s", out)
					}
					reply := tgbotapi.NewMessage(update.Message.Chat.ID, resp)
					bot.Send(reply)
					continue
				}

				if command == "status" {
					reply := tgbotapi.NewMessage(update.Message.Chat.ID, "🤖 Bot is running and tracking conversations.")
					bot.Send(reply)
					continue
				}

				// Default command response
				reply := tgbotapi.NewMessage(update.Message.Chat.ID,
					fmt.Sprintf("⚡ Command processed: /%s", command))
				bot.Send(reply)
			} else if shouldRespond(update.Message.Text, update.Message.Chat.ID) {
				// Smart response based on message type
				response, err := getSmartReply(update.Message.Text, msgType)
				if err != nil {
					log.Printf("Error getting smart reply: %v", err)
					continue
				}
				reply := tgbotapi.NewMessage(update.Message.Chat.ID, response)
				bot.Send(reply)
			}
		}
	}
}

func generateAutoReplies() {
	autoRepliesPath := filepath.Join(getAppDir(), "auto.json")
	if _, err := os.Stat(autoRepliesPath); err == nil {
		return // File already exists
	}

	autoReplies := []AutoReply{
		{1, "greeting", "👋 Hello! I'm here to help with software engineering tasks.", "when users greet the bot"},
		{2, "issue", "🐛 I see you've mentioned an issue. Can you provide more details like steps to reproduce, expected vs actual behavior?", "when users report bugs or issues"},
		{3, "feature", "💡 Interesting feature idea! Let's break it down. What's the main use case and expected outcome?", "when users suggest new features"},
		{4, "question", "🤔 Good question! Let me help you with that. Can you provide more context?", "when users ask questions"},
		{5, "code", "💻 I can help with code review, debugging, or implementation suggestions. Share your code!", "when users mention code-related topics"},
		// Add more auto replies as needed...
	}

	file, err := os.Create(autoRepliesPath)
	if err != nil {
		log.Printf("Error creating auto.json: %v", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.Encode(autoReplies)
	log.Println("Generated auto.json with default replies")
}

// Replace the hardcoded model with the selected one from ai.go
func ollamaChat(prompt string) (string, error) {
	// Use the model set by the user via ai ollama model set
	model := getOllamaModel()
	ollamaURL := "http://localhost:11434/api/generate"
	payload := `{"model":` + jsonString(model) + `,"prompt":` + jsonString(prompt) + `,"stream":false}`
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
	type ollamaResp struct {
		Response string `json:"response"`
	}
	var o ollamaResp
	if err := json.Unmarshal(body, &o); err != nil {
		return "", fmt.Errorf("ollama response: %s", string(body))
	}
	return strings.TrimSpace(o.Response), nil
}

// Helper to get the current ollama model from the ai command package
func getOllamaModel() string {
	// Import as: "github.com/nathfavour/ideasbglobot/cmd"
	return cmd.OllamaModel()
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

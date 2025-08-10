package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nathfavour/ideasbglobot/internal"
	"github.com/nathfavour/ideasbglobot/internal/autoreply"
	"github.com/nathfavour/ideasbglobot/internal/config"
	"github.com/nathfavour/ideasbglobot/internal/db"
)

func RunDefaultBot(cfg *config.Configs) {
	if cfg.DefaultBotID == "" {
		fmt.Println("No default bot configured. Please add a bot to ~/.ideasbglobe/configs.json and set default_bot_id.")
		return
	}
	botCfg, ok := cfg.Bots[cfg.DefaultBotID]
	if !ok {
		fmt.Printf("Default bot id '%s' not found in configs.\n", cfg.DefaultBotID)
		return
	}
	fmt.Printf("Starting default bot: %s\n", botCfg.ID)
	if botCfg.Token == "" {
		fmt.Println("Bot token empty; cannot start.")
		return
	}
	RunTelegramBot(botCfg.Token)
}

func RunTelegramBot(token string) {
	if err := db.EnsureDatabase(); err != nil {
		log.Printf("Database init error: %v", err)
		return
	}
	defer db.DB.Close()
	autoreply.EnsureAutoReplies()

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Failed to create bot: %v", err)
		return
	}

	bot.Debug = false
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

			msgType := detectMessageType(update.Message.Text)
			msg := db.Message{
				ChatID:   update.Message.Chat.ID,
				UserID:   update.Message.From.ID,
				Username: username,
				Text:     update.Message.Text,
				IsBot:    update.Message.From.IsBot,
				Type:     msgType,
				Created:  time.Now(),
			}
			db.SaveMessage(msg)

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

				reply := tgbotapi.NewMessage(update.Message.Chat.ID,
					fmt.Sprintf("⚡ Command processed: /%s", command))
				bot.Send(reply)
			} else if shouldRespond(update.Message.Text, update.Message.Chat.ID) {
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

// --- Helper functions (move from main.go as needed) ---

func detectMessageType(text string) string {
	text = strings.ToLower(text)
	switch {
	case strings.Contains(text, "issue"), strings.Contains(text, "bug"), strings.Contains(text, "error"), strings.Contains(text, "problem"):
		return "issue"
	case strings.Contains(text, "feature"), strings.Contains(text, "enhancement"), strings.Contains(text, "request"), strings.Contains(text, "add"):
		return "feature_request"
	case strings.Contains(text, "question"), strings.Contains(text, "how"), strings.Contains(text, "what"), strings.Contains(text, "?"):
		return "question"
	default:
		return "message"
	}
}

func shouldRespond(text string, chatID int64) bool {
	botMentioned := strings.Contains(text, "@ideabglobe_bot") || strings.Contains(text, "@ideabglobe")
	isQuestion := strings.Contains(text, "?")
	isCommand := strings.HasPrefix(text, "/")
	isDM := chatID > 0
	return botMentioned || isDM || isQuestion || isCommand
}

func getSmartReply(text string, msgType string) (string, error) {
	if reply, err := internal.OllamaChat(buildAIPrompt(text, msgType)); err == nil {
		return reply, nil
	}
	return getAutoReply(msgType), nil
}

func buildAIPrompt(text string, msgType string) string {
	context := fmt.Sprintf("You are a software engineering assistant bot in a Telegram group. Message type: %s. Be concise and helpful.", msgType)
	return fmt.Sprintf("%s\n\nUser message: %s", context, text)
}

func getAutoReply(category string) string {
	// For MVP, just use a static set as before, or load from auto.json if desired
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
	return replies[len(replies)%3]
}

func runShellCommand(cmdline string) (string, error) {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return "", fmt.Errorf("no command provided")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

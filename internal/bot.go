package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Global config cache for live updates
var botConfig *Configs

func StartBot(token string) {
	EnsureAutoReplies()

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
			msg := Message{
				ChatID:   update.Message.Chat.ID,
				UserID:   update.Message.From.ID,
				Username: username,
				Text:     update.Message.Text,
				IsBot:    update.Message.From.IsBot,
				Type:     msgType,
				Created:  time.Now(),
			}
			SaveMessage(msg)

			log.Printf("[%s] Chat: %d, User: %s, Text: %s",
				strings.ToUpper(msgType), update.Message.Chat.ID, username, update.Message.Text)

			// AI model set command: /ai ollama model set <modelname>
			if matched, _ := regexp.MatchString(`/ai ollama model set [a-zA-Z0-9_\-]+`, update.Message.Text); matched {
				parts := strings.Fields(update.Message.Text)
				for i := 0; i < len(parts)-3; i++ {
					if parts[i] == "/ai" && parts[i+1] == "ollama" && parts[i+2] == "model" && parts[i+3] == "set" {
						model := parts[i+4]
						botConfig.DefaultAIModel = model
						// Save config live
						configPath, _ := GetConfigPath()
						SaveConfig(configPath, botConfig)
						reply := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("✅ Default AI model set to '%s' (will be used for next /ai)", model))
						bot.Send(reply)
						continue
					}
				}
			}

			// /ai anywhere in the message triggers AI reply
			aiRe := regexp.MustCompile(`/ai(\s|$|[^a-zA-Z0-9_])`)
			if aiRe.MatchString(update.Message.Text) {
				prompt := botConfig.DefaultAIPrompt
				if prompt == "" {
					prompt = "Reply in one concise sentence. Use two only if absolutely necessary, and use as few words as possible."
				}
				model := botConfig.DefaultAIModel
				if model == "" {
					model = "llama2"
				}
				userMsg := update.Message.Text
				aiPrompt := prompt + "\n\nUser message: " + userMsg
				response, err := OllamaChatWithModel(aiPrompt, model)
				if err != nil {
					reply := tgbotapi.NewMessage(update.Message.Chat.ID, "[AI error] "+err.Error())
					bot.Send(reply)
				} else {
					reply := tgbotapi.NewMessage(update.Message.Chat.ID, response)
					bot.Send(reply)
				}
				continue
			}

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

func runShellCommand(cmdline string) (string, error) {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return "", fmt.Errorf("no command provided")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

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

// shouldRespond returns true if the message should trigger a bot reply.
func shouldRespond(text string, chatID int64) bool {
	lower := strings.ToLower(text)
	// Detect mention (case-insensitive, multiple forms)
	botMentioned := strings.Contains(lower, "@ideabglobe_bot") || strings.Contains(lower, "@ideabglobe")
	// Detect /command anywhere in the message (not just at the start, and not if followed by a space)
	isCommand := false
	// Regex: match / followed by at least one word character, not / followed by a space
	commandRe := regexp.MustCompile(`/[a-zA-Z0-9_]+`)
	if commandRe.MatchString(lower) {
		isCommand = true
	}
	// Detect direct message
	isDM := chatID > 0
	// Detect question
	isQuestion := strings.Contains(lower, "?")
	// Detect keywords from auto-reply categories
	if matchAutoReplyCategory(lower) != "" {
		return true
	}
	return botMentioned || isDM || isQuestion || isCommand
}

// matchAutoReplyCategory returns the category if a keyword from auto.json is found in the text, else "".
func matchAutoReplyCategory(text string) string {
	replies := loadAutoReplies()
	for _, ar := range replies {
		if ar.Category == "greeting" && (strings.Contains(text, "hello") || strings.Contains(text, "hi") || strings.Contains(text, "hey")) {
			return "greeting"
		}
		if ar.Category == "issue" && (strings.Contains(text, "issue") || strings.Contains(text, "bug") || strings.Contains(text, "problem") || strings.Contains(text, "error")) {
			return "issue"
		}
		if ar.Category == "feature" && (strings.Contains(text, "feature") || strings.Contains(text, "enhancement") || strings.Contains(text, "request")) {
			return "feature"
		}
		if ar.Category == "question" && (strings.Contains(text, "question") || strings.Contains(text, "how") || strings.Contains(text, "what") || strings.Contains(text, "why") || strings.Contains(text, "?")) {
			return "question"
		}
		if ar.Category == "code" && (strings.Contains(text, "code") || strings.Contains(text, "review") || strings.Contains(text, "debug") || strings.Contains(text, "implementation")) {
			return "code"
		}
	}
	return ""
}

// loadAutoReplies loads auto.json from the app directory.
func loadAutoReplies() []AutoReply {
	path := GetAppDir() + "/auto.json"
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var replies []AutoReply
	if err := json.NewDecoder(f).Decode(&replies); err != nil {
		return nil
	}
	return replies
}

func getSmartReply(text string, msgType string) (string, error) {
	if reply, err := OllamaChat(buildAIPrompt(text, msgType)); err == nil {
		return reply, nil
	}
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
	return replies[len(replies)%3]
}

// RunDefaultBot starts the default bot from config
func RunDefaultBot(cfg *Configs) {
	if cfg == nil {
		log.Fatal("Config is nil")
	}
	if cfg.DefaultBotID == "" {
		log.Fatal("No default bot ID set in config")
	}
	botCfg, ok := cfg.Bots[cfg.DefaultBotID]
	if !ok {
		log.Fatalf("No bot config found for default bot ID: %s", cfg.DefaultBotID)
	}
	if botCfg.Token == "" {
		log.Fatal("No token found for default bot")
	}
	botConfig = cfg // set global config for live updates
	StartBot(botCfg.Token)
}

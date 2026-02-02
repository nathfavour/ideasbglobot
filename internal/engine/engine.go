package engine

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/nathfavour/ideasbglobot/internal"
	"github.com/nathfavour/ideasbglobot/internal/platform"
	"github.com/nathfavour/ideasbglobot/internal/provider"
)

type Engine struct {
	Platforms []platform.Platform
	AI        provider.AIProvider
	Config    *internal.Configs
}

func NewEngine(cfg *internal.Configs, ai provider.AIProvider) *Engine {
	return &Engine{
		Config: cfg,
		AI:     ai,
	}
}

func (e *Engine) AddPlatform(p platform.Platform) {
	e.Platforms = append(e.Platforms, p)
}

func (e *Engine) Start() {
	for _, p := range e.Platforms {
		go func(plt platform.Platform) {
			err := plt.Listen(e.HandleMessage)
			if err != nil {
				log.Printf("Platform %s error: %v", plt.Name(), err)
			}
		}(p)
	}
}

func (e *Engine) HandleMessage(msg platform.IncomingMessage) error {
	msgType := e.detectMessageType(msg.Text)
	
	// Persistence
	internal.SaveMessage(internal.Message{
		ChatID:   msg.ChatID,
		UserID:   msg.UserID,
		Username: msg.Username,
		Text:     msg.Text,
		IsBot:    msg.IsBot,
		Type:     msgType,
		Created:  time.Now(),
	})

	log.Printf("[%s:%s] %s: %s", msg.Platform, strings.ToUpper(msgType), msg.Username, msg.Text)

	// Command Handling
	if msg.IsCommand {
		return e.handleCommand(msg)
	}

	// AI Trigger
	if e.shouldTriggerAI(msg) {
		return e.handleAI(msg, msgType)
	}

	// Auto-reply logic
	if e.shouldRespond(msg) {
		reply, err := e.getSmartReply(msg.Text, msgType)
		if err != nil {
			return err
		}
		return e.reply(msg, reply)
	}

	return nil
}

func (e *Engine) handleCommand(msg platform.IncomingMessage) error {
	switch msg.Command {
	case "run":
		if msg.Args == "" {
			return e.reply(msg, "❌ Please provide a command to run.")
		}
		out, err := e.runShellCommand(msg.Args)
		if err != nil {
			return e.reply(msg, fmt.Sprintf("❌ Error: %v", err))
		}
		return e.reply(msg, fmt.Sprintf("💻 Output:\n%s", out))
	case "status":
		return e.reply(msg, "🤖 Bot is running in modular mode.")
	case "ai":
		// Handle AI model updates (e.g., /ai model set <name>)
		if strings.HasPrefix(msg.Args, "model set ") {
			model := strings.TrimPrefix(msg.Args, "model set ")
			e.Config.DefaultAIModel = model
			path, _ := internal.GetConfigPath()
			internal.SaveConfig(path, e.Config)
			return e.reply(msg, fmt.Sprintf("✅ Default AI model set to '%s'", model))
		}
	}
	return e.reply(msg, fmt.Sprintf("⚡ Command processed: /%s", msg.Command))
}

func (e *Engine) handleAI(msg platform.IncomingMessage, msgType string) error {
	prompt := e.Config.DefaultAIPrompt
	if prompt == "" {
		prompt = "Reply in one concise sentence. Be helpful."
	}
	
	fullPrompt := fmt.Sprintf("%s\nContext: %s\nUser: %s", prompt, msgType, msg.Text)
	response, err := e.AI.Generate(fullPrompt, e.Config.DefaultAIModel)
	if err != nil {
		return e.reply(msg, "[AI Error] "+err.Error())
	}
	return e.reply(msg, response)
}

func (e *Engine) reply(msg platform.IncomingMessage, text string) error {
	for _, p := range e.Platforms {
		if p.Name() == msg.Platform {
			return p.Send(platform.OutgoingMessage{
				ChatID: msg.ChatID,
				Text:   text,
			})
		}
	}
	return fmt.Errorf("platform %s not found", msg.Platform)
}

// Utility methods migrated from internal/bot.go
func (e *Engine) detectMessageType(text string) string {
	text = strings.ToLower(text)
	sswitch {
	case strings.Contains(text, "issue"), strings.Contains(text, "bug"): return "issue"
	case strings.Contains(text, "feature"), strings.Contains(text, "request"): return "feature_request"
	case strings.Contains(text, "question"), strings.Contains(text, "?"): return "question"
	default: return "message"
	}
}

func (e *Engine) shouldTriggerAI(msg platform.IncomingMessage) bool {
	aiRe := regexp.MustCompile(`/ai(\s|$|[^a-zA-Z0-9_])`)
	return aiRe.MatchString(msg.Text)
}

func (e *Engine) shouldRespond(msg platform.IncomingMessage) bool {
	lower := strings.ToLower(msg.Text)
	return strings.Contains(lower, "@bot") || msg.ChatID > 0 || strings.Contains(lower, "?")
}

func (e *Engine) getSmartReply(text string, msgType string) (string, error) {
	// Simple placeholder for existing auto-reply logic
	return "I'm processing your " + msgType, nil
}

func (e *Engine) runShellCommand(cmdline string) (string, error) {
	parts := strings.Fields(cmdline)
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

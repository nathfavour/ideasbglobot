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
	Mode      string // "chat", "agent", "shell"
}

func NewEngine(cfg *internal.Configs, ai provider.AIProvider) *Engine {
	return &Engine{
		Config: cfg,
		AI:     ai,
		Mode:   "chat",
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

	// Command Handling (Global Commands)
	if msg.IsCommand {
		// Specific check for /mode or /status which should always work
		if msg.Command == "mode" || msg.Command == "status" || msg.Command == "start" {
			return e.handleCommand(msg)
		}
	}

	// Route based on mode
	switch e.Mode {
	case "shell":
		return e.handleShellMode(msg, msg.Text)
	case "agent", "chat":
		return e.handleAI(msg, msgType)
	default:
		return e.handleAI(msg, msgType)
	}
}

func (e *Engine) handleCommand(msg platform.IncomingMessage) error {
	switch msg.Command {
	case "run":
		return e.handleShellMode(msg, msg.Args)
	case "status":
		return e.reply(msg, fmt.Sprintf("🤖 Bot Status\nMode: %s\nProvider: %s", strings.ToUpper(e.Mode), e.AI.Name()))
	case "mode":
		newMode := strings.ToLower(strings.TrimSpace(msg.Args))
		if newMode == "chat" || newMode == "agent" || newMode == "shell" {
			e.Mode = newMode
			return e.reply(msg, fmt.Sprintf("✅ Mode switched to: %s", strings.ToUpper(newMode)))
		}
		return e.reply(msg, "Invalid mode. Choose: chat, agent, shell")
	}
	return e.reply(msg, fmt.Sprintf("⚡ Command processed: /%s", msg.Command))
}

func (e *Engine) handleShellMode(msg platform.IncomingMessage, command string) error {
	if command == "" {
		return e.reply(msg, "❌ Please provide a command to run.")
	}

	shellBlacklist := []string{"rm ", "mkfs", "dd ", "fdisk", "reboot", "shutdown"}
	lowerCmd := strings.ToLower(command)
	for _, restricted := range shellBlacklist {
		if strings.Contains(lowerCmd, restricted) {
			return e.replyHTML(msg, fmt.Sprintf("🛑 <b>SECURITY ALERT</b>: Restricted command."))
		}
	}

	e.sendAction(msg.ChatID, platform.ActionTyping)
	out, err := e.runShellCommand(command)
	if err != nil {
		return e.replyHTML(msg, fmt.Sprintf("❌ <b>Error:</b> %v\n<pre>%s</pre>", err, e.escapeHTML(out)))
	}
	return e.replyHTML(msg, fmt.Sprintf("<pre>%s</pre>", e.escapeHTML(out)))
}

func (e *Engine) handleAI(msg platform.IncomingMessage, msgType string) error {
	e.sendAction(msg.ChatID, platform.ActionTyping)

	prompt := msg.Text
	// Remove /ai prefix if present
	prompt = regexp.MustCompile(`(?i)^/ai\s*`).ReplaceAllString(prompt, "")

	rawOutput, err := e.AI.Generate(prompt, e.Mode)
	if err != nil {
		return e.replyHTML(msg, fmt.Sprintf("<b>⚠️ Error</b>\n<pre>%s</pre>", e.escapeHTML(err.Error())))
	}

	// Parse thinking and response (replicating tel logic)
	lines := strings.Split(rawOutput, "\n")
	var thinking []string
	var reply []string

	statusRegex := regexp.MustCompile(`^(\x1b\[[0-9;]*m)?[.+?]\]\s+[A-Z-]+\s+\|.*`)

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "User:") {
			continue
		}

		if statusRegex.MatchString(line) {
			thinking = append(thinking, e.stripANSI(line))
		} else {
			cleanLine := strings.TrimSpace(line)
			if cleanLine != "" {
				reply = append(reply, line)
			}
		}
	}

	var respBuilder strings.Builder
	if len(thinking) > 0 {
		respBuilder.WriteString("<b>💭 Thinking...</b>\n")
		respBuilder.WriteString("<pre>")
		respBuilder.WriteString(e.escapeHTML(strings.Join(thinking, "\n")))
		respBuilder.WriteString("</pre>\n\n")
	}

	finalReply := strings.Join(reply, "\n")
	if finalReply == "" {
		finalReply = "_No clear response captured._"
	}
	respBuilder.WriteString(e.escapeHTML(finalReply))

	return e.replyHTML(msg, respBuilder.String())
}

func (e *Engine) replyHTML(msg platform.IncomingMessage, text string) error {
	for _, p := range e.Platforms {
		if p.Name() == msg.Platform {
			return p.Send(platform.OutgoingMessage{
				ChatID:    msg.ChatID,
				Text:      text,
				ParseMode: platform.ParseModeHTML,
			})
		}
	}
	return fmt.Errorf("platform %s not found", msg.Platform)
}

func (e *Engine) sendAction(chatID int64, action string) {
	for _, p := range e.Platforms {
		p.SendAction(chatID, action)
	}
}

func (e *Engine) escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func (e *Engine) stripANSI(str string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(str, "")
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
	switch {
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

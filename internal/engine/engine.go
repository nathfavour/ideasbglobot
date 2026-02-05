package engine

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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
	Scheduler *internal.Scheduler
	Mode      string // "chat", "agent", "shell"
}

func NewEngine(cfg *internal.Configs, ai provider.AIProvider) *Engine {
	e := &Engine{
		Config: cfg,
		AI:     ai,
		Mode:   "chat",
	}
	e.Scheduler = &internal.Scheduler{
		NotifyFunc: func(chatID int64, message string) {
			e.broadcast(chatID, message)
		},
	}
	return e
}

func (e *Engine) broadcast(chatID int64, text string) {
	log.Printf("Broadcasting to %d: %s", chatID, text)
	for _, p := range e.Platforms {
		err := p.Send(platform.OutgoingMessage{
			ChatID:    chatID,
			Text:      text,
			ParseMode: platform.ParseModeHTML,
		})
		if err != nil {
			log.Printf("Broadcast error on %s: %v", p.Name(), err)
		}
	}
}

func (e *Engine) AddPlatform(p platform.Platform) {
	e.Platforms = append(e.Platforms, p)
}

func (e *Engine) Start() {
	if e.Scheduler != nil {
		e.Scheduler.Start()
	}
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
	log.Printf("Received message from %d (%s): %s", msg.ChatID, msg.Username, msg.Text)

	// Lockdown Logic
	if len(e.Config.AllowedIDs) == 0 {
		e.Config.AllowedIDs = append(e.Config.AllowedIDs, msg.ChatID)
		path, _ := internal.GetConfigPath()
		internal.SaveConfig(path, e.Config)
		log.Printf("First message received from %d. Bot locked to this ID.", msg.ChatID)
	}

	isAllowed := false
	for _, id := range e.Config.AllowedIDs {
		if id == msg.ChatID {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		log.Printf("Unauthorized message from %d (User: %s). Ignored.", msg.ChatID, msg.Username)
		e.reply(msg, "⛔ **Access Denied**: You are not authorized to use this bot.")
		return nil
	}

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

	// Smart Filter: Should we respond to this message?
	if !e.shouldRespond(msg) {
		log.Printf("Ignoring message from %d: Not directed at bot.", msg.ChatID)
		return nil
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

	var toolOutputs []string
	maxTurns := 3

	for turn := 0; turn < maxTurns; turn++ {
		// 1. Build Context
		var contextBuilder strings.Builder
		
		// Add System Instructions
		contextBuilder.WriteString("### SYSTEM INSTRUCTIONS\n")
		contextBuilder.WriteString("You are ideasbglobe, a founder-minded assistant. You are currently in a group chat with Benjamin and Nathaniel.\n")
		contextBuilder.WriteString("STRICT SILENCE RULE:\n")
		contextBuilder.WriteString("1. If the message is general chatter (e.g., 'Ok', 'Alright', 'Thanks'), output ONLY [IGNORE]. Do NOT acknowledge it.\n")
		contextBuilder.WriteString("2. If you are using a tool, output ONLY the tool tag. NEVER explain what you are about to do or what you are thinking.\n")
		contextBuilder.WriteString("3. NEVER output internal planning, bold headers like '**Planning...**', or status updates to the user.\n")
		contextBuilder.WriteString("4. Your ONLY tools are: [READ_FILE], [LIST_FILES], [FETCH_URL], [GIT_STATUS], [LEARN], [TASK]. Do NOT attempt to use any other tags.\n")
		contextBuilder.WriteString("\n")
		contextBuilder.WriteString(e.Config.DefaultAIPrompt)
		contextBuilder.WriteString("\n\n")

		// Add Persistent Context and Learned Facts
		ctxData, _ := internal.FormatContextForPrompt(msg.ChatID)
		contextBuilder.WriteString(ctxData)

		// Add Semantic Memory (Relevant Past Messages)
		// Extract keywords from prompt (simple split)
		keywords := strings.Fields(prompt)
		if len(keywords) > 0 {
			relevant, _ := internal.SearchMessages(msg.ChatID, keywords[0], 5)
			if len(relevant) > 0 {
				contextBuilder.WriteString("### RELEVANT PAST MESSAGES\n")
				for _, m := range relevant {
					role := "User"
					if m.IsBot {
						role = "Assistant"
					}
					contextBuilder.WriteString(fmt.Sprintf("%s: %s\n", role, m.Text))
				}
				contextBuilder.WriteString("\n")
			}
		}

		// Add Recent Conversation History
		history, err := internal.GetChatHistory(msg.ChatID, 10)
		if err == nil && len(history) > 0 {
			contextBuilder.WriteString("### RECENT CONVERSATION HISTORY\n")
			for _, m := range history {
				role := "User"
				if m.IsBot {
					role = "Assistant"
				}
				if m.Text == msg.Text && !m.IsBot {
					continue
				}
				contextBuilder.WriteString(fmt.Sprintf("%s: %s\n", role, m.Text))
			}
			contextBuilder.WriteString("\n")
		}

		if len(toolOutputs) > 0 {
			contextBuilder.WriteString("### TOOL OUTPUTS\n")
			for _, out := range toolOutputs {
				contextBuilder.WriteString(out + "\n")
			}
			contextBuilder.WriteString("\n")
		}

		contextBuilder.WriteString("### NEW USER INPUT\n")
		contextBuilder.WriteString(prompt)
		contextBuilder.WriteString("\n\n")
		
		contextBuilder.WriteString("### RESPONSE GUIDELINES\n")
		contextBuilder.WriteString("You are an AGENT. Gather info silently using tags. Provide ONLY the final answer.\n")
		contextBuilder.WriteString("- [READ_FILE: path]\n")
		contextBuilder.WriteString("- [LIST_FILES: path]\n")
		contextBuilder.WriteString("- [FETCH_URL: url]\n")
		contextBuilder.WriteString("- [GIT_STATUS]\n")
		contextBuilder.WriteString("- [LEARN: key=value]\n")
		contextBuilder.WriteString("- [TASK: title | description | YYYY-MM-DD HH:MM]\n")
		contextBuilder.WriteString("\n")
		contextBuilder.WriteString("NO CHATTER: If no response is strictly necessary or you're just acknowledging, use [IGNORE].\n")
		contextBuilder.WriteString("NO REASONING: Do not tell the user what you are doing. Just do it.\n")

		fullPrompt := contextBuilder.String()

		rawOutput, err := e.AI.Generate(fullPrompt, e.Mode)
		if err != nil {
			return e.replyHTML(msg, fmt.Sprintf("<b>⚠️ Error</b>\n<pre>%s</pre>", e.escapeHTML(err.Error())))
		}

		// Check for tools
		hasTools := false
		
		// Process READ_FILE
		readFileRegex := regexp.MustCompile(`\[READ_FILE:\s*([^\]]+)\]`)
		if match := readFileRegex.FindStringSubmatch(rawOutput); len(match) > 1 {
			path := strings.TrimSpace(match[1])
			content, err := os.ReadFile(path)
			if err != nil {
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_ERROR: READ_FILE %s: %v]", path, err))
			} else {
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_OUTPUT: READ_FILE %s]\n%s", path, string(content)))
			}
			hasTools = true
		}

		// Process LIST_FILES
		listFilesRegex := regexp.MustCompile(`\[LIST_FILES:\s*([^\]]+)\]`)
		if match := listFilesRegex.FindStringSubmatch(rawOutput); len(match) > 1 {
			path := strings.TrimSpace(match[1])
			files, err := os.ReadDir(path)
			if err != nil {
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_ERROR: LIST_FILES %s: %v]", path, err))
			} else {
				var fileList []string
				for _, f := range files {
					fileList = append(fileList, f.Name())
				}
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_OUTPUT: LIST_FILES %s]\n%s", path, strings.Join(fileList, ", ")))
			}
			hasTools = true
		}

		// Process FETCH_URL
		fetchUrlRegex := regexp.MustCompile(`\[FETCH_URL:\s*([^\]]+)\]`)
		if match := fetchUrlRegex.FindStringSubmatch(rawOutput); len(match) > 1 {
			url := strings.TrimSpace(match[1])
			resp, err := http.Get(url)
			if err != nil {
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_ERROR: FETCH_URL %s: %v]", url, err))
			} else {
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				// Limit output to avoid token bloat
				content := string(body)
				if len(content) > 2000 {
					content = content[:2000] + "... (truncated)"
				}
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_OUTPUT: FETCH_URL %s]\n%s", url, content))
			}
			hasTools = true
		}

		// Process GIT_STATUS
		if strings.Contains(rawOutput, "[GIT_STATUS]") {
			out, err := e.runShellCommand("git status")
			if err != nil {
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_ERROR: GIT_STATUS: %v]", err))
			} else {
				toolOutputs = append(toolOutputs, fmt.Sprintf("[TOOL_OUTPUT: GIT_STATUS]\n%s", out))
			}
			hasTools = true
		}

				if !hasTools {

					// Final response

					e.processLearningTags(msg.ChatID, rawOutput)

					e.processTaskTags(msg.ChatID, rawOutput)

					cleanOutput := e.stripLearningTags(rawOutput)

					cleanOutput = e.stripTaskTags(cleanOutput)

					cleanOutput = regexp.MustCompile(`\[READ_FILE:\s*[^\]]+\]`).ReplaceAllString(cleanOutput, "")

					cleanOutput = regexp.MustCompile(`\[LIST_FILES:\s*[^\]]+\]`).ReplaceAllString(cleanOutput, "")

					cleanOutput = regexp.MustCompile(`\[FETCH_URL:\s*[^\]]+\]`).ReplaceAllString(cleanOutput, "")

					cleanOutput = strings.ReplaceAll(cleanOutput, "[GIT_STATUS]", "")

					

					if strings.Contains(cleanOutput, "[IGNORE]") || strings.TrimSpace(cleanOutput) == "" {

						log.Printf("AI requested to ignore or produced no text. Remaining silent.")

						return nil

					}

		

					return e.replyHTML(msg, e.escapeHTML(cleanOutput))

				}

		
		
		// If it has tools, loop again with tool results
		log.Printf("Tool used, entering turn %d", turn+1)
	}

	return e.reply(msg, "Max turns reached without final answer.")
}

func (e *Engine) processLearningTags(chatID int64, output string) {
	// [LEARN: key=value]
	learnRegex := regexp.MustCompile(`\[LEARN:\s*([^=]+)=([^\]]+)\]`)
	matches := learnRegex.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) == 3 {
			key := strings.TrimSpace(m[1])
			val := strings.TrimSpace(m[2])
			internal.SaveContextFact(internal.ContextFact{
				ChatID:     chatID,
				Key:        key,
				Value:      val,
				Source:     "ai_inference",
				Confidence: 0.8,
			})
			log.Printf("Learned fact: %s = %s", key, val)
		}
	}

	// [UNLEARN: key]
	unlearnRegex := regexp.MustCompile(`\[UNLEARN:\s*([^\]]+)\]`)
	unlearnMatches := unlearnRegex.FindAllStringSubmatch(output, -1)
	for _, m := range unlearnMatches {
		if len(m) == 2 {
			key := strings.TrimSpace(m[1])
			internal.DeleteContextFact(chatID, key)
			log.Printf("Unlearned fact: %s", key)
		}
	}

	// [UPDATE_CONTEXT: new content]
	// This might be tricky if the content has newlines. Let's use a more robust approach if possible.
	// For now, simple regex.
	updateRegex := regexp.MustCompile(`(?s)\[UPDATE_CONTEXT:\s*(.*?)\]`)
	updateMatch := updateRegex.FindStringSubmatch(output)
	if len(updateMatch) == 2 {
		newContext := strings.TrimSpace(updateMatch[1])
		if newContext != "" {
			err := internal.WriteContextFile(newContext)
			if err != nil {
				log.Printf("Error updating CONTEXT.md: %v", err)
			} else {
				log.Printf("Updated CONTEXT.md")
			}
		}
	}
}

func (e *Engine) stripLearningTags(output string) string {
	output = regexp.MustCompile(`\[LEARN:\s*[^\]]+\]`).ReplaceAllString(output, "")
	output = regexp.MustCompile(`\[UNLEARN:\s*[^\]]+\]`).ReplaceAllString(output, "")
	output = regexp.MustCompile(`(?s)\[UPDATE_CONTEXT:\s*.*?\]`).ReplaceAllString(output, "")
	// Also strip bold headers that look like thinking/planning
	output = regexp.MustCompile(`(?m)^\*\*.*?\*\*.*$`).ReplaceAllString(output, "")
	return strings.TrimSpace(output)
}

func (e *Engine) processTaskTags(chatID int64, output string) {
	// [TASK: title | description | YYYY-MM-DD HH:MM]
	taskRegex := regexp.MustCompile(`\[TASK:\s*([^|]+)\|([^|]+)\|([^\]]+)\]`)
	matches := taskRegex.FindAllStringSubmatch(output, -1)
	for _, m := range matches {
		if len(m) == 4 {
			title := strings.TrimSpace(m[1])
			desc := strings.TrimSpace(m[2])
			dueStr := strings.TrimSpace(m[3])
			
			dueAt, err := time.Parse("2006-01-02 15:04", dueStr)
			if err != nil {
				log.Printf("Error parsing task date %s: %v", dueStr, err)
				continue
			}

			err = internal.SaveTask(internal.Task{
				ChatID:      chatID,
				Title:       title,
				Description: desc,
				DueAt:       dueAt,
			})
			if err != nil {
				log.Printf("Error saving task: %v", err)
			} else {
				log.Printf("Scheduled task: %s at %s", title, dueAt)
			}
		}
	}
}

func (e *Engine) stripTaskTags(output string) string {
	return strings.TrimSpace(regexp.MustCompile(`\[TASK:\s*[^\]]+\]`).ReplaceAllString(output, ""))
}

func (e *Engine) replyHTML(msg platform.IncomingMessage, text string) error {
	log.Printf("Replying to %d (HTML): %s", msg.ChatID, text)
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
	log.Printf("Replying to %d: %s", msg.ChatID, text)
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
	// Private Chat: Always respond
	if msg.ChatID > 0 {
		return true
	}

	// Commands: Always respond (they start with /)
	if msg.IsCommand {
		return true
	}

	// Mentions: Always respond if specifically tagged
	if msg.IsMentioned {
		return true
	}

	// Replies: Respond ONLY if replying directly to the bot's own message
	if msg.IsReplyToBot {
		// Even if it's a reply, if the text is too short (like "alright", "ok", "thx"), 
		// maybe we should still be cautious. But for now, direct replies are usually intentional.
		lower := strings.ToLower(strings.TrimSpace(msg.Text))
		if len(lower) < 3 && lower != "hi" && lower != "no" && lower != "ok" {
			return false
		}
		return true
	}
	
	return false
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

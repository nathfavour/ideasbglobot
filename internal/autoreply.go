package internal

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

type AutoReply struct {
	ID       int    `json:"id"`
	Category string `json:"category"`
	Reply    string `json:"reply"`
	Context  string `json:"context"`
}

func EnsureAutoReplies() {
	autoRepliesPath := filepath.Join(GetAppDir(), "auto.json")
	dir := filepath.Dir(autoRepliesPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0700)
	}
	if _, err := os.Stat(autoRepliesPath); err == nil {
		f, err := os.Open(autoRepliesPath)
		if err == nil {
			defer f.Close()
			var replies []AutoReply
			if json.NewDecoder(f).Decode(&replies) == nil && len(replies) > 0 {
				return // valid
			}
		}
	}
	GenerateAutoReplies()
}

func GenerateAutoReplies() {
	autoRepliesPath := filepath.Join(GetAppDir(), "auto.json")
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

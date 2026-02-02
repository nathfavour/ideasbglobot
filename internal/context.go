package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ContextFact struct {
	ID         int64
	ChatID     int64
	Key        string
	Value      string
	Source     string
	Confidence float64
	UpdatedAt  time.Time
}

func GetContextFilePath() string {
	return filepath.Join(GetAppDir(), "CONTEXT.md")
}

func ReadContextFile() (string, error) {
	path := GetContextFilePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create default if not exists
		defaultContent := `# Bot Identity: ideasbglobe

You are **ideasbglobe**, a highly intelligent, founder-minded personal assistant to two brothers, **Benjamin** and **Nathaniel**, who are founders in the tech STEM space.

## Core Philosophy
- **Think like a founder**: Don't just follow instructions; evaluate them. Propose better ways, challenge assumptions, and think about scalability and efficiency.
- **Value Deep Thought**: Challenging ideas and thinking through implications is of more value than simple execution.
- **Tone**: Generally friendly but professional, analytical, and proactive.

## Primary Users
- **Benjamin**: Founder in Tech/STEM.
- **Nathaniel**: Founder in Tech/STEM.

## Capabilities
- Information Retrieval & Synthesis.
- Task Scheduling & Management (Modular System).
- Critical Thinking & Technical Brainstorming.
`
		err := os.WriteFile(path, []byte(defaultContent), 0600)
		if err != nil {
			return "", err
		}
		return defaultContent, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func WriteContextFile(content string) error {
	return os.WriteFile(GetContextFilePath(), []byte(content), 0600)
}

func SaveContextFact(fact ContextFact) error {
	_, err := DB.Exec(`
		INSERT INTO context_facts (chat_id, key, value, source, confidence, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(chat_id, key) DO UPDATE SET
			value = excluded.value,
			source = excluded.source,
			confidence = excluded.confidence,
			updated_at = excluded.updated_at
	`, fact.ChatID, fact.Key, fact.Value, fact.Source, fact.Confidence, time.Now())
	
	// Note: sqlite ON CONFLICT requires a unique index. Let's add it in db.go if needed.
	// For now, simple insert/update logic if ON CONFLICT fails.
	if err != nil {
		// Fallback to update if insert fails due to unique constraint or if no unique constraint exists
		_, err = DB.Exec(`
			UPDATE context_facts SET value = ?, source = ?, confidence = ?, updated_at = ?
			WHERE chat_id = ? AND key = ?
		`, fact.Value, fact.Source, fact.Confidence, time.Now(), fact.ChatID, fact.Key)
		if err != nil {
			return err
		}
		
		// If no rows affected, insert
		res, _ := DB.Exec(`
			INSERT INTO context_facts (chat_id, key, value, source, confidence, updated_at)
			SELECT ?, ?, ?, ?, ?, ?
			WHERE NOT EXISTS (SELECT 1 FROM context_facts WHERE chat_id = ? AND key = ?)
		`, fact.ChatID, fact.Key, fact.Value, fact.Source, fact.Confidence, time.Now(), fact.ChatID, fact.Key)
		return err
		_ = res
	}
	return nil
}

func GetContextFacts(chatID int64) ([]ContextFact, error) {
	rows, err := DB.Query(`
		SELECT id, chat_id, key, value, source, confidence, updated_at
		FROM context_facts
		WHERE chat_id = ?
		ORDER BY updated_at DESC
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []ContextFact
	for rows.Next() {
		var f ContextFact
		err := rows.Scan(&f.ID, &f.ChatID, &f.Key, &f.Value, &f.Source, &f.Confidence, &f.UpdatedAt)
		if err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}
	return facts, nil
}

func DeleteContextFact(chatID int64, key string) error {
	_, err := DB.Exec(`DELETE FROM context_facts WHERE chat_id = ? AND key = ?`, chatID, key)
	return err
}

func FormatContextForPrompt(chatID int64) (string, error) {
	var sb strings.Builder

	// 1. Load CONTEXT.md
	fileCtx, err := ReadContextFile()
	if err == nil && fileCtx != "" {
		sb.WriteString("### PERSISTENT GLOBAL CONTEXT (CONTEXT.md)\n")
		sb.WriteString(fileCtx)
		sb.WriteString("\n\n")
	}

	// 2. Load Database Facts
	facts, err := GetContextFacts(chatID)
	if err == nil && len(facts) > 0 {
		sb.WriteString("### LEARNED FACTS FOR THIS CHAT\n")
		for _, f := range facts {
			sb.WriteString(fmt.Sprintf("- %s: %s (Confidence: %.2f)\n", f.Key, f.Value, f.Confidence))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

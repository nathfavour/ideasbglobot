package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type VibeAuraProvider struct {
	BinaryPath string
}

func NewVibeAuraProvider(path string) *VibeAuraProvider {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".local/bin/vibeaura")
	}
	return &VibeAuraProvider{BinaryPath: path}
}

func (p *VibeAuraProvider) Name() string {
	return "vibeaura"
}

func (p *VibeAuraProvider) Generate(prompt string, mode string) (string, error) {
	// mode here can be "chat" or "agent"
	vibePrompt := prompt
	if prompt != "" {
		if mode == "chat" {
			vibePrompt = "CONVERSATIONAL MODE: Provide a concise response. Minimal tools.\n\n" + prompt
		} else if mode == "agent" {
			vibePrompt = "AGENT MODE: Use tools to solve the request.\n\n" + prompt
		}
	}

	args := []string{"direct", "--verbose=false"}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, p.BinaryPath, args...)
	cmd.Stdin = strings.NewReader(vibePrompt)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("vibeaura error: %w\n%s", err, string(output))
	}

	// Clean REPL noise
	res := string(output)
	res = strings.ReplaceAll(res, "--- VibeAuracle Direct REPL ---", "")
	res = strings.ReplaceAll(res, "Type 'exit' to quit, 'clear' to clear screen.", "")
	
	// Remove thought markers like "> **Thinking...**" but keep the text after them if it's not another marker
	reThoughtMarkers := regexp.MustCompile(`(?m)^> \*\*.*?\*\*`)
	res = reThoughtMarkers.ReplaceAllString(res, "")
	
	// Remove empty prompts "> "
	res = strings.ReplaceAll(res, "> ", "")

	return strings.TrimSpace(res), nil
}

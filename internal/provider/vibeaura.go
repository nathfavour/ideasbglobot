package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
	// mode here is ignored for the prompt prefix to avoid triggering verbose 'Agent Mode' explanations
	vibePrompt := prompt

	args := []string{"direct", "--verbose=false"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	
	// AGGRESSIVE PRUNING: Remove ANY line that starts with bold text or markers (Thinking, Planning, etc.)
	// This stops the 'The latest user input instructs...' or '**Planning...**' chatter.
	reReasoning := regexp.MustCompile(`(?m)^(\*\*|#|> ).*$`)
	res = reReasoning.ReplaceAllString(res, "")
	
	// Remove empty prompts "> "
	res = strings.ReplaceAll(res, "> ", "")

	return strings.TrimSpace(res), nil
}

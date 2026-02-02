package provider

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type VibeAuraProvider struct {
	BinaryPath string
}

func NewVibeAuraProvider(path string) *VibeAuraProvider {
	if path == "" {
		path = "/home/nathfavour/.local/bin/vibeaura"
	}
	return &VibeAuraProvider{BinaryPath: path}
}

func (p *VibeAuraProvider) Name() string {
	return "vibeaura"
}

func (p *VibeAuraProvider) Generate(prompt string, mode string) (string, error) {
	// mode here can be "chat" or "agent"
	vibePrompt := prompt
	if mode == "chat" {
		vibePrompt = "CONVERSATIONAL MODE: Provide a concise response. Minimal tools.\n\n" + prompt
	} else if mode == "agent" {
		vibePrompt = "AGENT MODE: Use tools to solve the request.\n\n" + prompt
	}

	args := []string{"direct", "--verbose=false", "--non-interactive"}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, p.BinaryPath, args...)
	cmd.Stdin = strings.NewReader(vibePrompt)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("vibeaura error: %w\n%s", err, string(output))
	}

	return string(output), nil
}

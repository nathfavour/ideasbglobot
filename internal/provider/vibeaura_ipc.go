package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

type VibeAuraIPCProvider struct {
	SocketPath string
}

func NewVibeAuraIPCProvider(path string) *VibeAuraIPCProvider {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".vibeauracle/vibeaura.sock")
	}
	return &VibeAuraIPCProvider{SocketPath: path}
}

func (p *VibeAuraIPCProvider) Name() string {
	return "vibeaura-ipc"
}

type ipcRequest struct {
	Type    string                 `json:"type"`
	Method  string                 `json:"method"`
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

type ipcResponse struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

func (p *VibeAuraIPCProvider) Generate(prompt string, mode string) (string, error) {
	conn, err := net.Dial("unix", p.SocketPath)
	if err != nil {
		return "", fmt.Errorf("failed to connect to vibeaura socket: %w", err)
	}
	defer conn.Close()

	// Set a timeout for the entire operation
	conn.SetDeadline(time.Now().Add(2 * time.Minute))

	req := ipcRequest{
		Type:   "request",
		Method: "query",
		ID:     fmt.Sprintf("bot-%d", time.Now().UnixNano()),
		Payload: map[string]interface{}{
			"content": prompt,
			"intent":  "ask", // Use 'ask' to keep it concise, or 'crud' for agentic
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	// Send request with newline as delimiter (LDJSON)
	_, err = conn.Write(append(data, '\n'))
	if err != nil {
		return "", fmt.Errorf("failed to write to socket: %w", err)
	}

	// Read response (one line)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read from socket: %w", err)
	}

	var resp ipcResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal socket response: %w", err)
	}

	if resp.Type == "error" {
		msg := "unknown error"
		if m, ok := resp.Payload["message"].(string); ok {
			msg = m
		}
		return "", fmt.Errorf("vibeaura ipc error: %s", msg)
	}

	content, ok := resp.Payload["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response payload: missing content")
	}

	return content, nil
}

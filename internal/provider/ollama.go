package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OllamaProvider struct {
	BaseURL string
}

func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{BaseURL: baseURL}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Generate(prompt string, model string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", p.BaseURL)
	payload := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(respBody, &res); err != nil {
		return "", err
	}

	return strings.TrimSpace(res.Response), nil
}

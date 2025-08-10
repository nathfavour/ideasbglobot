package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func OllamaChat(prompt string) (string, error) {
	cfg, err := EnsureConfigFile()
	if err != nil {
		return "", err
	}
	model := cfg.DefaultAIModel
	return OllamaChatWithModel(prompt, model)
}

func OllamaChatWithModel(prompt, model string) (string, error) {
	ollamaURL := "http://localhost:11434/api/generate"
	payload := `{"model":` + jsonString(model) + `,"prompt":` + jsonString(prompt) + `,"stream":false}`
	req, err := http.NewRequest("POST", ollamaURL, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	type ollamaResp struct {
		Response string `json:"response"`
	}
	var o ollamaResp
	if err := json.Unmarshal(body, &o); err != nil {
		return "", fmt.Errorf("ollama response: %s", string(body))
	}
	return strings.TrimSpace(o.Response), nil
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

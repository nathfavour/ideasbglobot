package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nathfavour/ideasbglobot/internal"
)

var ollamaModel string = "llama3" // default, can be changed by user

var AiCmd = &cobra.Command{
	Use:   "ai",
	Short: "Interact with Ollama AI models",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Current Ollama model: %s\n", ollamaModel)
	},
}

var ollamaModelSetCmd = &cobra.Command{
	Use:   "ollama model set",
	Short: "Scan Ollama installation, list models, and select model for AI functionalities",
	Run: func(cmd *cobra.Command, args []string) {
		models, err := listOllamaModels()
		if err != nil {
			fmt.Printf("Error listing Ollama models: %v\n", err)
			return
		}
		if len(models) == 0 {
			fmt.Println("No Ollama models found.")
			return
		}
		fmt.Println("Available Ollama models:")
		for i, m := range models {
			fmt.Printf("  [%d] %s\n", i+1, m)
		}
		fmt.Print("Select model number to use: ")
		reader := bufio.NewReader(os.Stdin)
		choiceStr, _ := reader.ReadString('\n')
		choiceStr = strings.TrimSpace(choiceStr)
		var idx int
		fmt.Sscanf(choiceStr, "%d", &idx)
		if idx < 1 || idx > len(models) {
			fmt.Println("Invalid selection.")
			return
		}
		ollamaModel = models[idx-1]
		fmt.Printf("Ollama model set to: %s\n", ollamaModel)

		// Persist to config
		cfg, err := internal.EnsureConfigFile()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		cfg.DefaultAIModel = ollamaModel
		configPath, err := internal.GetConfigPath()
		if err != nil {
			fmt.Printf("Error getting config path: %v\n", err)
			return
		}
		if err := internal.SaveConfig(configPath, cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Println("Model selection persisted to config.")
	},
}

func listOllamaModels() ([]string, error) {
	resp, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}
	return models, nil
}

// Exported function to get the current ollama model
func OllamaModel() string {
	return ollamaModel
}

func init() {
	AiCmd.AddCommand(ollamaModelSetCmd)
}

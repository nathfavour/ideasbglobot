package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var AiCmd = &cobra.Command{
	Use:   "ai",
	Short: "Interact with Ollama AI models",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please provide a prompt for the AI model.")
			return
		}
		prompt := args[0]
		// Call Ollama API (see internal/ollama.go)
		fmt.Printf("Querying Ollama AI: %s\n", prompt)
		// ...existing code...
	},
}

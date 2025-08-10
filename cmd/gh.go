package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var GhCmd = &cobra.Command{
	Use:   "gh",
	Short: "Run GitHub CLI commands",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please provide a gh command.")
			return
		}
		out, err := exec.Command("gh", args...).CombinedOutput()
		if err != nil {
			fmt.Printf("gh error: %v\n", err)
		}
		fmt.Println(string(out))
	},
}

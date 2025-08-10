package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var GitCmd = &cobra.Command{
	Use:   "git",
	Short: "Run git commands",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Please provide a git command.")
			return
		}
		out, err := exec.Command("git", args...).CombinedOutput()
		if err != nil {
			fmt.Printf("git error: %v\n", err)
		}
		fmt.Println(string(out))
	},
}

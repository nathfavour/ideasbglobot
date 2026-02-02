package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathfavour/ideasbglobot/internal"
	"github.com/spf13/cobra"
)

var BotCmd = &cobra.Command{
	Use:   "bot",
	Short: "Start and control the Telegram bot",
	Run: func(cmd *cobra.Command, args []string) {
		token := os.Getenv("TELEGRAM_BOT_TOKEN")
		if token != "" {
			fmt.Println("Starting Telegram bot...")
			// Start bot logic here (see bot/telegram.go)
		} else {
			fmt.Println("TELEGRAM_BOT_TOKEN not set")
		}
	},
}

var botAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Telegram bot configuration",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter bot ID (unique name): ")
		id, _ := reader.ReadString('\n')
		id = strings.TrimSpace(id)
		fmt.Print("Enter bot token: ")
		token, _ := reader.ReadString('\n')
		token = strings.TrimSpace(token)

		// Load config
		cfgPath, err := internal.GetConfigPath()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		cfg, err := internal.EnsureConfigFile()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		if cfg.Bots == nil {
			cfg.Bots = map[string]internal.BotConfig{}
		}
		cfg.Bots[id] = internal.BotConfig{ID: id, Token: token}

		fmt.Print("Set this bot as default? (y/N): ")
		setDefault, _ := reader.ReadString('\n')
		setDefault = strings.TrimSpace(strings.ToLower(setDefault))
		if setDefault == "y" || setDefault == "yes" {
			cfg.DefaultBotID = id
		}

		// Save config
		if err := internal.SaveConfig(cfgPath, cfg); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			return
		}
		fmt.Println("Bot added successfully.")
	},
}

var botUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update ideasbglobot to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		repoURL := "https://github.com/nathfavour/ideasbglobot"
		appDir := internal.GetAppDir()
		srcDir := filepath.Join(appDir, "src")
		installDir := filepath.Join(os.Getenv("HOME"), ".local/bin")
		appName := "ideasbglobot"

		fmt.Println("⏳ Checking for updates...")

		if _, err := os.Stat(filepath.Join(srcDir, ".git")); os.IsNotExist(err) {
			fmt.Println("📦 Source not found. Performing full installation...")
			updateCmd := "curl -sSL https://raw.githubusercontent.com/nathfavour/ideasbglobot/main/install.sh | bash"
			execCmd := internal.NewCommand("bash", "-c", updateCmd)
			output, _ := execCmd.CombinedOutput()
			fmt.Println(string(output))
			return
		}

		// Get remote HEAD commit
		remoteCmd := internal.NewCommand("git", "ls-remote", repoURL, "HEAD")
		remoteOut, err := remoteCmd.Output()
		if err != nil {
			fmt.Printf("❌ Failed to check remote: %v\n", err)
			return
		}
		remoteHash := strings.Fields(string(remoteOut))[0]

		// Get local HEAD commit
		localCmd := internal.NewCommand("git", "-C", srcDir, "rev-parse", "HEAD")
		localOut, err := localCmd.Output()
		if err != nil {
			fmt.Printf("❌ Failed to check local: %v\n", err)
			return
		}
		localHash := strings.TrimSpace(string(localOut))

		if remoteHash == localHash {
			fmt.Println("✅ Already up to date.")
			return
		}

		fmt.Printf("🔄 New version detected (%s). Updating...\n", remoteHash[:7])

		// Pull latest
		pullCmd := internal.NewCommand("git", "-C", srcDir, "pull")
		if out, err := pullCmd.CombinedOutput(); err != nil {
			fmt.Printf("❌ Pull failed: %v\n%s\n", err, string(out))
			return
		}

		// Build
		fmt.Println("🛠 Building...")
		buildCmd := internal.NewCommand("go", "-C", srcDir, "build", "-o", appName, "main.go")
		if out, err := buildCmd.CombinedOutput(); err != nil {
			fmt.Printf("❌ Build failed: %v\n%s\n", err, string(out))
			return
		}

		// Install
		fmt.Printf("📥 Installing to %s...\n", installDir)
		installPath := filepath.Join(installDir, appName)
		srcBinary := filepath.Join(srcDir, appName)
		
		// Move/Copy binary
		input, err := os.ReadFile(srcBinary)
		if err != nil {
			fmt.Printf("❌ Failed to read built binary: %v\n", err)
			return
		}
		err = os.WriteFile(installPath, input, 0755)
		if err != nil {
			fmt.Printf("❌ Failed to install binary: %v\n", err)
			return
		}

		fmt.Println("✨ Update complete!")
	},
}

// getConfigPath is now in internal/config.go as GetConfigPath

// Use BotConfig and Configs from internal/config.go

func loadConfig(path string) (*internal.Configs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg internal.Configs
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(path string, cfg *internal.Configs) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func init() {
	BotCmd.AddCommand(botAddCmd)
	BotCmd.AddCommand(botUpdateCmd)
}

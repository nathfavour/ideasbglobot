package config

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
)

type BotConfig struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type Configs struct {
	DefaultBotID string               `json:"default_bot_id"`
	Bots         map[string]BotConfig `json:"bots"`
}

func getConfigPath() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(usr.HomeDir, ".ideasbglobe")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "configs.json"), nil
}

func EnsureConfigFile() (*Configs, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &Configs{
			DefaultBotID: "",
			Bots:         map[string]BotConfig{},
		}
		if err := SaveConfig(configPath, defaultConfig); err != nil {
			return nil, err
		}
		return defaultConfig, nil
	}
	f, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Configs
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	changed := false
	if cfg.Bots == nil {
		cfg.Bots = map[string]BotConfig{}
		changed = true
	}
	if changed {
		if err := SaveConfig(configPath, &cfg); err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Configs) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func GetAppDir() string {
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, ".ideasbglobe")
}

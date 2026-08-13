package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	gitflowConfigDirMode  os.FileMode = 0o700
	gitflowConfigFileMode os.FileMode = 0o600
)

func configLocation() (string, string, bool) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil || home == "" {
			return "", "", false
		}
		configDir = filepath.Join(home, ".config")
	}
	appDir := filepath.Join(configDir, "gitflow")
	return appDir, filepath.Join(appDir, "config.json"), true
}

func loadConfig() AppConfig {
	cfg := AppConfig{DefaultTab: 0, RefreshInterval: 10, PrimaryColor: "212", BorderColor: "63"}
	configDir, configPath, ok := configLocation()
	if !ok {
		return cfg
	}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return AppConfig{DefaultTab: 0, RefreshInterval: 10, PrimaryColor: "212", BorderColor: "63"}
		}
		return cfg
	}
	if err := os.MkdirAll(configDir, gitflowConfigDirMode); err != nil {
		return cfg
	}
	_ = os.Chmod(configDir, gitflowConfigDirMode)
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = writeConfigAtomically(configPath, data)
	}
	return cfg
}

func saveConfig(cfg AppConfig) {
	configDir, configPath, ok := configLocation()
	if !ok || os.MkdirAll(configDir, gitflowConfigDirMode) != nil {
		return
	}
	_ = os.Chmod(configDir, gitflowConfigDirMode)
	if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
		_ = writeConfigAtomically(configPath, data)
	}
}

func writeConfigAtomically(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(gitflowConfigFileMode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

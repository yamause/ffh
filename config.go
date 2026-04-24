package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const defaultHostsPath = "/etc/hosts"

// configFilePath returns ~/.config/ffh/config
func configFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ffh", "config")
}

// loadConfig parses ~/.config/ffh/config and returns key-value pairs.
// Format: key = value (lines starting with # are comments)
func loadConfig() map[string]string {
	cfg := make(map[string]string)
	f, err := os.Open(configFilePath())
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		cfg[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return cfg
}

// resolveHostsPath determines the hosts file to use.
// Priority: CLI arg > FFH_HOSTS_FILE env var > config file > /etc/hosts
func resolveHostsPath(cliArg string) string {
	if cliArg != "" {
		return cliArg
	}
	if v := os.Getenv("FFH_HOSTS_FILE"); v != "" {
		return v
	}
	if v := loadConfig()["hosts_file"]; v != "" {
		return v
	}
	return defaultHostsPath
}

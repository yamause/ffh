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

// resolveSSHConfigPath determines the SSH config file to use.
// Priority: CLI -F arg > FFH_SSH_CONFIG env var > ssh_config in config file > ~/.ssh/config
func resolveSSHConfigPath(cliArg string) string {
	if cliArg != "" {
		return cliArg
	}
	if v := os.Getenv("FFH_SSH_CONFIG"); v != "" {
		return v
	}
	if v := loadConfig()["ssh_config"]; v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

// extractSSHConfigFlagValue returns the value of -F in args, or "" if not present.
func extractSSHConfigFlagValue(args []string) string {
	for i, arg := range args {
		if arg == "-F" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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

// resolveTabSource determines how tabs are grouped ("tag" or "source").
// Priority: --tab-source CLI arg > FFH_TAB_SOURCE env var > tab_source in config file > "tag"
func resolveTabSource(cliArg string) string {
	if cliArg != "" {
		return normalizeTabSource(cliArg)
	}
	if v := os.Getenv("FFH_TAB_SOURCE"); v != "" {
		return normalizeTabSource(v)
	}
	if v := loadConfig()["tab_source"]; v != "" {
		return normalizeTabSource(v)
	}
	return "source"
}

func normalizeTabSource(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "source" {
		return "source"
	}
	return "tag"
}

// extractTabSourceFlagValue returns the value of --tab-source in args, or "".
func extractTabSourceFlagValue(args []string) string {
	for i, arg := range args {
		if arg == "--tab-source" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// stripTabSourceFlag removes --tab-source and its value from args before passing to ssh.
func stripTabSourceFlag(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--tab-source" {
			i++ // skip value
			continue
		}
		out = append(out, args[i])
	}
	return out
}

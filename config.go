package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// splitAtDoubleDash splits args at the first "--" separator.
// Everything before "--" is returned as ffhArgs; everything after as sshArgs.
// If "--" is absent, sshArgs is nil.
func splitAtDoubleDash(args []string) (ffhArgs, sshArgs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
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

// unknownFFHFlag returns the first unrecognised flag in ffhArgs, or "".
// Known ffh flags are -F <value> and --tab-source <value>.
func unknownFFHFlag(ffhArgs []string) string {
	i := 0
	for i < len(ffhArgs) {
		arg := ffhArgs[i]
		if arg == "-F" || arg == "--tab-source" {
			i += 2
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return arg
		}
		i++
	}
	return ""
}

var (
	configMu     sync.Mutex
	cachedConfig map[string]string
)

// loadConfig parses ~/.config/ffh/config and returns key-value pairs.
// Format: key = value (lines starting with # are comments).
// Result is cached for the lifetime of the process.
func loadConfig() map[string]string {
	configMu.Lock()
	defer configMu.Unlock()
	if cachedConfig != nil {
		return cachedConfig
	}
	cfg := make(map[string]string)
	f, err := os.Open(configFilePath())
	if err == nil {
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
	}
	cachedConfig = cfg
	return cachedConfig
}

// resetConfigCache clears the loadConfig cache. Only used in tests.
func resetConfigCache() {
	configMu.Lock()
	cachedConfig = nil
	configMu.Unlock()
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



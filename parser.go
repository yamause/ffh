package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Host struct {
	Name         string
	HostName     string
	User         string
	Port         string
	ProxyJump    string
	IdentityFile string
	Tag          string
	Description  string
	SourceFile   string
}

func ParseSSHConfig(configPath string) ([]Host, error) {
	files, err := collectFiles(configPath)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var result []Host
	for _, f := range files {
		hosts, err := parseFile(f)
		if err != nil {
			return nil, err
		}
		for _, h := range hosts {
			if !seen[h.Name] {
				seen[h.Name] = true
				result = append(result, h)
			}
		}
	}
	return result, nil
}

func collectFiles(configPath string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	configDir := filepath.Dir(configPath)
	var included []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "include ") {
			// Stop scanning for Includes once we hit a Host or Match block
			if strings.HasPrefix(lower, "host ") || strings.HasPrefix(lower, "match ") {
				break
			}
			continue
		}
		pattern := strings.TrimSpace(line[len("include "):])
		pattern = expandHome(pattern, home)
		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(configDir, pattern)
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		included = append(included, matches...)
	}

	// Include files first, then the main config (mirrors OpenSSH behavior)
	return append(included, configPath), nil
}

func parseFile(path string) ([]Host, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	home, _ := os.UserHomeDir()

	var hosts []Host
	var current *Host
	var extraNames []string
	var pendingComments []string
	inMatch := false

	finalize := func() {
		if current != nil {
			hosts = append(hosts, *current)
			for _, name := range extraNames {
				clone := *current
				clone.Name = name
				hosts = append(hosts, clone)
			}
			current = nil
			extraNames = nil
		}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			finalize()
			pendingComments = nil
			inMatch = false
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			if current == nil {
				pendingComments = append(pendingComments, trimmed)
			}
			// Comments inside a Host block are ignored
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		keyword := strings.ToLower(fields[0])

		if keyword == "match" {
			finalize()
			pendingComments = nil
			inMatch = true
			continue
		}

		if keyword == "host" {
			finalize()
			inMatch = false
			var validNames []string
			for _, name := range fields[1:] {
				if !strings.ContainsAny(name, "*?") {
					validNames = append(validNames, name)
				}
			}
			if len(validNames) == 0 {
				pendingComments = nil
				continue
			}
			desc := extractDescription(pendingComments)
			pendingComments = nil
			current = &Host{Name: validNames[0], SourceFile: path, Description: desc}
			extraNames = validNames[1:]
			continue
		}

		if inMatch || current == nil {
			continue
		}

		value := strings.Join(fields[1:], " ")
		switch keyword {
		case "hostname":
			current.HostName = value
		case "user":
			current.User = value
		case "port":
			current.Port = value
		case "proxyjump":
			current.ProxyJump = value
		case "identityfile":
			if current.IdentityFile == "" {
				current.IdentityFile = unexpandHome(value, home)
			}
		case "tag":
			current.Tag = value
		}
	}
	finalize()

	return hosts, scanner.Err()
}

// extractDescription finds "# Description:" as a start marker in comments,
// then collects that line and all immediately following comment lines as body text.
//
// Example:
//
//	# Description:
//	# line one
//	# line two
//	→ "line one\nline two"
//
// Also handles the inline form:
//
//	# Description: single line text
//	→ "single line text"
func extractDescription(comments []string) string {
	markerIdx := -1
	for i, c := range comments {
		s := strings.TrimSpace(strings.TrimPrefix(c, "#"))
		if strings.HasPrefix(strings.ToLower(s), "description:") {
			markerIdx = i
			break
		}
	}
	if markerIdx < 0 {
		return ""
	}

	// Text on the same line as the marker (e.g. "# Description: some text")
	markerLine := strings.TrimSpace(strings.TrimPrefix(comments[markerIdx], "#"))
	inlineText := strings.TrimSpace(markerLine[len("description:"):])

	var lines []string
	if inlineText != "" {
		lines = append(lines, inlineText)
	}

	// Subsequent comment lines are body text
	for _, c := range comments[markerIdx+1:] {
		body := strings.TrimSpace(strings.TrimPrefix(c, "#"))
		lines = append(lines, body)
	}

	return strings.Join(lines, "\n")
}

// findHost returns a pointer to the first Host in hosts whose Name matches name, or nil.
func findHost(hosts []Host, name string) *Host {
	for i := range hosts {
		if hosts[i].Name == name {
			return &hosts[i]
		}
	}
	return nil
}

func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}
	return path
}

func unexpandHome(path, home string) string {
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

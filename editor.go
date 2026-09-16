package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// sshConfigView launches a nested fzf showing the full resolved ssh -G output for hostname.
// Enter opens an inline fzf-based edit dialog for the selected option; after saving the list reloads.
func sshConfigView(hostname string, sshConfigPath string) error {
	out, err := exec.Command("ssh", "-F", sshConfigPath, "-G", hostname).Output()
	if err != nil {
		return fmt.Errorf("ssh -G %s: %w", hostname, err)
	}
	lines := strings.TrimSpace(string(out))
	if lines == "" {
		return fmt.Errorf("ssh -G returned empty output for %s", hostname)
	}

	exPath := selfPath()
	// Enter: open fzf edit dialog for selected option, then reload ssh -G output.
	bindEnter := fmt.Sprintf(
		"enter:execute(%s --edit-host-option %s %s {})+reload(ssh -F %s -G %s 2>/dev/null || echo '')",
		exPath, hostname, sshConfigPath,
		sshConfigPath, hostname,
	)
	_ = runFzf(lines, []string{
		"--layout=reverse",
		"--border=rounded",
		"--prompt=ssh -G " + hostname + "> ",
		"--preview=" + exPath + " --preview-option {}",
		"--preview-window=right:50%:wrap",
		"--preview-label=" + msgs.labelOptionDesc,
		"--ansi",
		"--header=" + msgs.configViewHeader(hostname),
		"--header-first",
		"--no-sort",
		"--bind=" + bindEnter,
	})
	return nil
}

// printOptionPreview prints a formatted description for an ssh -G output line.
func printOptionPreview(line string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return
	}
	keyword := strings.ToLower(fields[0])
	value := ""
	if len(fields) > 1 {
		value = strings.Join(fields[1:], " ")
	}

	fmt.Printf("%s  %s%s\n", ansiBold, keyword, ansiReset)
	fmt.Println("  " + strings.Repeat("─", 36))
	if value != "" {
		fmt.Printf("  %sValue:%s  %s%s%s\n\n", ansiBold, ansiReset, ansiCyan, value, ansiReset)
	}
	desc, known := msgs.optionDescriptions[keyword]
	if known {
		fmt.Printf("  %s%s%s\n", ansiBold, msgs.labelDesc, ansiReset)
		for _, dline := range wrapText(desc, 60) {
			fmt.Printf("  %s%s%s\n", ansiYellow, dline, ansiReset)
		}
	} else {
		fmt.Printf("  %s%s%s\n", ansiDim, msgs.noDescription, ansiReset)
	}
}

// wrapText wraps s at maxCols rune-width columns, breaking on spaces.
func wrapText(s string, maxCols int) []string {
	if len([]rune(s)) <= maxCols {
		return []string{s}
	}
	var lines []string
	words := strings.Fields(s)
	var cur []string
	curLen := 0
	for _, w := range words {
		wLen := len([]rune(w))
		if curLen > 0 && curLen+1+wLen > maxCols {
			lines = append(lines, strings.Join(cur, " "))
			cur = cur[:0]
			curLen = 0
		}
		cur = append(cur, w)
		curLen += wLen + 1
	}
	if len(cur) > 0 {
		lines = append(lines, strings.Join(cur, " "))
	}
	return lines
}

// editHostOption opens a fzf-based modal input dialog to edit one SSH config directive.
// optionLine is a "keyword value..." string (from ssh -G output / fzf selection).
// The user edits the value inside a small fzf window; on confirm the file is updated
// and validated with ssh -G, rolling back on syntax error.
func editHostOption(hostname, sshConfigPath, optionLine string) error {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		return fmt.Errorf("parse SSH config: %w", err)
	}
	found := findHost(hosts, hostname)
	if found == nil || found.SourceFile == "" {
		return fmt.Errorf("%s %s", msgs.errEditNoSource, hostname)
	}

	fields := strings.Fields(optionLine)
	if len(fields) == 0 {
		return fmt.Errorf("empty option line")
	}
	keyword := fields[0]
	currentValue := ""
	if len(fields) > 1 {
		currentValue = strings.Join(fields[1:], " ")
	}

	home, _ := os.UserHomeDir()
	src := unexpandHome(found.SourceFile, home)
	header := msgs.editModalHeader(hostname, keyword, src, currentValue)

	// Use fzf as a text-input modal: --disabled keeps the dummy item always selected
	// so Enter exits with code 0, and --print-query returns what the user typed.
	newValue, confirmed := runFzfQuery(currentValue, []string{
		"--layout=reverse",
		"--border=rounded",
		"--border-label=" + msgs.editModalLabel,
		"--prompt=" + keyword + ": ",
		"--header=" + header,
		"--header-first",
		"--disabled",
		"--print-query",
		"--no-info",
		"--bind=esc:abort",
	})

	if !confirmed || newValue == "" || newValue == currentValue {
		return nil
	}
	return applyHostDirective(found.SourceFile, hostname, sshConfigPath, keyword, newValue)
}

// applyHostDirective writes keyword=newValue into the SSH config file for hostname,
// validates with ssh -G, and rolls back on syntax error.
func applyHostDirective(sourceFile, hostname, sshConfigPath, keyword, newValue string) error {
	original, err := updateHostDirective(sourceFile, hostname, keyword, newValue)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}

	if out, chkErr := exec.Command("ssh", "-F", sshConfigPath, "-G", hostname).CombinedOutput(); chkErr != nil {
		_ = os.WriteFile(sourceFile, original, 0600)
		return fmt.Errorf("%s\n%s", msgs.editRollback, strings.TrimSpace(string(out)))
	}
	return nil
}

// hostBlockMatches reports whether hostname is one of the space-separated name
// tokens following a "Host" keyword (fields[0]), ignoring wildcard tokens (parser.go's
// parseFile never resolves a Host.Name from one, so hostname itself never contains
// "*"/"?" -- treating a wildcard token as a match here would risk rewriting a
// catch-all block like "Host *" for every host). A Host line can list several plain
// names sharing one block, each expanded into its own Host entry by parseFile, so the
// block's resolved Host.Name may be any one of those tokens, not just the first.
func hostBlockMatches(fields []string, hostname string) bool {
	for _, name := range fields[1:] {
		if strings.ContainsAny(name, "*?") {
			continue
		}
		if strings.EqualFold(name, hostname) {
			return true
		}
	}
	return false
}

// updateHostDirective rewrites filePath in-place, updating or inserting keyword newValue
// inside the first Host block whose name matches hostname (case-insensitive).
// Returns the original file contents for rollback purposes.
func updateHostDirective(filePath, hostname, keyword, newValue string) ([]byte, error) {
	original, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(original), "\n")
	// Preserve whether the file ended with a newline.
	trailingNewline := len(original) > 0 && original[len(original)-1] == '\n'

	var result []string
	inBlock := false
	updated := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if inBlock {
			// End of block: blank line, next Host, or Match
			if trimmed == "" || strings.HasPrefix(lower, "host ") || strings.HasPrefix(lower, "match ") {
				if !updated {
					result = append(result, "  "+keyword+" "+newValue)
					updated = true
				}
				inBlock = false
				if strings.HasPrefix(lower, "host ") {
					blockFields := strings.Fields(trimmed)
					if len(blockFields) >= 2 && hostBlockMatches(blockFields, hostname) {
						inBlock = true
						updated = false
					}
				}
				result = append(result, line)
				continue
			}
			// Check if this line is the directive we want to update.
			lineFields := strings.Fields(trimmed)
			if len(lineFields) >= 1 && strings.EqualFold(lineFields[0], keyword) && !updated {
				// Preserve leading whitespace.
				leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				result = append(result, leading+keyword+" "+newValue)
				updated = true
				continue
			}
			result = append(result, line)
			continue
		}

		// Outside any block: look for matching Host line.
		if strings.HasPrefix(lower, "host ") {
			blockFields := strings.Fields(trimmed)
			if len(blockFields) >= 2 && hostBlockMatches(blockFields, hostname) {
				inBlock = true
				updated = false
			}
		}
		result = append(result, line)
	}

	// EOF while still inside the target block.
	if inBlock && !updated {
		result = append(result, "  "+keyword+" "+newValue)
	}

	joined := strings.Join(result, "\n")
	if trailingNewline && !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}

	if err := os.WriteFile(filePath, []byte(joined), 0600); err != nil {
		return original, err
	}
	return original, nil
}

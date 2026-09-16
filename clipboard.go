package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// copySSHCommand builds an ssh command string for host and writes it to the clipboard.
func copySSHCommand(name string, sshConfigPath string) {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, msgs.errParseSSHConfig, err)
		return
	}
	found := findHost(hosts, name)

	var parts []string
	parts = append(parts, "ssh")
	if found != nil {
		if found.User != "" {
			parts = append(parts, "-l", found.User)
		}
		if found.Port != "" {
			parts = append(parts, "-p", found.Port)
		}
		if found.ProxyJump != "" {
			parts = append(parts, "-J", found.ProxyJump)
		}
	}
	parts = append(parts, name)
	cmd := strings.Join(parts, " ")

	if err := writeClipboard(cmd); err != nil {
		fmt.Fprintln(os.Stderr, msgs.errClipboard, err)
		return
	}
	fmt.Fprintln(os.Stderr, msgs.msgCopied, cmd)
}

// writeClipboard writes text to the system clipboard using the first available tool.
func writeClipboard(text string) error {
	tools := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
	}
	for _, t := range tools {
		if _, err := exec.LookPath(t[0]); err != nil {
			continue
		}
		cmd := exec.Command(t[0], t[1:]...)
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return fmt.Errorf("no clipboard tool found (install wl-copy, xclip, xsel, or pbcopy)")
}

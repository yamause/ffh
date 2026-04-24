package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	args := os.Args[1:]

	if len(args) >= 2 && args[0] == "--preview-host" {
		if err := printPreview(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Internal command used by fzf reload bindings: --tab-list <statefile> <delta>
	// Outputs header as line 1, then filtered host list. Used with --header-lines=1.
	if len(args) >= 3 && args[0] == "--tab-list" {
		tabList(args[1], mustAtoi(args[2]))
		return
	}

	if len(args) >= 1 && args[0] == "--hosts" {
		rest := args[1:]
		var cliPath string
		if len(rest) >= 1 && !strings.HasPrefix(rest[0], "-") {
			cliPath = rest[0]
			rest = rest[1:]
		}
		hostsMode(resolveHostsPath(cliPath), rest)
		return
	}

	sshMode(args)
}

// tabState holds the ordered tag list and current index, persisted in a temp file.
// Format: "<idx>\n<tag0>\n<tag1>\n..." where tag0 is always "All".
type tabState struct {
	tags []string // tags[0] == "All"
	idx  int
}

func loadTabState(path string) tabState {
	data, err := os.ReadFile(path)
	if err != nil {
		return tabState{}
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 2 {
		return tabState{}
	}
	idx, _ := strconv.Atoi(lines[0])
	return tabState{tags: lines[1:], idx: idx}
}

func (s tabState) save(path string) {
	lines := []string{strconv.Itoa(s.idx)}
	lines = append(lines, s.tags...)
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

func (s tabState) currentTag() string {
	if s.idx == 0 || s.idx >= len(s.tags) {
		return ""
	}
	return s.tags[s.idx]
}

func buildTabState(hosts []Host) tabState {
	seen := make(map[string]bool)
	var tags []string
	for _, h := range hosts {
		if h.Tag != "" && !seen[h.Tag] {
			seen[h.Tag] = true
			tags = append(tags, h.Tag)
		}
	}
	sort.Strings(tags)
	return tabState{tags: append([]string{"All"}, tags...), idx: 0}
}

func renderHeader(s tabState) string {
	var parts []string
	for i, t := range s.tags {
		if i == s.idx {
			parts = append(parts, "\033[1;7m "+t+" \033[0m") // bold + reverse = selected
		} else {
			parts = append(parts, "\033[2m "+t+" \033[0m") // dim = inactive
		}
	}
	return "  " + strings.Join(parts, " ")
}

func filterHosts(hosts []Host, tag string) []string {
	var names []string
	for _, h := range hosts {
		if tag == "" || h.Tag == tag {
			names = append(names, h.Name)
		}
	}
	return names
}

// tabList is called by fzf reload bindings. It advances the tab index by delta,
// then outputs: line 1 = header (consumed by --header-lines=1), remaining lines = host names.
func tabList(statefile string, delta int) {
	s := loadTabState(statefile)
	if len(s.tags) == 0 {
		return
	}
	s.idx = (s.idx + delta + len(s.tags)) % len(s.tags)
	s.save(statefile)

	hosts := loadHosts()
	names := filterHosts(hosts, s.currentTag())
	// Header on line 1, hosts on subsequent lines.
	fmt.Println(renderHeader(s))
	fmt.Print(strings.Join(names, "\n"))
}

func loadHosts() []Host {
	configPath := filepath.Join(must(os.UserHomeDir()), ".ssh", "config")
	hosts, err := ParseSSHConfig(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing SSH config:", err)
		os.Exit(1)
	}
	return hosts
}

func sshMode(args []string) {
	hosts := loadHosts()

	// Build and persist initial tab state
	statefile := tempStateFile()
	s := buildTabState(hosts)
	s.save(statefile)
	defer os.Remove(statefile)

	names := filterHosts(hosts, "") // All
	exPath := selfPath()

	// Initial input: header on line 1 (consumed by --header-lines=1), hosts follow.
	initialInput := renderHeader(s) + "\n" + strings.Join(names, "\n")

	// Tab = next tag, Shift-Tab = prev tag.
	// --tab-list outputs header as line 1 + host list, so reload replaces both atomically.
	bindNext := fmt.Sprintf("tab:reload(%s --tab-list %s 1)", exPath, statefile)
	bindPrev := fmt.Sprintf("shift-tab:reload(%s --tab-list %s -1)", exPath, statefile)

	selected := runFzf(
		initialInput,
		[]string{
			"--layout=reverse",
			"--border=rounded",
			"--prompt=ssh> ",
			"--preview=" + exPath + " --preview-host {}",
			"--preview-window=left:40%:wrap",
			"--preview-label= Host Details ",
			"--ansi",
			"--header-lines=1",
			"--header-first",
			"--bind=" + bindNext,
			"--bind=" + bindPrev,
		},
	)
	if selected == "" {
		return
	}

	fmt.Fprintln(os.Stderr, "Connect to", selected)
	execSSH(selected, args)
}

func hostsMode(path string, args []string) {
	entries, err := parseHostsFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading hosts file:", err)
		os.Exit(1)
	}

	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.IP + "\t" + e.Hostname
	}

	selected := runFzf(
		strings.Join(lines, "\n"),
		[]string{
			"--layout=reverse",
			"--border=rounded",
			"--prompt=hosts> ",
			"--nth=2",
			"--with-nth=1,2",
		},
	)
	if selected == "" {
		return
	}

	fields := strings.Fields(selected)
	hostname := fields[len(fields)-1]
	fmt.Fprintln(os.Stderr, "Connect to", hostname)
	execSSH(hostname, args)
}

func printPreview(name string) error {
	configPath := filepath.Join(must(os.UserHomeDir()), ".ssh", "config")
	hosts, err := ParseSSHConfig(configPath)
	if err != nil {
		return fmt.Errorf("parse SSH config: %w", err)
	}

	var found *Host
	for i := range hosts {
		if hosts[i].Name == name {
			found = &hosts[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("host not found: %s", name)
	}

	bold := "\033[1m"
	reset := "\033[0m"
	cyan := "\033[36m"
	yellow := "\033[33m"

	label := func(k, v string) {
		if v == "" {
			return
		}
		fmt.Printf("  %s%-14s%s %s%s%s\n", bold, k+":", reset, cyan, v, reset)
	}

	fmt.Printf("%s  Host:%s         %s%s%s\n", bold, reset, cyan, found.Name, reset)
	fmt.Println("  " + strings.Repeat("─", 32))

	port := found.Port
	if port == "" {
		port = "22 (default)"
	}
	label("HostName", found.HostName)
	label("User", found.User)
	label("Port", port)
	label("ProxyJump", found.ProxyJump)
	label("IdentityFile", found.IdentityFile)
	label("Tag", found.Tag)

	src := unexpandHome(found.SourceFile, must(os.UserHomeDir()))
	label("Source", src)

	if found.Description != "" {
		fmt.Println()
		fmt.Println("  " + strings.Repeat("─", 32))
		fmt.Printf("  %s Description %s\n", bold, reset)
		for _, line := range strings.Split(found.Description, "\n") {
			fmt.Printf("  %s%s%s\n", yellow, line, reset)
		}
	}

	return nil
}

func runFzf(input string, fzfArgs []string) string {
	cmd := exec.Command("fzf", fzfArgs...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func execSSH(host string, args []string) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ssh not found in PATH")
		os.Exit(1)
	}
	sshArgs := append([]string{"ssh", host}, args...)
	if err := syscall.Exec(sshPath, sshArgs, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "exec ssh:", err)
		os.Exit(1)
	}
}

func selfPath() string {
	ex, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	real, err := filepath.EvalSymlinks(ex)
	if err != nil {
		return ex
	}
	return real
}

func tempStateFile() string {
	f, err := os.CreateTemp("", "ffh-tab-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create temp file:", err)
		os.Exit(1)
	}
	f.Close()
	return f.Name()
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// version is set at build time via -ldflags "-X main.version=x.y.z".
var version = "dev"

func main() {
	initMessages()
	args := os.Args[1:]

	if len(args) >= 1 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Println("ffh version " + version)
		return
	}

	if len(args) >= 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(msgs.helpText(version))
		return
	}

	if len(args) >= 2 && args[0] == "--preview-host" {
		var configArg string
		if len(args) >= 3 {
			configArg = args[2]
		}
		if err := printPreview(args[1], resolveSSHConfigPath(configArg)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Internal command used by fzf reload bindings: --tab-list <statefile> <delta> [<sshconfig>]
	// Outputs header as line 1, then filtered host list. Used with --header-lines=1.
	if len(args) >= 3 && args[0] == "--tab-list" {
		var configArg string
		if len(args) >= 4 {
			configArg = args[3]
		}
		tabList(args[1], mustAtoi(args[2]), resolveSSHConfigPath(configArg))
		return
	}

	// Internal command used by ctrl-g execute binding: --ssh-config-view <hostname> [<sshconfig>]
	// Launches a nested fzf showing ssh -G output with per-option descriptions.
	if len(args) >= 2 && args[0] == "--ssh-config-view" {
		var configArg string
		if len(args) >= 3 {
			configArg = args[2]
		}
		if err := sshConfigView(args[1], resolveSSHConfigPath(configArg)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Internal command used by nested fzf preview: --preview-option <option-line>
	// args[1:] is rejoined to reconstruct the full "keyword value..." line from ssh -G.
	if len(args) >= 2 && args[0] == "--preview-option" {
		printOptionPreview(strings.Join(args[1:], " "))
		return
	}

	// Internal command: --check-host <hostname> <sshconfig>
	// Prints UP/DOWN status to stdout for fzf preview.
	if len(args) >= 2 && args[0] == "--check-host" {
		var configArg string
		if len(args) >= 3 {
			configArg = args[2]
		}
		printHostCheck(args[1], resolveSSHConfigPath(configArg))
		return
	}

	// Internal command: --copy-ssh-cmd <hostname> <sshconfig>
	// Builds the ssh command string and copies it to clipboard.
	if len(args) >= 2 && args[0] == "--copy-ssh-cmd" {
		var configArg string
		if len(args) >= 3 {
			configArg = args[2]
		}
		copySSHCommand(args[1], resolveSSHConfigPath(configArg))
		return
	}

	// ffh --history [--delete <host> | --list]
	if len(args) >= 1 && args[0] == "--history" {
		rest := args[1:]
		if len(rest) >= 2 && rest[0] == "--delete" {
			host := rest[1]
			if deleteHistoryEntry(host) {
				fmt.Println(msgs.msgHistoryDeleted, host)
			} else {
				fmt.Fprintln(os.Stderr, msgs.errHistoryNotFound, host)
				os.Exit(1)
			}
			return
		}
		// Internal: --history --list outputs the history lines for fzf reload.
		if len(rest) >= 1 && rest[0] == "--list" {
			printHistoryLines()
			return
		}
		sshConfigPath := resolveSSHConfigPath(extractSSHConfigFlagValue(rest))
		historyMode(rest, sshConfigPath)
		return
	}

	// ffh --check [<sshconfig>]
	if len(args) >= 1 && args[0] == "--check" {
		var configArg string
		if len(args) >= 2 {
			configArg = args[1]
		}
		checkDuplicates(resolveSSHConfigPath(configArg))
		return
	}

	// ffh --exec <tag> <command...>
	if len(args) >= 3 && args[0] == "--exec" {
		sshConfigPath := resolveSSHConfigPath(extractSSHConfigFlagValue(args))
		execTag(args[1], args[2:], sshConfigPath)
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

	// Extract -F value before passing args to sshMode so the config path is known.
	sshConfigPath := resolveSSHConfigPath(extractSSHConfigFlagValue(args))
	sshMode(args, sshConfigPath)
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
	return tabState{tags: append([]string{msgs.tabAll}, tags...), idx: 0}
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
func tabList(statefile string, delta int, sshConfigPath string) {
	s := loadTabState(statefile)
	if len(s.tags) == 0 {
		return
	}
	s.idx = (s.idx + delta + len(s.tags)) % len(s.tags)
	s.save(statefile)

	hosts := loadHosts(sshConfigPath)
	names := filterHosts(hosts, s.currentTag())
	// Header on line 1, hosts on subsequent lines.
	fmt.Println(renderHeader(s))
	fmt.Print(strings.Join(names, "\n"))
}

func loadHosts(sshConfigPath string) []Host {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, msgs.errParseSSHConfig, err)
		os.Exit(1)
	}
	return hosts
}

func sshMode(args []string, sshConfigPath string) {
	hosts := loadHosts(sshConfigPath)

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
	bindNext := fmt.Sprintf("tab:reload(%s --tab-list %s 1 %s)", exPath, statefile, sshConfigPath)
	bindPrev := fmt.Sprintf("shift-tab:reload(%s --tab-list %s -1 %s)", exPath, statefile, sshConfigPath)
	// Ctrl-G opens a nested fzf showing full ssh -G output with per-option descriptions.
	bindConfigView := fmt.Sprintf("ctrl-g:execute(%s --ssh-config-view {} %s)", exPath, sshConfigPath)
	// Ctrl-Y copies the ssh command to clipboard.
	bindCopy := fmt.Sprintf("ctrl-y:execute(%s --copy-ssh-cmd {} %s)", exPath, sshConfigPath)
	// Ctrl-P refreshes the preview pane to show host connectivity check.
	bindCheck := fmt.Sprintf("ctrl-p:preview(%s --check-host {} %s)", exPath, sshConfigPath)

	selected := runFzf(
		initialInput,
		[]string{
			"--layout=reverse",
			"--border=rounded",
			"--prompt=" + msgs.promptSSH,
			"--preview=" + exPath + " --preview-host {} " + sshConfigPath,
			"--preview-window=left:40%:wrap",
			"--preview-label=" + msgs.labelHostDetails,
			"--ansi",
			"--header-lines=1",
			"--header-first",
			"--bind=" + bindNext,
			"--bind=" + bindPrev,
			"--bind=" + bindConfigView,
			"--bind=" + bindCopy,
			"--bind=" + bindCheck,
		},
	)
	if selected == "" {
		return
	}

	fmt.Fprintln(os.Stderr, msgs.msgConnectTo, selected)
	recordHistory(selected)
	// Inject -F into ssh args if the config path came from env/config (not already in args).
	sshArgs := args
	if extractSSHConfigFlagValue(args) == "" {
		sshArgs = append([]string{"-F", sshConfigPath}, args...)
	}
	execSSH(selected, sshArgs)
}

func hostsMode(path string, args []string) {
	entries, err := parseHostsFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, msgs.errReadHostsFile, err)
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
			"--prompt=" + msgs.promptHosts,
			"--nth=2",
			"--with-nth=1,2",
		},
	)
	if selected == "" {
		return
	}

	fields := strings.Fields(selected)
	hostname := fields[len(fields)-1]
	fmt.Fprintln(os.Stderr, msgs.msgConnectTo, hostname)
	execSSH(hostname, args)
}

func printPreview(name string, sshConfigPath string) error {
	hosts, err := ParseSSHConfig(sshConfigPath)
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
	green := "\033[32m"
	dim := "\033[2m"

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
		port = msgs.portDefault
	}
	label("HostName", found.HostName)
	label("User", found.User)
	label("Port", port)
	label("ProxyJump", found.ProxyJump)
	label("IdentityFile", found.IdentityFile)
	label("Tag", found.Tag)

	src := unexpandHome(found.SourceFile, must(os.UserHomeDir()))
	label("Source", src)

	// Show connection history if available
	if e := findHistoryEntry(name); e != nil {
		ago := formatAgo(e.LastUsed)
		histLine := fmt.Sprintf("%s (%s x%d)", ago, msgs.labelHistoryConnected, e.ConnCount)
		fmt.Printf("  %s%-14s%s %s%s%s\n", bold, msgs.labelLastUsed+":", reset, green, histLine, reset)
	}

	if found.Description != "" {
		fmt.Println()
		fmt.Println("  " + strings.Repeat("─", 32))
		fmt.Printf("  %s%s%s\n", bold, msgs.labelDescriptionSection, reset)
		for _, line := range strings.Split(found.Description, "\n") {
			fmt.Printf("  %s%s%s\n", yellow, line, reset)
		}
	}

	_ = dim
	return nil
}

// printHostCheck performs a TCP dial to the host's SSH port and prints UP/DOWN status.
func printHostCheck(name string, sshConfigPath string) {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		fmt.Println(msgs.errParseSSHConfig, err)
		return
	}
	var found *Host
	for i := range hosts {
		if hosts[i].Name == name {
			found = &hosts[i]
			break
		}
	}
	if found == nil {
		fmt.Println(msgs.errHostNotFound, name)
		return
	}

	target := found.HostName
	if target == "" {
		target = name
	}
	port := found.Port
	if port == "" {
		port = "22"
	}
	addr := net.JoinHostPort(target, port)

	green := "\033[32m"
	red := "\033[31m"
	bold := "\033[1m"
	reset := "\033[0m"

	start := time.Now()
	conn, dialErr := net.DialTimeout("tcp", addr, 3*time.Second)
	elapsed := time.Since(start)

	if dialErr == nil {
		conn.Close()
		fmt.Printf("%s%s● %s%s  %s (%dms)%s\n",
			bold, green, reset, green,
			msgs.statusUp, elapsed.Milliseconds(), reset)
	} else {
		fmt.Printf("%s%s○ %s%s  %s%s\n",
			bold, red, reset, red,
			msgs.statusDown, reset)
	}
	fmt.Printf("  %s → %s\n", name, addr)
}

// copySSHCommand builds an ssh command string for host and writes it to the clipboard.
func copySSHCommand(name string, sshConfigPath string) {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, msgs.errParseSSHConfig, err)
		return
	}
	var found *Host
	for i := range hosts {
		if hosts[i].Name == name {
			found = &hosts[i]
			break
		}
	}

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


// historyMode opens an fzf selector over connection history and connects to the selected host.
func historyMode(args []string, sshConfigPath string) {
	entries := loadHistory()
	if len(entries) == 0 {
		fmt.Println(msgs.msgHistoryEmpty)
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})

	exPath := selfPath()
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = fmt.Sprintf("%-30s  %s  x%d",
			e.Host,
			e.LastUsed.Format("2006-01-02 15:04"),
			e.ConnCount,
		)
	}

	bindDelete := fmt.Sprintf(
		"ctrl-d:execute(%s --history --delete {1})+reload(%s --history --list)",
		exPath, exPath,
	)
	bindConfigView := fmt.Sprintf("ctrl-g:execute(%s --ssh-config-view {1} %s)", exPath, sshConfigPath)
	bindCopy := fmt.Sprintf("ctrl-y:execute(%s --copy-ssh-cmd {1} %s)", exPath, sshConfigPath)
	bindCheck := fmt.Sprintf("ctrl-p:preview(%s --check-host {1} %s)", exPath, sshConfigPath)

	selected := runFzf(
		strings.Join(lines, "\n"),
		[]string{
			"--layout=reverse",
			"--border=rounded",
			"--prompt=" + msgs.promptHistory,
			"--nth=1",
			"--with-nth=1,2,3",
			"--preview=" + exPath + " --preview-host {1} " + sshConfigPath,
			"--preview-window=left:40%:wrap",
			"--preview-label=" + msgs.labelHostDetails,
			"--ansi",
			"--header=" + msgs.historyHeader,
			"--bind=" + bindDelete,
			"--bind=" + bindConfigView,
			"--bind=" + bindCopy,
			"--bind=" + bindCheck,
		},
	)
	if selected == "" {
		return
	}
	host := strings.Fields(selected)[0]
	fmt.Fprintln(os.Stderr, msgs.msgConnectTo, host)
	recordHistory(host)
	sshArgs := args
	if extractSSHConfigFlagValue(args) == "" {
		sshArgs = append([]string{"-F", sshConfigPath}, args...)
	}
	execSSH(host, sshArgs)
}

// printHistoryLines outputs history in the same format used by historyMode for fzf reload.
func printHistoryLines() {
	entries := loadHistory()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = fmt.Sprintf("%-30s  %s  x%d",
			e.Host,
			e.LastUsed.Format("2006-01-02 15:04"),
			e.ConnCount,
		)
	}
	fmt.Print(strings.Join(lines, "\n"))
}

// printHistory prints connection history sorted by most recently used.
func printHistory() {
	entries := loadHistory()
	if len(entries) == 0 {
		fmt.Println(msgs.msgHistoryEmpty)
		return
	}
	// Sort by most recently used
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})
	bold := "\033[1m"
	cyan := "\033[36m"
	reset := "\033[0m"
	fmt.Printf("%s%-30s %-20s %s%s\n", bold, msgs.colHost, msgs.colLastUsed, msgs.colCount, reset)
	fmt.Println(strings.Repeat("─", 60))
	for _, e := range entries {
		fmt.Printf("%s%-30s%s %-20s %d\n",
			cyan, e.Host, reset,
			e.LastUsed.Format("2006-01-02 15:04"),
			e.ConnCount,
		)
	}
}

// checkDuplicates reports hosts defined in multiple SSH config files.
func checkDuplicates(sshConfigPath string) {
	files, err := collectFiles(sshConfigPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, msgs.errParseSSHConfig, err)
		os.Exit(1)
	}

	// Map host name → list of source files
	type occurrence struct {
		file string
		host Host
	}
	seen := make(map[string][]occurrence)
	for _, f := range files {
		hosts, err := parseFile(f)
		if err != nil {
			continue
		}
		for _, h := range hosts {
			seen[h.Name] = append(seen[h.Name], occurrence{file: f, host: h})
		}
	}

	bold := "\033[1m"
	yellow := "\033[33m"
	green := "\033[32m"
	reset := "\033[0m"
	dim := "\033[2m"

	var dups []string
	for name, occ := range seen {
		if len(occ) > 1 {
			dups = append(dups, name)
		}
	}
	sort.Strings(dups)

	if len(dups) == 0 {
		fmt.Printf("%s%s%s\n", green, msgs.msgNoDuplicates, reset)
		return
	}

	fmt.Printf("%s%s%s\n\n", bold, msgs.msgDuplicatesFound, reset)
	for _, name := range dups {
		occ := seen[name]
		fmt.Printf("%s%s%s\n", bold, name, reset)
		for i, o := range occ {
			src := unexpandHome(o.file, must(os.UserHomeDir()))
			if i == 0 {
				fmt.Printf("  %s✓ %s%s  %s(%s)%s\n", green, src, reset, dim, msgs.labelEffective, reset)
			} else {
				fmt.Printf("  %s✗ %s%s  %s(%s)%s\n", yellow, src, reset, dim, msgs.labelIgnored, reset)
			}
		}
		fmt.Println()
	}
}

// execTag runs a command on all hosts with the given tag, sequentially.
func execTag(tag string, cmdArgs []string, sshConfigPath string) {
	hosts := loadHosts(sshConfigPath)
	var targets []Host
	for _, h := range hosts {
		if h.Tag == tag {
			targets = append(targets, h)
		}
	}
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, msgs.errNoHostsForTag, tag)
		os.Exit(1)
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		fmt.Fprintln(os.Stderr, msgs.errSSHNotFound)
		os.Exit(1)
	}

	colors := []string{"\033[32m", "\033[33m", "\033[34m", "\033[35m", "\033[36m"}
	reset := "\033[0m"
	bold := "\033[1m"

	var wg sync.WaitGroup
	for i, h := range targets {
		wg.Add(1)
		go func(idx int, host Host) {
			defer wg.Done()
			color := colors[idx%len(colors)]
			prefix := fmt.Sprintf("%s%s[%s]%s ", bold, color, host.Name, reset)

			sshArgs := []string{"-F", sshConfigPath, host.Name}
			sshArgs = append(sshArgs, cmdArgs...)
			cmd := exec.Command(sshPath, sshArgs...)

			out, err := cmd.CombinedOutput()
			for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
				fmt.Printf("%s%s\n", prefix, line)
			}
			if err != nil {
				fmt.Printf("%s%s%s\n", prefix, msgs.errExecSSH+" "+err.Error(), reset)
			}
		}(i, h)
	}
	wg.Wait()
}

// formatAgo returns a human-readable "N days ago" string.
func formatAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return msgs.agoJustNow
	case d < time.Hour:
		return fmt.Sprintf("%d%s", int(d.Minutes()), msgs.agoMinutes)
	case d < 24*time.Hour:
		return fmt.Sprintf("%d%s", int(d.Hours()), msgs.agoHours)
	default:
		return fmt.Sprintf("%d%s", int(d.Hours()/24), msgs.agoDays)
	}
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
		fmt.Fprintln(os.Stderr, msgs.errSSHNotFound)
		os.Exit(1)
	}
	sshArgs := append([]string{"ssh", host}, args...)
	if err := syscall.Exec(sshPath, sshArgs, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, msgs.errExecSSH, err)
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
		fmt.Fprintln(os.Stderr, msgs.errTempFile, err)
		os.Exit(1)
	}
	f.Close()
	return f.Name()
}

// sshConfigView launches a nested fzf showing the full resolved ssh -G output for hostname.
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

	bold := "\033[1m"
	reset := "\033[0m"
	cyan := "\033[36m"
	yellow := "\033[33m"
	dim := "\033[2m"

	fmt.Printf("%s  %s%s\n", bold, keyword, reset)
	fmt.Println("  " + strings.Repeat("─", 36))
	if value != "" {
		fmt.Printf("  %sValue:%s  %s%s%s\n\n", bold, reset, cyan, value, reset)
	}
	desc, known := msgs.optionDescriptions[keyword]
	if known {
		fmt.Printf("  %s%s%s\n", bold, msgs.labelDesc, reset)
		for _, dline := range wrapText(desc, 60) {
			fmt.Printf("  %s%s%s\n", yellow, dline, reset)
		}
	} else {
		fmt.Printf("  %s%s%s\n", dim, msgs.noDescription, reset)
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

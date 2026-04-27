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
	"unsafe"
)

// version is set at build time via -ldflags "-X main.version=x.y.z".
var version = "dev"

const (
	ansiBold   = "\033[1m"
	ansiReset  = "\033[0m"
	ansiCyan   = "\033[36m"
	ansiYellow = "\033[33m"
	ansiGreen  = "\033[32m"
	ansiRed    = "\033[31m"
	ansiDim    = "\033[2m"
)

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

	// Internal command: --tab-source-toggle <statefile> [<sshconfig>]
	// Toggles tab source between "tag" and "source", resets to index 0, outputs reloaded list.
	if len(args) >= 2 && args[0] == "--tab-source-toggle" {
		var configArg string
		if len(args) >= 3 {
			configArg = args[2]
		}
		tabSourceToggle(args[1], resolveSSHConfigPath(configArg))
		return
	}

	// Internal command: --edit-host-option <hostname> <sshconfig> [<keyword> [<value...>]]
	// Opens a fzf-based input dialog to edit one SSH config directive for hostname.
	if len(args) >= 3 && args[0] == "--edit-host-option" {
		optionLine := strings.Join(args[3:], " ")
		if err := editHostOption(args[1], resolveSSHConfigPath(args[2]), optionLine); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
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

	// Split at "--": everything before is ffh flags, everything after goes to ssh.
	ffhArgs, sshArgs := splitAtDoubleDash(args)

	// ffh --history [--delete <host> | --list]
	if len(ffhArgs) >= 1 && ffhArgs[0] == "--history" {
		rest := ffhArgs[1:]
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
		historyMode(sshArgs, sshConfigPath)
		return
	}

	// ffh --check [-F <sshconfig>]
	if len(ffhArgs) >= 1 && ffhArgs[0] == "--check" {
		configArg := extractSSHConfigFlagValue(ffhArgs[1:])
		checkDuplicates(resolveSSHConfigPath(configArg))
		return
	}

	// ffh --exec <tag> <command...>  (command args go after tag, no -- needed)
	if len(ffhArgs) >= 3 && ffhArgs[0] == "--exec" {
		sshConfigPath := resolveSSHConfigPath(extractSSHConfigFlagValue(ffhArgs))
		execTag(ffhArgs[1], ffhArgs[2:], sshConfigPath)
		return
	}

	// ffh --hosts [path] [-- ssh-options]
	if len(ffhArgs) >= 1 && ffhArgs[0] == "--hosts" {
		rest := ffhArgs[1:]
		var cliPath string
		if len(rest) >= 1 && !strings.HasPrefix(rest[0], "-") {
			cliPath = rest[0]
			rest = rest[1:]
		}
		if unknown := unknownFFHFlag(rest); unknown != "" {
			fmt.Fprintf(os.Stderr, msgs.errUnknownFlag+"\n", unknown)
			os.Exit(1)
		}
		hostsMode(resolveHostsPath(cliPath), sshArgs)
		return
	}

	if unknown := unknownFFHFlag(ffhArgs); unknown != "" {
		fmt.Fprintf(os.Stderr, msgs.errUnknownFlag+"\n", unknown)
		os.Exit(1)
	}

	sshConfigPath := resolveSSHConfigPath(extractSSHConfigFlagValue(ffhArgs))
	tabSource := resolveTabSource(extractTabSourceFlagValue(ffhArgs))
	sshMode(sshArgs, sshConfigPath, tabSource)
}

// tabState holds the ordered tag list and current index, persisted in a temp file.
// Format: "<idx>\n<source>\n<tag0>\n<tag1>\n..." where tag0 is always "All".
type tabState struct {
	tags   []string // tags[0] == "All"
	idx    int
	source string // "tag" or "source"
}

func loadTabState(path string) tabState {
	data, err := os.ReadFile(path)
	if err != nil {
		return tabState{}
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) < 3 {
		return tabState{}
	}
	idx, _ := strconv.Atoi(lines[0])
	src := lines[1]
	if src != "source" {
		src = "tag"
	}
	return tabState{tags: lines[2:], idx: idx, source: src}
}

func (s tabState) save(path string) {
	lines := []string{strconv.Itoa(s.idx), s.source}
	lines = append(lines, s.tags...)
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
}

func (s tabState) currentTag() string {
	if s.idx == 0 || s.idx >= len(s.tags) {
		return ""
	}
	return s.tags[s.idx]
}

func buildTabState(hosts []Host, source string) tabState {
	seen := make(map[string]bool)
	var items []string
	for _, h := range hosts {
		var key string
		if source == "source" {
			key = h.SourceFile
		} else {
			key = h.Tag
		}
		if key != "" && !seen[key] {
			seen[key] = true
			items = append(items, key)
		}
	}
	sort.Strings(items)
	return tabState{tags: append([]string{msgs.tabAll}, items...), idx: 0, source: source}
}

// tabDisplayName returns a short label for a tab value.
// In source mode, absolute paths are shortened by replacing the home dir with ~.
func tabDisplayName(value string, source string) string {
	if source == "source" && value != msgs.tabAll {
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(value, home) {
			return "~" + value[len(home):]
		}
		return filepath.Base(value)
	}
	return value
}

// terminalWidth returns the terminal column count via TIOCGWINSZ.
// FZF_COLUMNS is checked first because fzf sets it in reload/execute contexts.
// Falls back to 80 if unavailable.
func terminalWidth() int {
	if s := os.Getenv("FZF_COLUMNS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	type winsize struct {
		Row, Col, Xpixel, Ypixel uint16
	}
	var ws winsize
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdout), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws))); errno == 0 && ws.Col > 0 {
		return int(ws.Col)
	}
	return 80
}

// stripAnsi returns the visible (non-ANSI) length of s.
func stripAnsiLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\033' {
			inEsc = true
			continue
		}
		n++
	}
	return n
}

// renderHeader builds a single-line tab bar. When all tabs fit within the
// terminal width they are shown in full. Otherwise a sliding window centred on
// the selected tab is shown, with "←" / "→" indicators for hidden tabs.
func renderHeader(s tabState) string {
	width := terminalWidth()
	const indent = "  "
	const arrowL = "\033[2m ← \033[0m"
	const arrowR = "\033[2m → \033[0m"
	arrowW := stripAnsiLen(arrowL) // == stripAnsiLen(arrowR) == 3

	type tabPart struct {
		text string
		w    int
	}
	parts := make([]tabPart, len(s.tags))
	for i, t := range s.tags {
		label := tabDisplayName(t, s.source)
		var text string
		if i == s.idx {
			text = "\033[1;7m " + label + " \033[0m"
		} else {
			text = "\033[2m " + label + " \033[0m"
		}
		parts[i] = tabPart{text: text, w: stripAnsiLen(text)}
	}

	// Calculate total width for all tabs.
	total := len(indent)
	for i, p := range parts {
		if i > 0 {
			total++
		}
		total += p.w
	}
	if total <= width {
		var sb strings.Builder
		sb.WriteString(indent)
		for i, p := range parts {
			if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(p.text)
		}
		return sb.String() + "\n"
	}

	// Sliding window: expand outward from the selected tab until we run out of space.
	lo, hi := s.idx, s.idx

	windowWidth := func() int {
		w := len(indent)
		if lo > 0 {
			w += arrowW + 1
		}
		if hi < len(parts)-1 {
			w += 1 + arrowW
		}
		for i := lo; i <= hi; i++ {
			if i > lo {
				w++
			}
			w += parts[i].w
		}
		return w
	}

	for {
		expanded := false
		if lo > 0 {
			extra := parts[lo-1].w + 1
			// after expanding left: lo-1 might eliminate the left arrow if lo-1==0
			var arrowSave int
			if lo == 1 {
				arrowSave = arrowW + 1
			}
			if windowWidth()+extra-arrowSave <= width {
				lo--
				expanded = true
				continue
			}
		}
		if hi < len(parts)-1 {
			extra := 1 + parts[hi+1].w
			var arrowSave int
			if hi+1 == len(parts)-1 {
				arrowSave = 1 + arrowW
			}
			if windowWidth()+extra-arrowSave <= width {
				hi++
				expanded = true
				continue
			}
		}
		if !expanded {
			break
		}
	}

	var sb strings.Builder
	sb.WriteString(indent)
	if lo > 0 {
		sb.WriteString(arrowL)
		sb.WriteString(" ")
	}
	for i := lo; i <= hi; i++ {
		if i > lo {
			sb.WriteString(" ")
		}
		sb.WriteString(parts[i].text)
	}
	if hi < len(parts)-1 {
		sb.WriteString(" ")
		sb.WriteString(arrowR)
	}
	return sb.String() + "\n"
}

func filterHosts(hosts []Host, source string, key string) []string {
	var names []string
	for _, h := range hosts {
		if key == "" {
			names = append(names, h.Name)
			continue
		}
		if source == "source" {
			if h.SourceFile == key {
				names = append(names, h.Name)
			}
		} else {
			if h.Tag == key {
				names = append(names, h.Name)
			}
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
	names := filterHosts(hosts, s.source, s.currentTag())
	// Header on line 1 (consumed by --header-lines=1), hosts follow.
	fmt.Print(renderHeader(s))
	fmt.Print(strings.Join(names, "\n"))
}

// tabSourceToggle is called by fzf Ctrl-T binding. It toggles the tab source between
// "tag" and "source", resets to index 0, and outputs the reloaded list.
func tabSourceToggle(statefile string, sshConfigPath string) {
	s := loadTabState(statefile)
	if s.source == "source" {
		s.source = "tag"
	} else {
		s.source = "source"
	}
	hosts := loadHosts(sshConfigPath)
	s = buildTabState(hosts, s.source)
	s.save(statefile)
	names := filterHosts(hosts, s.source, "")
	fmt.Print(renderHeader(s))
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

func sshMode(args []string, sshConfigPath string, tabSource string) {
	hosts := loadHosts(sshConfigPath)

	// Build and persist initial tab state
	statefile := tempStateFile()
	s := buildTabState(hosts, tabSource)
	s.save(statefile)
	defer os.Remove(statefile)

	names := filterHosts(hosts, tabSource, "") // All
	exPath := selfPath()

	// Initial input: header on line 1 (consumed by --header-lines=1), hosts follow.
	initialInput := renderHeader(s) + strings.Join(names, "\n")

	// Tab = next tag, Shift-Tab = prev tag.
	bindNext := fmt.Sprintf("tab:reload(%s --tab-list %s 1 %s)", exPath, statefile, sshConfigPath)
	bindPrev := fmt.Sprintf("shift-tab:reload(%s --tab-list %s -1 %s)", exPath, statefile, sshConfigPath)
	// Ctrl-G opens a nested fzf showing full ssh -G output with per-option descriptions.
	bindConfigView := fmt.Sprintf("ctrl-g:execute(%s --ssh-config-view {} %s)", exPath, sshConfigPath)
	// Ctrl-Y copies the ssh command to clipboard.
	bindCopy := fmt.Sprintf("ctrl-y:execute(%s --copy-ssh-cmd {} %s)", exPath, sshConfigPath)
	// Ctrl-P refreshes the preview pane to show host connectivity check.
	bindCheck := fmt.Sprintf("ctrl-p:preview(%s --check-host {} %s)", exPath, sshConfigPath)
	// Ctrl-T toggles tab source between tag and source file grouping.
	bindToggleSource := fmt.Sprintf("ctrl-t:reload(%s --tab-source-toggle %s %s)", exPath, statefile, sshConfigPath)

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
			"--bind=" + bindToggleSource,
		},
	)
	if selected == "" {
		return
	}

	fmt.Fprintln(os.Stderr, msgs.msgConnectTo, selected)
	recordHistory(selected)
	// args is already ssh-only (split at "--"); just prepend -F if not already present.
	sshPassArgs := args
	if extractSSHConfigFlagValue(args) == "" {
		sshPassArgs = append([]string{"-F", sshConfigPath}, args...)
	}
	execSSH(selected, sshPassArgs)
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
	found := findHost(hosts, name)
	if found == nil {
		return fmt.Errorf("host not found: %s", name)
	}

	label := func(k, v string) {
		if v == "" {
			return
		}
		fmt.Printf("  %s%-14s%s %s%s%s\n", ansiBold, k+":", ansiReset, ansiCyan, v, ansiReset)
	}

	fmt.Printf("%s  Host:%s         %s%s%s\n", ansiBold, ansiReset, ansiCyan, found.Name, ansiReset)
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

	if e := findHistoryEntry(name); e != nil {
		ago := formatAgo(e.LastUsed)
		histLine := fmt.Sprintf("%s (%s x%d)", ago, msgs.labelHistoryConnected, e.ConnCount)
		fmt.Printf("  %s%-14s%s %s%s%s\n", ansiBold, msgs.labelLastUsed+":", ansiReset, ansiGreen, histLine, ansiReset)
	}

	if found.Description != "" {
		fmt.Println()
		fmt.Println("  " + strings.Repeat("─", 32))
		fmt.Printf("  %s%s%s\n", ansiBold, msgs.labelDescriptionSection, ansiReset)
		for _, line := range strings.Split(found.Description, "\n") {
			fmt.Printf("  %s%s%s\n", ansiYellow, line, ansiReset)
		}
	}

	return nil
}

// printHostCheck performs a TCP dial to the host's SSH port and prints UP/DOWN status.
// Output goes to stdout because this function is called from fzf --preview.
func printHostCheck(name string, sshConfigPath string) {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		fmt.Println(msgs.errParseSSHConfig, err)
		return
	}
	found := findHost(hosts, name)
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

	start := time.Now()
	conn, dialErr := net.DialTimeout("tcp", addr, 3*time.Second)
	elapsed := time.Since(start)

	if dialErr == nil {
		conn.Close()
		fmt.Printf("%s%s● %s%s  %s (%dms)%s\n",
			ansiBold, ansiGreen, ansiReset, ansiGreen,
			msgs.statusUp, elapsed.Milliseconds(), ansiReset)
	} else {
		fmt.Printf("%s%s○ %s%s  %s%s\n",
			ansiBold, ansiRed, ansiReset, ansiRed,
			msgs.statusDown, ansiReset)
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


// historyMode opens an fzf selector over connection history and connects to the selected host.
func historyMode(args []string, sshConfigPath string) {
	entries := loadHistorySorted()
	if len(entries) == 0 {
		fmt.Println(msgs.msgHistoryEmpty)
		return
	}

	exPath := selfPath()
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = formatHistoryLine(e)
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
	entries := loadHistorySorted()
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = formatHistoryLine(e)
	}
	fmt.Print(strings.Join(lines, "\n"))
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

	var dups []string
	for name, occ := range seen {
		if len(occ) > 1 {
			dups = append(dups, name)
		}
	}
	sort.Strings(dups)

	if len(dups) == 0 {
		fmt.Printf("%s%s%s\n", ansiGreen, msgs.msgNoDuplicates, ansiReset)
		return
	}

	fmt.Printf("%s%s%s\n\n", ansiBold, msgs.msgDuplicatesFound, ansiReset)
	for _, name := range dups {
		occ := seen[name]
		fmt.Printf("%s%s%s\n", ansiBold, name, ansiReset)
		for i, o := range occ {
			src := unexpandHome(o.file, must(os.UserHomeDir()))
			if i == 0 {
				fmt.Printf("  %s✓ %s%s  %s(%s)%s\n", ansiGreen, src, ansiReset, ansiDim, msgs.labelEffective, ansiReset)
			} else {
				fmt.Printf("  %s✗ %s%s  %s(%s)%s\n", ansiYellow, src, ansiReset, ansiDim, msgs.labelIgnored, ansiReset)
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

	colors := []string{ansiGreen, ansiYellow, "\033[34m", "\033[35m", ansiCyan}

	var wg sync.WaitGroup
	for i, h := range targets {
		wg.Add(1)
		go func(idx int, host Host) {
			defer wg.Done()
			color := colors[idx%len(colors)]
			prefix := fmt.Sprintf("%s%s[%s]%s ", ansiBold, color, host.Name, ansiReset)

			sshArgs := []string{"-F", sshConfigPath, host.Name}
			sshArgs = append(sshArgs, cmdArgs...)
			cmd := exec.Command(sshPath, sshArgs...)

			out, err := cmd.CombinedOutput()
			for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
				fmt.Printf("%s%s\n", prefix, line)
			}
			if err != nil {
				fmt.Printf("%s%s%s\n", prefix, msgs.errExecSSH+" "+err.Error(), ansiReset)
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

// runFzfQuery runs fzf as a text-input dialog using --print-query + --disabled.
// Returns (query, true) when the user presses Enter to confirm, ("", false) on Esc/abort.
// --disabled prevents fzf from filtering the dummy item, so Enter always accepts.
func runFzfQuery(initialQuery string, fzfArgs []string) (string, bool) {
	args := append([]string{"--query=" + initialQuery}, fzfArgs...)
	cmd := exec.Command("fzf", args...)
	// Feed one invisible placeholder so there is always a selectable item.
	cmd.Stdin = strings.NewReader(" ")
	cmd.Stderr = os.Stderr
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	// --print-query outputs the query on line 1 (line 2 is the selected item, ignored).
	lines := strings.SplitN(strings.TrimRight(out.String(), "\n"), "\n", 2)
	query := ""
	if len(lines) >= 1 {
		query = strings.TrimSpace(lines[0])
	}
	return query, err == nil
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

	src := unexpandHome(found.SourceFile, must(os.UserHomeDir()))
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
					if len(blockFields) >= 2 && strings.EqualFold(blockFields[1], hostname) && !strings.ContainsAny(blockFields[1], "*?") {
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
			if len(blockFields) >= 2 && strings.EqualFold(blockFields[1], hostname) && !strings.ContainsAny(blockFields[1], "*?") {
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

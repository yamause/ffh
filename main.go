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
	// Re-invocation as an SSH_ASKPASS helper (see credentialEnv). Checked before
	// anything else since ssh calls this with no other flags, just a prompt argument.
	if os.Getenv("FFH_ASKPASS_MODE") == "1" {
		initMessages()
		if err := runAskpass(); err != nil {
			if backend := credentialBackendByName(os.Getenv("FFH_CRED_BACKEND")); backend != nil && backend.isAuthError(err) {
				fmt.Fprintln(os.Stderr, msgs.warnCredNotSignedIn(backend.displayName()))
			}
			fatal("ffh askpass:", err)
		}
		return
	}

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
			fatal(err)
		}
		return
	}

	// Internal command used by fzf reload bindings: --tab-list <statefile> <delta> [<sshconfig>]
	// Outputs header as lines 1-2, then filtered host list. Used with --header-lines=2.
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

	// Internal command used by Ctrl-/ execute binding: --tab-jump <statefile>
	// Opens a nested fzf to fuzzy-select a tab by name and sets it as current;
	// the outer fzf's paired reload(--tab-list ... 0 ...) then re-renders on top of it.
	if len(args) >= 2 && args[0] == "--tab-jump" {
		tabJump(args[1])
		return
	}

	// Internal command used by "?" execute binding: --show-help
	// Opens a nested fzf listing every key binding available in sshMode.
	if len(args) >= 1 && args[0] == "--show-help" {
		showHelp()
		return
	}

	// Internal command: --edit-host-option <hostname> <sshconfig> [<keyword> [<value...>]]
	// Opens a fzf-based input dialog to edit one SSH config directive for hostname.
	if len(args) >= 3 && args[0] == "--edit-host-option" {
		optionLine := strings.Join(args[3:], " ")
		if err := editHostOption(args[1], resolveSSHConfigPath(args[2]), optionLine); err != nil {
			fatal(err)
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
			fatal(err)
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
				fatal(msgs.errHistoryNotFound, host)
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
			fatalf(msgs.errUnknownFlag+"\n", unknown)
		}
		hostsMode(resolveHostsPath(cliPath), sshArgs)
		return
	}

	if unknown := unknownFFHFlag(ffhArgs); unknown != "" {
		fatalf(msgs.errUnknownFlag+"\n", unknown)
	}

	sshConfigPath := resolveSSHConfigPath(extractSSHConfigFlagValue(ffhArgs))
	tabSource := resolveTabSource(extractTabSourceFlagValue(ffhArgs))
	sshMode(sshArgs, sshConfigPath, tabSource)
}

func loadHosts(sshConfigPath string) []Host {
	hosts, err := ParseSSHConfig(sshConfigPath)
	if err != nil {
		fatal(msgs.errParseSSHConfig, err)
	}
	return hosts
}

func sshMode(args []string, sshConfigPath string, tabSource string) {
	hosts := loadHosts(sshConfigPath)
	tagDelimiter := resolveTagDelimiter()

	// Build and persist initial tab state
	statefile := tempStateFile()
	s := buildTabState(hosts, tabSource, tagDelimiter)
	s.save(statefile)
	defer os.Remove(statefile)

	names := filterHosts(hosts, tabSource, "", tagDelimiter) // All
	exPath := selfPath()

	// Initial input: header on lines 1-2 (consumed by --header-lines=2), hosts follow.
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
	// Ctrl-/ opens a nested fzf to fuzzy-jump straight to a tab by name (k9s-style
	// command bar); tabJump saves the new index, then this reload() re-renders on it.
	bindTabJump := fmt.Sprintf("ctrl-/:execute(%s --tab-jump %s)+reload(%s --tab-list %s 0 %s)", exPath, statefile, exPath, statefile, sshConfigPath)
	// ? opens a nested fzf showing the full key-binding list (help overlay), since the
	// persistent header line only has room for a short hint.
	bindHelp := fmt.Sprintf("?:execute(%s --show-help)", exPath)

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
			"--header-lines=2",
			"--header-first",
			"--bind=" + bindNext,
			"--bind=" + bindPrev,
			"--bind=" + bindConfigView,
			"--bind=" + bindCopy,
			"--bind=" + bindCheck,
			"--bind=" + bindToggleSource,
			"--bind=" + bindTabJump,
			"--bind=" + bindHelp,
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
		fatal(msgs.errReadHostsFile, err)
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

	home, _ := os.UserHomeDir()
	src := unexpandHome(found.SourceFile, home)
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
		fatal(msgs.errParseSSHConfig, err)
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
	home, _ := os.UserHomeDir()
	for _, name := range dups {
		occ := seen[name]
		fmt.Printf("%s%s%s\n", ansiBold, name, ansiReset)
		for i, o := range occ {
			src := unexpandHome(o.file, home)
			if i == 0 {
				fmt.Printf("  %s✓ %s%s  %s(%s)%s\n", ansiGreen, src, ansiReset, ansiDim, msgs.labelEffective, ansiReset)
			} else {
				fmt.Printf("  %s✗ %s%s  %s(%s)%s\n", ansiYellow, src, ansiReset, ansiDim, msgs.labelIgnored, ansiReset)
			}
		}
		fmt.Println()
	}
}

// execTag runs a command on all hosts with the given tag concurrently, one
// goroutine per host, printing each host's combined output as it completes
// (interleaved, not host-by-host in order).
func execTag(tag string, cmdArgs []string, sshConfigPath string) {
	hosts := loadHosts(sshConfigPath)
	tagDelimiter := resolveTagDelimiter()
	var targets []Host
	for _, h := range hosts {
		for _, seg := range tagSegments(h.Tag, tagDelimiter) {
			if seg == tag {
				targets = append(targets, h)
				break
			}
		}
	}
	if len(targets) == 0 {
		fatal(msgs.errNoHostsForTag, tag)
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		fatal(msgs.errSSHNotFound)
	}

	colors := []string{ansiGreen, ansiYellow, "\033[34m", "\033[35m", ansiCyan}

	var wg sync.WaitGroup
	for i, h := range targets {
		wg.Add(1)
		go func(idx int, host Host) {
			defer wg.Done()
			color := colors[idx%len(colors)]
			prefix := fmt.Sprintf("%s%s[%s]%s ", ansiBold, color, host.Name, ansiReset)

			// Resolve any password-manager credential for this host, same as sshMode's
			// execSSH -- but never prompt to sign in here: targets run concurrently, and
			// a per-host interactive confirmation wouldn't make sense against N hosts at
			// once. A backend that isn't signed in just means this host connects without
			// one, same as the ordinary "no matching item" fallback.
			cred, _, _ := resolveCredential(sshConfigPath, host.Name)
			sshArgs := []string{"-F", sshConfigPath}
			if cred != nil && cred.username != "" {
				sshArgs = append(sshArgs, "-l", cred.username)
			}
			sshArgs = append(sshArgs, host.Name)
			sshArgs = append(sshArgs, cmdArgs...)
			cmd := exec.Command(sshPath, sshArgs...)
			if cred != nil {
				cmd.Env = append(os.Environ(), cred.env...)
			}

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
		fatal(msgs.errSSHNotFound)
	}
	sshConfigPath := extractSSHConfigFlagValue(args)
	cred, backend, notSignedIn := resolveCredential(sshConfigPath, host)
	if notSignedIn && confirmCredSignin(backend) {
		if err := backend.signin(); err != nil {
			fmt.Fprintln(os.Stderr, msgs.errCredSignin, err)
		} else {
			cred, _, _ = resolveCredential(sshConfigPath, host)
		}
	}

	sshArgs, credEnv := credentialSSHArgs(host, args, cred)
	env := append(os.Environ(), credEnv...)
	if err := syscall.Exec(sshPath, append([]string{"ssh"}, sshArgs...), env); err != nil {
		fatal(msgs.errExecSSH, err)
	}
}

// credentialSSHArgs returns the ssh argv (host and args, plus a resolved
// credential's -l <username> override) and the extra environment variables needed to
// arm SSH_ASKPASS for cred, given the caller's own ssh-args (so an explicit -l/-o
// User= from the caller still outranks a credential-provided username). cred may be
// nil, in which case this is just args prefixed with host and no extra env.
func credentialSSHArgs(host string, args []string, cred *credential) (sshArgs []string, env []string) {
	if cred != nil && cred.username != "" && !hasLoginOverride(args) {
		// -l on the command line outranks ssh_config's own User directive, so this
		// only takes effect for hosts without an explicit -l/-o User= from the caller.
		sshArgs = append(sshArgs, "-l", cred.username)
	}
	sshArgs = append(sshArgs, host)
	sshArgs = append(sshArgs, args...)
	if cred != nil {
		env = cred.env
	}
	return sshArgs, env
}

// hasLoginOverride reports whether args already specifies a login user via
// "-l <user>"/"-l<user>" or "-o User=<user>", so an explicit request from the caller
// always wins over a 1Password-derived username.
func hasLoginOverride(args []string) bool {
	for i, a := range args {
		switch {
		case a == "-l":
			return true
		case strings.HasPrefix(a, "-l") && a != "-l":
			return true
		case a == "-o" && i+1 < len(args) && strings.HasPrefix(strings.ToLower(args[i+1]), "user="):
			return true
		}
	}
	return false
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
		fatal(msgs.errTempFile, err)
	}
	f.Close()
	return f.Name()
}

// fatal prints args to stderr (fmt.Fprintln semantics) and exits with status 1. Used
// for every unrecoverable CLI error in main.go -- a 1Password/credential-backend
// outage is the one deliberate exception, handled non-fatally by resolveCredential.
func fatal(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

// fatalf is fatal with fmt.Fprintf-style formatting (no implicit trailing newline --
// include "\n" in format if needed, matching fmt.Fprintf's own behavior).
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

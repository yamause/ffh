package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

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

// tagSegments splits tag into the tab keys it should appear under. If delimiter is
// empty (feature disabled) or tag doesn't contain it, tag is returned as its own sole
// segment. Empty segments produced by leading/trailing delimiters are dropped, so
// "/hoge/fuga/" with delimiter "/" yields ["hoge", "fuga"], not ["", "hoge", "fuga", ""].
func tagSegments(tag, delimiter string) []string {
	if tag == "" {
		return nil
	}
	if delimiter == "" {
		return []string{tag}
	}
	var segments []string
	for _, part := range strings.Split(tag, delimiter) {
		if part != "" {
			segments = append(segments, part)
		}
	}
	if len(segments) == 0 {
		return []string{tag}
	}
	return segments
}

func buildTabState(hosts []Host, source string, tagDelimiter string) tabState {
	seen := make(map[string]bool)
	var items []string
	for _, h := range hosts {
		var keys []string
		if source == "source" {
			if h.SourceFile != "" {
				keys = []string{h.SourceFile}
			}
		} else {
			keys = tagSegments(h.Tag, tagDelimiter)
		}
		for _, key := range keys {
			if !seen[key] {
				seen[key] = true
				items = append(items, key)
			}
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
		if shortened := unexpandHome(value, home); shortened != value {
			return shortened
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
		return sb.String() + "\n" + renderKeyHints() + "\n"
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
		if lo > 0 {
			extra := parts[lo-1].w + 1
			// after expanding left: lo-1 might eliminate the left arrow if lo-1==0
			var arrowSave int
			if lo == 1 {
				arrowSave = arrowW + 1
			}
			if windowWidth()+extra-arrowSave <= width {
				lo--
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
				continue
			}
		}
		break
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
	return sb.String() + "\n" + renderKeyHints() + "\n"
}

// renderKeyHints returns a single dim, always-visible line summarizing the fzf key
// bindings available in sshMode, so the operations don't need to be memorized from
// the README (k9s-style persistent hint bar).
func renderKeyHints() string {
	return "  " + ansiDim + msgs.keyHintsSSH + ansiReset
}

func filterHosts(hosts []Host, source string, key string, tagDelimiter string) []string {
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
			continue
		}
		for _, seg := range tagSegments(h.Tag, tagDelimiter) {
			if seg == key {
				names = append(names, h.Name)
				break
			}
		}
	}
	return names
}

// tabList is called by fzf reload bindings. It advances the tab index by delta,
// then outputs: lines 1-2 = header (consumed by --header-lines=2), remaining lines = host names.
func tabList(statefile string, delta int, sshConfigPath string) {
	s := loadTabState(statefile)
	if len(s.tags) == 0 {
		return
	}
	s.idx = (s.idx + delta + len(s.tags)) % len(s.tags)
	s.save(statefile)

	hosts := loadHosts(sshConfigPath)
	names := filterHosts(hosts, s.source, s.currentTag(), resolveTagDelimiter())
	// Header on lines 1-2 (consumed by --header-lines=2), hosts follow.
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
	tagDelimiter := resolveTagDelimiter()
	s = buildTabState(hosts, s.source, tagDelimiter)
	s.save(statefile)
	names := filterHosts(hosts, s.source, "", tagDelimiter)
	fmt.Print(renderHeader(s))
	fmt.Print(strings.Join(names, "\n"))
}

// tabJump is called by fzf Ctrl-/ binding. It opens a nested fzf listing every tab
// name (fuzzy-searchable, k9s command-bar style) and, on selection, sets that tab as
// current in statefile. Does not itself print anything -- the outer bind chains this
// with reload(--tab-list ... 0 ...) to re-render once the new index is saved.
func tabJump(statefile string) {
	s := loadTabState(statefile)
	if len(s.tags) == 0 {
		return
	}
	labels := make([]string, len(s.tags))
	for i, t := range s.tags {
		labels[i] = tabDisplayName(t, s.source)
	}
	selected := runFzf(strings.Join(labels, "\n"), []string{
		"--layout=reverse",
		"--border=rounded",
		"--prompt=" + msgs.promptTabJump,
		"--header=" + msgs.tabJumpHeader,
		"--header-first",
	})
	if selected == "" {
		return
	}
	if idx := tabIndexByLabel(s, selected); idx >= 0 {
		s.idx = idx
		s.save(statefile)
	}
}

// tabIndexByLabel returns the index in s.tags whose display label matches label, or -1.
func tabIndexByLabel(s tabState, label string) int {
	for i, t := range s.tags {
		if tabDisplayName(t, s.source) == label {
			return i
		}
	}
	return -1
}

// formatHelpLines aligns key bindings into a "key  action" table, padding every key
// to the width of the longest one so the action column lines up regardless of how
// long each translated key/action string is.
func formatHelpLines(bindings []keyBinding) []string {
	maxKeyLen := 0
	for _, kb := range bindings {
		if len(kb.Key) > maxKeyLen {
			maxKeyLen = len(kb.Key)
		}
	}
	lines := make([]string, len(bindings))
	for i, kb := range bindings {
		lines[i] = fmt.Sprintf("%-*s  %s", maxKeyLen, kb.Key, kb.Action)
	}
	return lines
}

// showHelp is called by fzf's "?" binding. It opens a nested fzf listing every key
// binding available in sshMode; purely informational, so both Enter and Esc just
// close it without taking any action.
func showHelp() {
	lines := formatHelpLines(msgs.helpKeyBindings)
	runFzf(strings.Join(lines, "\n"), []string{
		"--layout=reverse",
		"--border=rounded",
		"--border-label=" + msgs.helpModalLabel,
		"--header=" + msgs.helpModalHeader,
		"--header-first",
		"--no-info",
		"--bind=enter:abort",
		"--bind=esc:abort",
	})
}

package main

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestHasLoginOverride(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"none", []string{"-F", "/etc/ssh_config", "-p", "22"}, false},
		{"separate -l", []string{"-l", "alice"}, true},
		{"combined -l", []string{"-lalice"}, true},
		{"-o User=", []string{"-o", "User=alice"}, true},
		{"-o user= lowercase value", []string{"-o", "user=alice"}, true},
		{"unrelated -o", []string{"-o", "ProxyJump=bastion"}, false},
		{"empty", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasLoginOverride(c.args); got != c.want {
				t.Errorf("hasLoginOverride(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

func TestTagSegments(t *testing.T) {
	cases := []struct {
		name      string
		tag       string
		delimiter string
		want      []string
	}{
		{"disabled feature keeps whole tag", "/hoge/fuga/", "", []string{"/hoge/fuga/"}},
		{"leading and trailing delimiters dropped", "/hoge/fuga/", "/", []string{"hoge", "fuga"}},
		{"no delimiter in tag falls back to whole tag", "hoge", "/", []string{"hoge"}},
		{"empty tag", "", "/", nil},
		{"tag is only delimiters", "//", "/", []string{"//"}},
		{"comma delimiter", "hoge,fuga", ",", []string{"hoge", "fuga"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tagSegments(c.tag, c.delimiter)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("tagSegments(%q, %q) = %v, want %v", c.tag, c.delimiter, got, c.want)
			}
		})
	}
}

func TestBuildTabState_TagDelimiterSplitsIntoMultipleTabs(t *testing.T) {
	hosts := []Host{
		{Name: "host1", Tag: "/hoge/fuga/"},
		{Name: "host2", Tag: "hoge"},
	}
	s := buildTabState(hosts, "tag", "/")
	got := append([]string{}, s.tags...)
	sort.Strings(got)
	want := []string{msgs.tabAll, "fuga", "hoge"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tabs = %v, want %v", got, want)
	}
}

func TestBuildTabState_NoDelimiterConfiguredKeepsWholeTag(t *testing.T) {
	hosts := []Host{{Name: "host1", Tag: "/hoge/fuga/"}}
	s := buildTabState(hosts, "tag", "")
	got := append([]string{}, s.tags...)
	sort.Strings(got)
	want := []string{msgs.tabAll, "/hoge/fuga/"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tabs = %v, want %v", got, want)
	}
}

func TestFilterHosts_TagDelimiterMatchesEitherSegment(t *testing.T) {
	hosts := []Host{
		{Name: "host1", Tag: "/hoge/fuga/"},
		{Name: "host2", Tag: "hoge"},
		{Name: "host3", Tag: "other"},
	}
	got := filterHosts(hosts, "tag", "fuga", "/")
	want := []string{"host1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterHosts(...) = %v, want %v", got, want)
	}

	got = filterHosts(hosts, "tag", "hoge", "/")
	want = []string{"host1", "host2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterHosts(...) = %v, want %v", got, want)
	}
}

func TestTabIndexByLabel_TagMode(t *testing.T) {
	s := tabState{tags: []string{"All", "dev", "prod"}, source: "tag"}
	cases := []struct {
		label string
		want  int
	}{
		{"All", 0},
		{"dev", 1},
		{"prod", 2},
		{"nonexistent", -1},
	}
	for _, c := range cases {
		if got := tabIndexByLabel(s, c.label); got != c.want {
			t.Errorf("tabIndexByLabel(%q) = %d, want %d", c.label, got, c.want)
		}
	}
}

func TestTabIndexByLabel_SourceMode(t *testing.T) {
	s := tabState{
		tags:   []string{"All", "/etc/ssh/config.d/dev", "/etc/ssh/config.d/prod"},
		source: "source",
	}
	// tabDisplayName shortens source-file tags to their base name.
	if got := tabIndexByLabel(s, "dev"); got != 1 {
		t.Errorf("tabIndexByLabel(dev) = %d, want 1", got)
	}
	if got := tabIndexByLabel(s, "prod"); got != 2 {
		t.Errorf("tabIndexByLabel(prod) = %d, want 2", got)
	}
	if got := tabIndexByLabel(s, "/etc/ssh/config.d/dev"); got != -1 {
		t.Errorf("tabIndexByLabel(full path) = %d, want -1 (only the base name matches)", got)
	}
}

func TestRenderHeader_HasTwoLinesWithKeyHints(t *testing.T) {
	s := tabState{tags: []string{"All", "dev"}, idx: 0, source: "tag"}
	out := renderHeader(s)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("renderHeader produced %d lines, want 2 (tab bar + key hints): %q", len(lines), out)
	}
}

func TestFormatHelpLines_AlignsToLongestKey(t *testing.T) {
	bindings := []keyBinding{
		{"Enter", "connect"},
		{"Tab/Shift-Tab", "cycle tabs"},
	}
	lines := formatHelpLines(bindings)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	wantKeyWidth := len("Tab/Shift-Tab")
	for i, line := range lines {
		gotAction := strings.TrimLeft(line[wantKeyWidth:], " ")
		if gotAction != bindings[i].Action {
			t.Errorf("line %d = %q, action column misaligned (want action %q starting right after %d-wide key column)",
				i, line, bindings[i].Action, wantKeyWidth)
		}
	}
}

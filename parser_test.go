package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseFile_BasicHost(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `Host myserver
    HostName 10.0.0.1
    User admin
    Port 2222
    ProxyJump bastion
    IdentityFile ~/.ssh/id_ed25519
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("want 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.Name != "myserver" {
		t.Errorf("Name = %q", h.Name)
	}
	if h.HostName != "10.0.0.1" {
		t.Errorf("HostName = %q", h.HostName)
	}
	if h.User != "admin" {
		t.Errorf("User = %q", h.User)
	}
	if h.Port != "2222" {
		t.Errorf("Port = %q", h.Port)
	}
	if h.ProxyJump != "bastion" {
		t.Errorf("ProxyJump = %q", h.ProxyJump)
	}
	if h.IdentityFile != "~/.ssh/id_ed25519" {
		t.Errorf("IdentityFile = %q", h.IdentityFile)
	}
}

func TestParseFile_WildcardSkipped(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `Host leaf*
    User admin

Host *
    ServerAliveInterval 60

Host realhost
    HostName 10.0.0.2
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Name != "realhost" {
		t.Errorf("expected only realhost, got %v", hosts)
	}
}

func TestParseFile_DescriptionInline(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `# Description: My production server
Host prod
    HostName 10.1.1.1
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hosts[0].Description != "My production server" {
		t.Errorf("Description = %q", hosts[0].Description)
	}
}

func TestParseFile_DescriptionMarkerWithBody(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `# Description:
# コメントをここに記載
# 複数行の入力可能
Host devstep
    HostName 10.198.255.254
    Port 10100
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "コメントをここに記載\n複数行の入力可能"
	if hosts[0].Description != want {
		t.Errorf("Description = %q, want %q", hosts[0].Description, want)
	}
}

func TestParseFile_DescriptionMarkerWithInlineAndBody(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `# Description: タイトル
# 詳細説明
# 続き
Host mixed
    HostName 10.0.0.1
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "タイトル\n詳細説明\n続き"
	if hosts[0].Description != want {
		t.Errorf("Description = %q, want %q", hosts[0].Description, want)
	}
}

func TestParseFile_DescriptionBlankLineSeparated(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `# Description: should not be picked up

Host prod
    HostName 10.1.1.1
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if hosts[0].Description != "" {
		t.Errorf("Description should be empty, got %q", hosts[0].Description)
	}
}

func TestParseFile_CaseInsensitiveKeywords(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `host casetest
    HOSTNAME 10.2.2.2
    USER root
    PORT 22
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("want 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.HostName != "10.2.2.2" {
		t.Errorf("HostName = %q", h.HostName)
	}
	if h.User != "root" {
		t.Errorf("User = %q", h.User)
	}
}

func TestParseFile_MatchBlockIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `Host realhost
    HostName 10.0.0.1

Match host fakematch
    User matchuser

Host another
    HostName 10.0.0.2
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, h := range hosts {
		names[h.Name] = true
	}
	if names["fakematch"] {
		t.Error("fakematch should not be in hosts")
	}
	if !names["realhost"] || !names["another"] {
		t.Errorf("expected realhost and another, got %v", names)
	}
}

func TestParseFile_BackToBackHosts(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", `Host first
    HostName 10.0.0.1
Host second
    HostName 10.0.0.2
`)
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 {
		t.Fatalf("want 2 hosts, got %d", len(hosts))
	}
	if hosts[0].HostName != "10.0.0.1" {
		t.Errorf("first HostName = %q", hosts[0].HostName)
	}
	if hosts[1].HostName != "10.0.0.2" {
		t.Errorf("second HostName = %q", hosts[1].HostName)
	}
}

func TestParseFile_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", "Host notrail\n    HostName 10.9.9.9")
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].HostName != "10.9.9.9" {
		t.Errorf("unexpected hosts: %v", hosts)
	}
}

func TestParseFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config", "")
	hosts, err := parseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty, got %v", hosts)
	}
}

func TestParseSSHConfig_IncludeGlob(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "config.d")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatal(err)
	}

	writeFile(t, subDir, "10_first", `Host alpha
    HostName 10.0.1.1
`)
	writeFile(t, subDir, "20_second", `Host beta
    HostName 10.0.1.2
`)

	mainConfig := writeFile(t, dir, "config", "Include "+subDir+"/*\n\nHost gamma\n    HostName 10.0.1.3\n")

	hosts, err := ParseSSHConfig(mainConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 3 {
		t.Fatalf("want 3 hosts, got %d: %v", len(hosts), hosts)
	}
	// Included files appear first
	if hosts[0].Name != "alpha" {
		t.Errorf("expected alpha first, got %s", hosts[0].Name)
	}
}

func TestParseSSHConfig_DuplicateFirstWins(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "config.d")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatal(err)
	}

	writeFile(t, subDir, "inc", `Host myhost
    HostName 10.0.0.1
`)
	mainConfig := writeFile(t, dir, "config", "Include "+subDir+"/*\n\nHost myhost\n    HostName 10.0.0.99\n")

	hosts, err := ParseSSHConfig(mainConfig)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("want 1 host, got %d", len(hosts))
	}
	if hosts[0].HostName != "10.0.0.1" {
		t.Errorf("first occurrence should win, got HostName %q", hosts[0].HostName)
	}
}

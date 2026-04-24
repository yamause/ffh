package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeHostsFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readHostsFile(path string) ([]HostEntry, error) {
	// Temporarily swap the constant by using a helper that takes a path.
	// We test the parsing logic directly via a helper function.
	return parseHostsFile(path)
}

func TestHostsFile_Basic(t *testing.T) {
	path := writeHostsFile(t, `10.0.0.1	myserver
10.0.0.2	otherhost
`)
	entries, err := readHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].IP != "10.0.0.1" || entries[0].Hostname != "myserver" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
}

func TestHostsFile_SkipLoopback(t *testing.T) {
	path := writeHostsFile(t, `127.0.0.1 localhost
127.0.1.1 myhostname
::1 localhost
10.0.0.1 realhost
`)
	entries, err := readHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Hostname != "realhost" {
		t.Errorf("expected only realhost, got %v", entries)
	}
}

func TestHostsFile_SkipCommentsAndBlanks(t *testing.T) {
	path := writeHostsFile(t, `# This is a comment

10.0.0.1 myhost
`)
	entries, err := readHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Hostname != "myhost" {
		t.Errorf("expected 1 entry myhost, got %v", entries)
	}
}

func TestHostsFile_MultipleHostnames(t *testing.T) {
	path := writeHostsFile(t, "10.0.0.3 core1 rt0003\n")
	entries, err := readHostsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Hostname != "core1" {
		t.Errorf("expected only first hostname, got %v", entries)
	}
}

func TestHostsFile_NotFound(t *testing.T) {
	_, err := parseHostsFile("/nonexistent/path/hosts")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

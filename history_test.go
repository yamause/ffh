package main

import (
	"os"
	"testing"
)

func TestHistoryFlow(t *testing.T) {
	home, _ := os.UserHomeDir()
	os.Remove(home + "/.local/share/ffh/history.json")

	recordHistory("bastion")
	recordHistory("web01")
	recordHistory("bastion")

	entries := loadHistory()
	found := map[string]int{}
	for _, e := range entries {
		found[e.Host] = e.ConnCount
	}
	if found["bastion"] != 2 {
		t.Errorf("bastion count want 2 got %d", found["bastion"])
	}
	if found["web01"] != 1 {
		t.Errorf("web01 count want 1 got %d", found["web01"])
	}

	ok := deleteHistoryEntry("web01")
	if !ok {
		t.Error("deleteHistoryEntry returned false")
	}
	entries = loadHistory()
	if len(entries) != 1 || entries[0].Host != "bastion" {
		t.Errorf("expected 1 remaining entry (bastion), got %v", entries)
	}
	os.Remove(home + "/.local/share/ffh/history.json")
}

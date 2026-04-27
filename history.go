package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type historyEntry struct {
	Host      string    `json:"host"`
	LastUsed  time.Time `json:"last_used"`
	ConnCount int       `json:"count"`
}

func historyFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "ffh", "history.json")
}

func loadHistory() []historyEntry {
	data, err := os.ReadFile(historyFilePath())
	if err != nil {
		return nil
	}
	var entries []historyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

func saveHistory(entries []historyEntry) error {
	path := historyFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// recordHistory adds or updates the history entry for host.
func recordHistory(host string) {
	entries := loadHistory()
	now := time.Now()
	for i, e := range entries {
		if e.Host == host {
			entries[i].LastUsed = now
			entries[i].ConnCount++
			_ = saveHistory(entries)
			return
		}
	}
	entries = append(entries, historyEntry{Host: host, LastUsed: now, ConnCount: 1})
	_ = saveHistory(entries)
}

// findHistoryEntry returns the entry for host, or nil if not found.
func findHistoryEntry(host string) *historyEntry {
	for _, e := range loadHistory() {
		if e.Host == host {
			cp := e
			return &cp
		}
	}
	return nil
}

// loadHistorySorted returns all history entries sorted by most recently used.
func loadHistorySorted() []historyEntry {
	entries := loadHistory()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})
	return entries
}

// formatHistoryLine formats a history entry for display in fzf.
func formatHistoryLine(e historyEntry) string {
	return fmt.Sprintf("%-30s  %s  x%d",
		e.Host,
		e.LastUsed.Format("2006-01-02 15:04"),
		e.ConnCount,
	)
}

// deleteHistoryEntry removes the entry for host. Returns true if found.
func deleteHistoryEntry(host string) bool {
	entries := loadHistory()
	var next []historyEntry
	found := false
	for _, e := range entries {
		if e.Host == host {
			found = true
		} else {
			next = append(next, e)
		}
	}
	if found {
		_ = saveHistory(next)
	}
	return found
}

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// deduplicateEvents

func TestDeduplicateEvents_RemovesDuplicates(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)
	ev := model.Event{Type: model.EventChanStart, Timestamp: ts, UniqueID: "x.1", LinkedID: "x.1"}
	events := []model.Event{ev, ev, ev}
	got := deduplicateEvents(events)
	if len(got) != 1 {
		t.Errorf("want 1 unique event, got %d", len(got))
	}
}

func TestDeduplicateEvents_KeepsDistinct(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)
	events := []model.Event{
		{Type: model.EventChanStart, Timestamp: ts, UniqueID: "x.1"},
		{Type: model.EventHangup, Timestamp: ts.Add(time.Second), UniqueID: "x.1"},
		{Type: model.EventChanStart, Timestamp: ts, UniqueID: "y.1"},
	}
	got := deduplicateEvents(events)
	if len(got) != 3 {
		t.Errorf("want 3 distinct events, got %d", len(got))
	}
}

func TestDeduplicateEvents_SameTypeDifferentTimestamp(t *testing.T) {
	ts := time.Unix(1_700_000_000, 0)
	events := []model.Event{
		{Type: model.EventAppStart, Timestamp: ts, UniqueID: "x.1"},
		{Type: model.EventAppStart, Timestamp: ts.Add(time.Second), UniqueID: "x.1"},
	}
	got := deduplicateEvents(events)
	if len(got) != 2 {
		t.Errorf("want 2 events (same type, different ts), got %d", len(got))
	}
}

// celPathsFromDir

func TestCelPathsFromDir_ReturnsSortedCSVFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"c.csv", "a.csv", "b.csv", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := celPathsFromDir(dir)
	if err != nil {
		t.Fatalf("celPathsFromDir error: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("want 3 csv paths, got %d", len(paths))
	}
	// sorted
	if filepath.Base(paths[0]) != "a.csv" || filepath.Base(paths[1]) != "b.csv" || filepath.Base(paths[2]) != "c.csv" {
		t.Errorf("paths not sorted: %v", paths)
	}
}

func TestCelPathsFromDir_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir.csv"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "real.csv"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	paths, err := celPathsFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "real.csv" {
		t.Errorf("want only real.csv, got %v", paths)
	}
}

func TestCelPathsFromDir_EmptyDir_Error(t *testing.T) {
	dir := t.TempDir()
	_, err := celPathsFromDir(dir)
	if err == nil {
		t.Fatal("want error for directory with no csv files, got nil")
	}
}

func TestCelPathsFromDir_NonExistentDir_Error(t *testing.T) {
	_, err := celPathsFromDir("/no/such/dir")
	if err == nil {
		t.Fatal("want error for non-existent directory, got nil")
	}
}

// splitColumns

func TestSplitColumns(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{"", []string{}},
	}
	for _, tt := range tests {
		got := splitColumns(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitColumns(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitColumns(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

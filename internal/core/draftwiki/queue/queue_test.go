package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAppendAndReadEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-queue.jsonl")

	event := Event{
		ID:        "test-1",
		Kind:      "append",
		RepoRoot:  "/tmp/repo",
		Status:    "pending",
		CreatedAt: "2024-01-01T00:00:00Z",
	}
	if err := AppendEvent(path, event); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	events, warnings, err := ReadEvents(path)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if events[0].ID != "test-1" {
		t.Errorf("expected ID test-1, got %s", events[0].ID)
	}
}

func TestReadEventsMissingFile(t *testing.T) {
	events, warnings, err := ReadEvents("/nonexistent/path.jsonl")
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected empty events for missing file")
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for missing file, got %v", warnings)
	}
}

func TestReadEventsMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	os.WriteFile(path, []byte("not json\n"), 0o600)

	events, warnings, err := ReadEvents(path)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for malformed line")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestCountLines(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		count, err := CountLines("/nonexistent.jsonl", 100)
		if err != nil {
			t.Fatalf("CountLines: %v", err)
		}
		if count != 0 {
			t.Errorf("expected 0, got %d", count)
		}
	})

	t.Run("with events", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		for i := 0; i < 5; i++ {
			e := Event{ID: "e", Status: "ok", CreatedAt: "now"}
			b, _ := json.Marshal(e)
			f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			f.Write(append(b, '\n'))
			f.Close()
		}

		count, err := CountLines(path, 100)
		if err != nil {
			t.Fatalf("CountLines: %v", err)
		}
		if count != 5 {
			t.Errorf("expected 5, got %d", count)
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		for i := 0; i < 10; i++ {
			e := Event{ID: "e", Status: "ok", CreatedAt: "now"}
			b, _ := json.Marshal(e)
			f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			f.Write(append(b, '\n'))
			f.Close()
		}

		count, err := CountLines(path, 3)
		if err != nil {
			t.Fatalf("CountLines: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 (limit), got %d", count)
		}
	})
}

func TestPrunePath(t *testing.T) {
	t.Run("prune to keep last N", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		for i := 0; i < 10; i++ {
			e := Event{ID: "e", Status: "ok", CreatedAt: "now"}
			b, _ := json.Marshal(e)
			f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			f.Write(append(b, '\n'))
			f.Close()
		}

		result, err := PrunePath(path, 3)
		if err != nil {
			t.Fatalf("PrunePath: %v", err)
		}
		if result.Before != 10 {
			t.Errorf("Before = %d, want 10", result.Before)
		}
		if result.After != 3 {
			t.Errorf("After = %d, want 3", result.After)
		}
		if result.Pruned != 7 {
			t.Errorf("Pruned = %d, want 7", result.Pruned)
		}
	})

	t.Run("keep all if under limit", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		for i := 0; i < 3; i++ {
			e := Event{ID: "e", Status: "ok", CreatedAt: "now"}
			b, _ := json.Marshal(e)
			f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			f.Write(append(b, '\n'))
			f.Close()
		}

		result, err := PrunePath(path, 10)
		if err != nil {
			t.Fatalf("PrunePath: %v", err)
		}
		if result.Pruned != 0 {
			t.Errorf("expected 0 pruned, got %d", result.Pruned)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := PrunePath("/nonexistent.jsonl", 5)
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func TestCapEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	// No events - should not error
	if err := CapEvents(path, 5); err != nil {
		t.Fatalf("CapEvents on missing file: %v", err)
	}

	// Write some events
	for i := 0; i < 20; i++ {
		e := Event{ID: "e", Status: "ok", CreatedAt: "now"}
		b, _ := json.Marshal(e)
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		f.Write(append(b, '\n'))
		f.Close()
	}

	// Cap at 5 - 20 > 10 (keep*2), so should prune
	if err := CapEvents(path, 5); err != nil {
		t.Fatalf("CapEvents: %v", err)
	}

	events, _, _ := ReadEvents(path)
	if len(events) != 5 {
		t.Errorf("expected 5 events after cap, got %d", len(events))
	}
}

func TestRewriteEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rewrite.jsonl")

	events := []Event{
		{ID: "a", Status: "ok", CreatedAt: "t1"},
		{ID: "b", Status: "ok", CreatedAt: "t2"},
	}
	if err := RewriteEvents(path, events); err != nil {
		t.Fatalf("RewriteEvents: %v", err)
	}

	read, _, err := ReadEvents(path)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(read) != 2 {
		t.Errorf("expected 2 events, got %d", len(read))
	}
}

func TestAcquireLock(t *testing.T) {
	dir := t.TempDir()

	unlock, acquired, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if !acquired {
		t.Error("expected lock acquired")
	}
	unlock()

	// Second lock should succeed after unlock
	unlock2, acquired2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("second AcquireLock: %v", err)
	}
	if !acquired2 {
		t.Error("expected second lock acquired")
	}
	unlock2()
}

func TestAcquireLockContention(t *testing.T) {
	dir := t.TempDir()

	unlock1, acquired1, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if !acquired1 {
		t.Fatal("expected first lock acquired")
	}
	defer unlock1()

	// Second lock should fail while first is held
	_, acquired2, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("second AcquireLock: %v", err)
	}
	if acquired2 {
		t.Error("expected second lock not acquired")
	}
}

func TestEventID(t *testing.T) {
	id1 := EventID("repo-1", "material", "2024-01-01")
	id2 := EventID("repo-1", "material", "2024-01-01")
	id3 := EventID("repo-2", "material", "2024-01-01")

	if id1 == "" {
		t.Error("expected non-empty id")
	}
	if id1 != id2 {
		t.Error("same inputs should produce same id")
	}
	if id1 == id3 {
		t.Error("different repo should produce different id")
	}
}

func TestTrimMaterial(t *testing.T) {
	t.Run("short material", func(t *testing.T) {
		got := TrimMaterial("hello world")
		if got != "hello world" {
			t.Errorf("expected unchanged, got %q", got)
		}
	})

	t.Run("empty material", func(t *testing.T) {
		got := TrimMaterial("")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		got := TrimMaterial("  \n  ")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestFormatMalformedWarning(t *testing.T) {
	msg := FormatMalformedWarning(5, "bad json here")
	if msg == "" {
		t.Error("expected non-empty warning")
	}
}

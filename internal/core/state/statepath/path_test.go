package statepath

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDir(t *testing.T) {
	t.Run("uses HARNESS_STATE_DIR", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HARNESS_STATE_DIR", dir)
		got := Dir()
		if got != dir {
			t.Errorf("Dir() = %q, want %q", got, dir)
		}
	})

	t.Run("default path", func(t *testing.T) {
		t.Setenv("HARNESS_STATE_DIR", "")
		got := Dir()
		if got == "" {
			t.Error("expected non-empty Dir")
		}
	})
}

func TestPath(t *testing.T) {
	got := Path("/state/dir", "my-key")
	expected := filepath.Join("/state/dir", "my-key.json")
	if got != expected {
		t.Errorf("Path() = %q, want %q", got, expected)
	}
}

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		key     string
		wantErr bool
	}{
		{"valid-key", false},
		{"valid.key_123", false},
		{"a", false},
		{"", true},
		{"../escape", true},
		{"path/separator", true},
		{"path\\separator", true},
		{"key with spaces", true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, err := NormalizeKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeKey(%q) error=%v, wantErr=%v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"rfc3339 nano", "2024-01-01T00:00:00.000000000Z", false},
		{"rfc3339", "2024-01-01T00:00:00Z", false},
		{"empty", "", true},
		{"invalid", "not-a-time", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTime(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime(%q) error=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}

	t.Run("returns correct time", func(t *testing.T) {
		ts, err := ParseTime("2024-06-15T12:00:00Z")
		if err != nil {
			t.Fatalf("ParseTime: %v", err)
		}
		if ts.Year() != 2024 || ts.Month() != time.June || ts.Day() != 15 {
			t.Errorf("wrong date: %v", ts)
		}
	})
}

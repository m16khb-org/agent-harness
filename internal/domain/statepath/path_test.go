package statepath

import (
	"testing"
	"time"
)

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
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			_, err := NormalizeKey(test.key)
			if (err != nil) != test.wantErr {
				t.Errorf("NormalizeKey(%q) error=%v, wantErr=%v", test.key, err, test.wantErr)
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
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseTime(test.value)
			if (err != nil) != test.wantErr {
				t.Errorf("ParseTime(%q) error=%v, wantErr=%v", test.value, err, test.wantErr)
			}
		})
	}
	ts, err := ParseTime("2024-06-15T12:00:00Z")
	if err != nil {
		t.Fatalf("ParseTime: %v", err)
	}
	if ts.Year() != 2024 || ts.Month() != time.June || ts.Day() != 15 {
		t.Errorf("wrong date: %v", ts)
	}
}

package agysettings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	t.Run("explicit path", func(t *testing.T) {
		got := ResolvePath("/custom/settings.json")
		if got != "/custom/settings.json" {
			t.Errorf("expected /custom/settings.json, got %q", got)
		}
	})

	t.Run("empty path uses default", func(t *testing.T) {
		home, _ := os.UserHomeDir()
		got := ResolvePath("")
		expected := filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestReadConfiguredModel(t *testing.T) {
	t.Run("reads model from valid json", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		settings := file{Model: "gemini-2.5-flash"}
		b, _ := json.Marshal(settings)
		os.WriteFile(path, b, 0o644)

		model, err := ReadConfiguredModel(path)
		if err != nil {
			t.Fatalf("ReadConfiguredModel: %v", err)
		}
		if model != "gemini-2.5-flash" {
			t.Errorf("expected gemini-2.5-flash, got %q", model)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := ReadConfiguredModel("/nonexistent/settings.json")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("empty model returns error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "settings.json")
		os.WriteFile(path, []byte(`{}`), 0o644)

		_, err := ReadConfiguredModel(path)
		if err == nil {
			t.Error("expected error for empty model")
		}
	})
}

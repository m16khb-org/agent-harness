package selfworkflow

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFileForWrapperTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func historySkippedKey(skipped []SelfAugmentHistorySkipped, key string) bool {
	for _, item := range skipped {
		if item.Key == key {
			return true
		}
	}
	return false
}

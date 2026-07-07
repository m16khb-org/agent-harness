package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNoRemovedProviderEndpointReferences(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current file")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	for _, relRoot := range []string{"internal", "cmd"} {
		root := filepath.Join(repoRoot, relRoot)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbidden := range []string{"api." + "z.ai", "Z_AI" + "_API_KEY"} {
				if strings.Contains(string(data), forbidden) {
					rel, relErr := filepath.Rel(repoRoot, path)
					if relErr != nil {
						rel = path
					}
					t.Errorf("%s contains forbidden external LLM reference %q", rel, forbidden)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", relRoot, err)
		}
	}
}

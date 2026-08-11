package architecture

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIssueOpsVerticalsShareOneRecordStore(t *testing.T) {
	productionPackages := loadProductionPackages(t)
	if !slices.Contains(productionPackages, "internal/adapter/outbound/issueopsrecord") {
		t.Fatal("missing shared IssueOps record store adapter")
	}

	requiredImporters := map[string]bool{
		"internal/adapter/outbound/issueopsartifact":  false,
		"internal/adapter/outbound/issueopsdecision":  false,
		"internal/adapter/outbound/issueopsinventory": false,
		"internal/adapter/outbound/issueopsretention": false,
		"internal/adapter/outbound/issueopsrouting":   false,
		"internal/adapter/outbound/issueopsstatus":    false,
	}
	for _, edge := range loadProductionEdges(t) {
		if edge.imported == "internal/adapter/outbound/issueopsrecord" {
			if _, required := requiredImporters[edge.importer]; required {
				requiredImporters[edge.importer] = true
			}
		}
	}
	for importer, found := range requiredImporters {
		if !found {
			t.Errorf("%s does not use the shared IssueOps record store", importer)
		}
	}

	root := findRepoRoot(t)
	for _, relative := range []string{
		"internal/adapter/outbound/issueopsartifact/codec.go",
		"internal/adapter/outbound/issueopsdecision/codec.go",
		"internal/adapter/outbound/issueopsrouting/codec.go",
	} {
		path := filepath.Join(root, relative)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("duplicate IssueOps record codec remains: %s", relative)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("inspect duplicate codec %s: %v", relative, err)
		}
	}
}

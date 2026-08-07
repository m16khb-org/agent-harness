package toolconformance

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestRejectsWrongCaseCountUnknownFixtureAndUnknownClassification(t *testing.T) {
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.BaselineCases = manifest.BaselineCases[:9]
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadManifest(data, nil); err == nil || !strings.Contains(err.Error(), "baseline case count = 9, want 10") {
		t.Fatalf("wrong case count error=%v", err)
	}
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.BaselineCases[0].FixtureID = "missing"
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadManifest(data, nil); err == nil || !strings.Contains(err.Error(), "baseline case 0 references unknown fixture missing") {
		t.Fatalf("unknown fixture error=%v", err)
	}
	if err := json.Unmarshal([]byte(`{"expected_classification":"invented"}`), &BaselineCase{}); err == nil {
		t.Fatal("unknown manifest classification accepted")
	}
}

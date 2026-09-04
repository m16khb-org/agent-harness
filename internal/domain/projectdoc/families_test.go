package projectdoc

import (
	"encoding/json"
	"strings"
	"testing"
)

// 패밀리 조회/경로/기록 메타 계약을 잠근다.
func TestDocFamilyLookupAndPaths(t *testing.T) {
	families := DocFamilies()
	if len(families) != 6 || families[0].Root != "ADR.md" || families[4].ModuleDir != "operations/guides" {
		t.Fatalf("family catalog wrong: %#v", families)
	}
	// 반환된 슬라이스를 수정해도 카탈로그가 오염되지 않는다.
	families[0].Root = "MUTATED"
	if again := DocFamilies(); again[0].Root != "ADR.md" {
		t.Fatalf("DocFamilies must return a defensive copy: %v", again[0].Root)
	}
	if f, ok := FamilyByRoot("CAUTIONS.md"); !ok || f.ModuleDir != "cautions" {
		t.Fatalf("FamilyByRoot wrong: %#v %v", f, ok)
	}
	if _, ok := FamilyByRoot("README.md"); ok {
		t.Fatal("unknown root must not resolve")
	}
	if f, ok := FamilyByModuleDir("operations/guides"); !ok || f.Root != "OPERATIONS.md" {
		t.Fatalf("FamilyByModuleDir wrong: %#v %v", f, ok)
	}
	if _, ok := FamilyByModuleDir("operations"); ok {
		t.Fatal("partial module dir must not resolve")
	}
	f, _ := FamilyByRoot("ADR.md")
	if f.OverviewRel() != "adr/overview.md" {
		t.Fatalf("overview rel wrong: %q", f.OverviewRel())
	}
	if !strings.HasSuffix(ManifestRelPath(), ".issueops/documentation/manifest.json") {
		t.Fatalf("manifest path wrong: %q", ManifestRelPath())
	}
}

func TestRecordMetaDescriptionCatalog(t *testing.T) {
	for _, family := range DocFamilies() {
		desc, ok := RecordMetaDescription(family.ModuleDir)
		if !ok || strings.TrimSpace(desc) == "" {
			t.Fatalf("family %s must have a record meta description", family.ModuleDir)
		}
	}
	if _, ok := RecordMetaDescription("unknown"); ok {
		t.Fatal("unknown module dir must not resolve")
	}
}

func TestOptionalProjectDocNamesSnapshot(t *testing.T) {
	names := OptionalProjectDocNames()
	if len(names) == 0 {
		t.Fatal("optional doc names must not be empty")
	}
	names[0] = "MUTATED"
	if again := OptionalProjectDocNames(); again[0] == "MUTATED" {
		t.Fatal("OptionalProjectDocNames must return a defensive copy")
	}
}

func TestSHA256HexStable(t *testing.T) {
	if got := SHA256Hex("x"); len(got) != 64 || got != SHA256Hex("x") {
		t.Fatalf("sha256 hex unstable: %q", got)
	}
	if SHA256Hex("x") == SHA256Hex("y") {
		t.Fatal("distinct inputs must hash differently")
	}
}

func TestManifestJSONMatchesCheckerSchema(t *testing.T) {
	var manifest struct {
		SchemaVersion  int `json:"schema_version"`
		MaxRootLines   int `json:"max_root_lines"`
		MaxModuleLines int `json:"max_module_lines"`
		Families       []struct {
			Root           string `json:"root"`
			ModuleDir      string `json:"module_dir"`
			Responsibility string `json:"responsibility"`
		} `json:"families"`
	}
	if err := json.Unmarshal([]byte(ManifestJSON()), &manifest); err != nil {
		t.Fatalf("manifest JSON invalid: %v", err)
	}
	if manifest.SchemaVersion != 1 || manifest.MaxRootLines != 250 || manifest.MaxModuleLines != 250 {
		t.Fatalf("manifest header wrong: %+v", manifest)
	}
	if len(manifest.Families) != len(DocFamilies()) {
		t.Fatalf("family count wrong: %d", len(manifest.Families))
	}
	if !strings.HasPrefix(manifest.Families[0].Root, ".issueops/ADR.md") {
		t.Fatalf("family root path wrong: %q", manifest.Families[0].Root)
	}
}

func TestPrefixedAndStringHelpers(t *testing.T) {
	prefixed := PrefixedProjectDocNames()
	if len(prefixed) == 0 || !strings.HasPrefix(prefixed[0], ".issueops/") {
		t.Fatalf("prefixed names wrong: %v", prefixed)
	}
	if got := NonEmptyStrings([]string{" a ", "", "b"}); len(got) != 2 || got[0] != "a" {
		t.Fatalf("NonEmptyStrings wrong: %v", got)
	}
	items := []string{"a"}
	if got := AppendUnique(items, "a"); len(got) != 1 {
		t.Fatalf("duplicate append must be a no-op: %v", got)
	}
	if got := AppendUnique(items, "b"); len(got) != 2 || got[1] != "b" {
		t.Fatalf("unique append must add: %v", got)
	}
}

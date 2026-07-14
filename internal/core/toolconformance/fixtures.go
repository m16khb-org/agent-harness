package toolconformance

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed testdata/fixture_manifest.json
var manifestJSON []byte

type Fixture struct {
	ID                string         `json:"id"`
	ProbeTool         string         `json:"probe_tool"`
	SourceTool        string         `json:"source_tool"`
	SchemaSHA256      string         `json:"schema_sha256"`
	ExpectedArguments map[string]any `json:"expected_arguments"`
}

type fixtureManifest struct {
	SchemaVersion int            `json:"schema_version"`
	Fixtures      []Fixture      `json:"fixtures"`
	BaselineCases []BaselineCase `json:"baseline_cases"`
}

func LoadFixtures(descriptors []ToolDescriptor) ([]Fixture, error) {
	fixtures, _, err := LoadManifest(descriptors)
	return fixtures, err
}
func LoadManifest(descriptors []ToolDescriptor) ([]Fixture, []BaselineCase, error) {
	return loadManifest(manifestJSON, descriptors)
}

func loadManifest(data []byte, descriptors []ToolDescriptor) ([]Fixture, []BaselineCase, error) {
	var d fixtureManifest
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, nil, err
	}
	if d.SchemaVersion != FixtureManifestVersion {
		return nil, nil, fmt.Errorf("unsupported fixture manifest version %d", d.SchemaVersion)
	}
	if err := validateBaselineCases(d.Fixtures, d.BaselineCases); err != nil {
		return nil, nil, err
	}
	m := map[string]ToolDescriptor{}
	for _, v := range descriptors {
		m[v.Name] = v
	}
	for _, f := range d.Fixtures {
		tool, ok := m[f.SourceTool]
		if !ok {
			return nil, nil, fmt.Errorf("source tool not found: %s", f.SourceTool)
		}
		actualSHA, err := CanonicalSchemaSHA256(tool.InputSchema)
		if err != nil {
			return nil, nil, err
		}
		if f.SchemaSHA256 != actualSHA {
			return nil, nil, fmt.Errorf("fixture %s source tool %s schema hash mismatch: want %s got %s", f.ID, f.SourceTool, f.SchemaSHA256, actualSHA)
		}
	}
	sort.Slice(d.Fixtures, func(i, j int) bool { return d.Fixtures[i].ID < d.Fixtures[j].ID })
	for i := range d.Fixtures {
		d.Fixtures[i].ExpectedArguments = cloneArguments(d.Fixtures[i].ExpectedArguments)
	}
	for i := range d.BaselineCases {
		d.BaselineCases[i].Arguments = cloneArguments(d.BaselineCases[i].Arguments)
	}
	return d.Fixtures, d.BaselineCases, nil
}
func CanonicalSchemaSHA256(schema map[string]any) (string, error) {
	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(schema); err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func validateBaselineCases(fixtures []Fixture, cases []BaselineCase) error {
	if len(cases) != 10 {
		return fmt.Errorf("baseline case count = %d, want 10", len(cases))
	}
	known := map[string]bool{}
	for _, fixture := range fixtures {
		known[fixture.ID] = true
	}
	for i, baseline := range cases {
		if !known[baseline.FixtureID] {
			return fmt.Errorf("baseline case %d references unknown fixture %s", i, baseline.FixtureID)
		}
	}
	return nil
}

func cloneArguments(in map[string]any) map[string]any {
	b, _ := json.Marshal(in)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

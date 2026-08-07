package benchmark

import (
	"path/filepath"
	"testing"
)

func TestPioneerFixturesLoadWithTargets(t *testing.T) {
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 15 {
		t.Fatalf("expected 15 fixtures (5 workflow + 10 pioneer), got %d", len(fixtures))
	}
	wantTargets := map[string]string{
		"pioneer-von-neumann": "von-neumann",
		"pioneer-turing":      "turing",
		"pioneer-berners-lee": "berners-lee",
		"pioneer-dijkstra":    "dijkstra",
		"pioneer-codd":        "codd",
		"pioneer-hopper":      "hopper",
		"pioneer-shannon":     "shannon",
		"pioneer-karpathy":    "karpathy",
		"pioneer-torvalds":    "torvalds",
		"pioneer-issueops":    "issueops",
	}
	targeted := 0
	for _, fixture := range fixtures {
		want, isPioneer := wantTargets[fixture.ID]
		if isPioneer {
			targeted++
			if fixture.PioneerSkillTarget != want {
				t.Fatalf("fixture %s: pioneer_skill_target=%q want %q", fixture.ID, fixture.PioneerSkillTarget, want)
			}
			if len(fixture.ExpectedPioneerArtifact) == 0 {
				t.Fatalf("fixture %s: expected_pioneer_artifact is required", fixture.ID)
			}
			continue
		}
		if fixture.PioneerSkillTarget != "" {
			t.Fatalf("non-pioneer fixture %s must have empty pioneer_skill_target, got %q", fixture.ID, fixture.PioneerSkillTarget)
		}
	}
	if targeted != 10 {
		t.Fatalf("expected 10 pioneer-targeted fixtures, got %d", targeted)
	}
}

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
	if len(fixtures) != 9 {
		t.Fatalf("expected 9 fixtures (5 workflow + 4 pioneer), got %d", len(fixtures))
	}
	wantTargets := map[string]string{
		"pioneer-dijkstra": "dijkstra",
		"pioneer-codd":     "codd",
		"pioneer-hopper":   "hopper",
		"pioneer-shannon":  "shannon",
	}
	targeted := 0
	for _, fixture := range fixtures {
		want, isPioneer := wantTargets[fixture.ID]
		if isPioneer {
			targeted++
			if fixture.PioneerSkillTarget != want {
				t.Fatalf("fixture %s: pioneer_skill_target=%q want %q", fixture.ID, fixture.PioneerSkillTarget, want)
			}
			continue
		}
		if fixture.PioneerSkillTarget != "" {
			t.Fatalf("non-pioneer fixture %s must have empty pioneer_skill_target, got %q", fixture.ID, fixture.PioneerSkillTarget)
		}
	}
	if targeted != 4 {
		t.Fatalf("expected 4 pioneer-targeted fixtures, got %d", targeted)
	}
}

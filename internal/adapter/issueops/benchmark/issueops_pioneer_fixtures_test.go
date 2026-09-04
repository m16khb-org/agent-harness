package benchmark

import (
	"path/filepath"
	"testing"

	"issueops/internal/domain/pioneerskill"
)

func TestPioneerFixturesLoadWithTargets(t *testing.T) {
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 18 {
		t.Fatalf("expected 18 fixtures (5 workflow + 12 namesake + issueops), got %d", len(fixtures))
	}
	wantTargets := map[string]string{"pioneer-issueops": "issueops"}
	for _, name := range pioneerskill.Names() {
		wantTargets["pioneer-"+name] = name
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
	if targeted != len(pioneerskill.Names())+1 {
		t.Fatalf("expected %d targeted fixtures, got %d", len(pioneerskill.Names())+1, targeted)
	}
}

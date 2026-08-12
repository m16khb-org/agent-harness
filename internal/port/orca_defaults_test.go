package port

import (
	"testing"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

func TestLegacyImplementerDefaultsMatchPreparationContract(t *testing.T) {
	for _, host := range []string{"codex", "claude", "omo", "unknown"} {
		legacyModel, legacyEffort, legacyOK := IssueOpsImplementerDefaults(host)
		model, effort, ok := preparationcontract.ImplementerDefaults(host)
		if legacyModel != model || legacyEffort != effort || legacyOK != ok {
			t.Fatalf("host=%s legacy=(%q,%q,%v) preparation=(%q,%q,%v)", host, legacyModel, legacyEffort, legacyOK, model, effort, ok)
		}
	}
}

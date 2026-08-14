package issueops

import "testing"

func TestValidateRecordAcceptsEveryCurrentPhase(t *testing.T) {
	for _, phase := range IssueOpsPhases {
		t.Run(string(phase), func(t *testing.T) {
			record := IssueOpsRecord{
				SchemaVersion: IssueOpsSchemaVersion,
				ID:            "io-valid",
				Phase:         phase,
				PhaseLedger: IssueOpsPhaseLedger{
					phase: {Phase: phase},
				},
			}

			if err := ValidateRecord(record); err != nil {
				t.Fatalf("ValidateRecord(%q): %v", phase, err)
			}
		})
	}
}

func TestValidateRecordAcceptsCompletePlanPreparation(t *testing.T) {
	evidence := IssueOpsPlanPrepItem{Status: "evidence", Evidence: []string{"source"}}
	waived := IssueOpsPlanPrepItem{Status: "waived", WaiveReason: "not needed"}
	record := IssueOpsRecord{
		SchemaVersion: IssueOpsSchemaVersion,
		ID:            "io-planprep",
		Phase:         IssueOpsPhasePlan,
		PlanPrep: &IssueOpsPlanPrep{
			PriorDecisions: evidence,
			RelatedIssues:  waived,
			WebResearch:    evidence,
			CodebaseSurvey: waived,
		},
	}

	if err := ValidateRecord(record); err != nil {
		t.Fatalf("ValidateRecord: %v", err)
	}
}

func TestValidateRecordRejectsInvalidPlanPreparationShapes(t *testing.T) {
	tests := []IssueOpsPlanPrepItem{
		{Status: "evidence", Evidence: []string{"source"}, WaiveReason: "also waived"},
		{Status: "waived"},
		{Status: "waived", Evidence: []string{"source"}, WaiveReason: "not needed"},
	}
	for index, item := range tests {
		record := IssueOpsRecord{
			SchemaVersion: IssueOpsSchemaVersion,
			ID:            "io-invalid-planprep",
			Phase:         IssueOpsPhasePlan,
			PlanPrep: &IssueOpsPlanPrep{
				PriorDecisions: item,
			},
		}

		if err := ValidateRecord(record); err == nil {
			t.Fatalf("case %d unexpectedly accepted: %+v", index, item)
		}
	}
}

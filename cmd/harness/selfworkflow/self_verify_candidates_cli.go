package selfworkflow

import (
	"flag"
	"fmt"
)

type SelfVerifyCandidatesDeps struct {
	Export func() SelfVerificationCandidateExportResult
	Save   func(result *SelfVerificationCandidateExportResult, key string) error
}

func (deps SelfVerifyCandidatesDeps) withDefaults() SelfVerifyCandidatesDeps {
	if deps.Export == nil {
		deps.Export = ExportSelfVerificationCandidates
	}
	if deps.Save == nil {
		deps.Save = SaveSelfVerificationCandidateExport
	}
	return deps
}

func RunSelfVerifyCandidates(args []string) error {
	return RunSelfVerifyCandidatesWithDeps(args, SelfVerifyCandidatesDeps{})
}

func RunSelfVerifyCandidatesWithDeps(args []string, deps SelfVerifyCandidatesDeps) error {
	deps = deps.withDefaults()
	fs := flag.NewFlagSet("self-verify candidates", flag.ContinueOnError)
	saveState := fs.Bool("save-state", false, "save candidate export snapshot to harness state")
	stateKey := fs.String("state-key", "self-verify-candidates-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := deps.Export()
	if *saveState {
		if err := deps.Save(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("%s candidates: %d candidate(s), selected=%s\n", result.KoreanName, result.CandidateCount, SelectedSelfVerificationCandidateID(result.SelectedCandidate))
	for _, candidate := range result.Candidates {
		fmt.Printf("- %s %s score=%.1f status=%s\n", candidate.ID, candidate.Category, candidate.Score, candidate.Status)
	}
	return nil
}

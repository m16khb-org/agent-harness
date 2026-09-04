package candidatescmd

import (
	"flag"
	"fmt"

	"issueops/cmd/issueops/selfworkflow/candidateexport"
)

type Deps struct {
	Export    func() candidateexport.SelfVerificationCandidateExportResult
	Save      func(result *candidateexport.SelfVerificationCandidateExportResult, key string) error
	PrintJSON func(any) error
}

func Run(args []string, deps Deps) error {
	fs := flag.NewFlagSet("self-verify candidates", flag.ContinueOnError)
	saveState := fs.Bool("save-state", false, "save candidate export snapshot to issueops state")
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
		return deps.PrintJSON(result)
	}
	fmt.Printf("%s candidates: %d candidate(s), selected=%s\n", result.KoreanName, result.CandidateCount, candidateexport.SelectedSelfVerificationCandidateID(result.SelectedCandidate))
	for _, candidate := range result.Candidates {
		fmt.Printf("- %s %s score=%.1f status=%s\n", candidate.ID, candidate.Category, candidate.Score, candidate.Status)
	}
	return nil
}

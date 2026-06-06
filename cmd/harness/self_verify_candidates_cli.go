package main

import (
	"flag"
	"fmt"
)

type selfVerifyCandidatesDeps struct {
	export func() SelfVerificationCandidateExportResult
	save   func(result *SelfVerificationCandidateExportResult, key string) error
}

func (deps selfVerifyCandidatesDeps) withDefaults() selfVerifyCandidatesDeps {
	if deps.export == nil {
		deps.export = exportSelfVerificationCandidates
	}
	if deps.save == nil {
		deps.save = saveSelfVerificationCandidateExport
	}
	return deps
}

func runSelfVerifyCandidates(args []string) error {
	return runSelfVerifyCandidatesWithDeps(args, selfVerifyCandidatesDeps{})
}

func runSelfVerifyCandidatesWithDeps(args []string, deps selfVerifyCandidatesDeps) error {
	deps = deps.withDefaults()
	fs := flag.NewFlagSet("self-verify candidates", flag.ContinueOnError)
	saveState := fs.Bool("save-state", false, "save candidate export snapshot to harness state")
	stateKey := fs.String("state-key", "self-verify-candidates-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := deps.export()
	if *saveState {
		if err := deps.save(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("%s candidates: %d candidate(s), selected=%s\n", result.KoreanName, result.CandidateCount, selectedSelfVerificationCandidateID(result.SelectedCandidate))
	for _, candidate := range result.Candidates {
		fmt.Printf("- %s %s score=%.1f status=%s\n", candidate.ID, candidate.Category, candidate.Score, candidate.Status)
	}
	return nil
}

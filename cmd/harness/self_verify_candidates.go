package main

import (
	"path/filepath"
	"time"
)

const selfVerificationCandidateExportKind = "self_verification_candidate_export"

type SelfVerificationCandidateExportResult struct {
	OK                    bool                        `json:"ok"`
	Kind                  string                      `json:"kind"`
	LoopKind              string                      `json:"loop_kind"`
	KoreanName            string                      `json:"korean_name"`
	HarnessRoot           string                      `json:"harness_root"`
	GeneratedAt           string                      `json:"generated_at"`
	SourcePath            string                      `json:"source_path"`
	SourceExists          bool                        `json:"source_exists"`
	CandidateCount        int                         `json:"candidate_count"`
	OpenCandidateIDs      []string                    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                    `json:"satisfied_candidate_ids"`
	SelectedCandidate     *SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []SelfVerificationCandidate `json:"candidates"`
	StateCheckpoint       *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	Warnings              []string                    `json:"warnings"`
}

type SelfVerificationCandidate struct {
	Priority             int      `json:"priority"`
	ID                   string   `json:"id"`
	Category             string   `json:"category"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	WhyNow               []string `json:"why_now"`
	VerifyWith           []string `json:"verify_with"`
	SatisfactionEvidence []string `json:"satisfaction_evidence,omitempty"`
}

type SelfVerificationCandidateExportStateSnapshot struct {
	SchemaVersion         int                         `json:"schema_version"`
	Kind                  string                      `json:"kind"`
	LoopKind              string                      `json:"loop_kind"`
	KoreanName            string                      `json:"korean_name"`
	OK                    bool                        `json:"ok"`
	HarnessRoot           string                      `json:"harness_root"`
	GeneratedAt           string                      `json:"generated_at"`
	SourcePath            string                      `json:"source_path"`
	CandidateCount        int                         `json:"candidate_count"`
	OpenCandidateIDs      []string                    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                    `json:"satisfied_candidate_ids"`
	SelectedCandidate     *SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []SelfVerificationCandidate `json:"candidates"`
}

func exportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	root := harnessRoot()
	sourcePath := filepath.Join(root, "skills", "self-verify", "CANDIDATES.md")
	sourceExists := exists(sourcePath)
	candidates := selfVerificationCandidateCatalog()
	openIDs := selfVerificationCandidateIDsByStatus(candidates, selfAugmentCandidateStatusOpen)
	satisfiedIDs := selfVerificationCandidateIDsByStatus(candidates, selfAugmentCandidateStatusSatisfied)
	var selected *SelfVerificationCandidate
	for _, candidate := range candidates {
		if candidate.Status != selfAugmentCandidateStatusOpen {
			continue
		}
		if selected == nil || candidate.Score > selected.Score || (candidate.Score == selected.Score && candidate.Priority < selected.Priority) {
			copyCandidate := candidate
			selected = &copyCandidate
		}
	}
	warnings := []string{}
	if !sourceExists {
		warnings = append(warnings, "skills/self-verify/CANDIDATES.md not found; using built-in candidate export catalog")
	}
	return SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            selfVerificationKoreanName,
		HarnessRoot:           root,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		SourcePath:            sourcePath,
		SourceExists:          sourceExists,
		CandidateCount:        len(candidates),
		OpenCandidateIDs:      openIDs,
		SatisfiedCandidateIDs: satisfiedIDs,
		SelectedCandidate:     selected,
		Candidates:            candidates,
		Warnings:              warnings,
	}
}

func selfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}

func selectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	if candidate == nil {
		return "none"
	}
	return candidate.ID
}

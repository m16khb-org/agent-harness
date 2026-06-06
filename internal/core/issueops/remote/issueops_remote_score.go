package remote

import (
	"fmt"
	"strings"
)

func ScoreIssueOpsRemoteCandidates(req IssueOpsRemoteScoringRequest) (IssueOpsRemoteScoringResult, error) {
	provider := strings.ToLower(strings.TrimSpace(firstNonEmpty(req.Provider, req.Issue.Provider)))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" && provider != "gitlab" {
		return IssueOpsRemoteScoringResult{OK: false, Provider: provider}, fmt.Errorf("unsupported issueops remote provider %q", provider)
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = defaultIssueOpsRemoteThreshold
	}
	if threshold > 1 {
		return IssueOpsRemoteScoringResult{OK: false, Provider: provider}, fmt.Errorf("threshold must be between 0 and 1")
	}
	issueText := strings.TrimSpace(req.Issue.Title + "\n" + req.Issue.Body)
	if issueText == "" {
		return IssueOpsRemoteScoringResult{OK: false, Provider: provider, Threshold: threshold}, fmt.Errorf("issue title or body is required")
	}
	issueTokens := issueOpsRemoteTokens(issueText)
	result := normalizeIssueOpsRemoteScoringResult(IssueOpsRemoteScoringResult{OK: true, Provider: provider, Threshold: threshold})
	for _, candidate := range req.IssueCandidates {
		item := scoreIssueOpsRemoteIssue(provider, threshold, issueTokens, candidate)
		if item.Selected {
			result.SelectedRelatedIssues = append(result.SelectedRelatedIssues, item)
		} else {
			result.RejectedRelatedIssues = append(result.RejectedRelatedIssues, item)
		}
	}
	for _, candidate := range req.LabelCandidates {
		item := scoreIssueOpsRemoteLabel(provider, threshold, issueTokens, candidate)
		if item.Selected {
			result.SelectedLabels = append(result.SelectedLabels, item)
		} else {
			result.RejectedLabels = append(result.RejectedLabels, item)
		}
	}
	sortIssueOpsRemoteScoredItems(result.SelectedRelatedIssues)
	sortIssueOpsRemoteScoredItems(result.RejectedRelatedIssues)
	sortIssueOpsRemoteScoredItems(result.SelectedLabels)
	sortIssueOpsRemoteScoredItems(result.RejectedLabels)
	result.ApplyInstructions = issueOpsRemoteApplyInstructions(provider, result.SelectedRelatedIssues, result.SelectedLabels)
	if len(req.IssueCandidates) > 0 && len(result.SelectedRelatedIssues) == 0 {
		result.Warnings = append(result.Warnings, "no related issue candidates met threshold")
	}
	if len(req.LabelCandidates) > 0 && len(result.SelectedLabels) == 0 {
		result.Warnings = append(result.Warnings, "no label candidates met threshold")
		result.ApplyInstructions = append(result.ApplyInstructions, "stop before remote artifact writes and choose an explicit manual label or rerun scoring with corrected candidates; do not create an unlabeled issue, pull request, or merge request")
	}
	return normalizeIssueOpsRemoteScoringResult(result), nil
}

func normalizeIssueOpsRemoteScoringResult(result IssueOpsRemoteScoringResult) IssueOpsRemoteScoringResult {
	if strings.TrimSpace(result.ExecutionClass) == "" {
		result.ExecutionClass = "background_join"
	}
	result.ReadOnly = true
	if strings.TrimSpace(result.JoinBefore) == "" {
		result.JoinBefore = "remote_artifact_write"
	}
	if result.SelectedRelatedIssues == nil {
		result.SelectedRelatedIssues = []IssueOpsRemoteScoredItem{}
	}
	if result.RejectedRelatedIssues == nil {
		result.RejectedRelatedIssues = []IssueOpsRemoteScoredItem{}
	}
	if result.SelectedLabels == nil {
		result.SelectedLabels = []IssueOpsRemoteScoredItem{}
	}
	if result.RejectedLabels == nil {
		result.RejectedLabels = []IssueOpsRemoteScoredItem{}
	}
	if result.ApplyInstructions == nil {
		result.ApplyInstructions = []string{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result
}

func scoreIssueOpsRemoteIssue(provider string, threshold float64, issueTokens map[string]bool, candidate IssueOpsRemoteIssueCandidate) IssueOpsRemoteScoredItem {
	score := scorePtr(candidate.Score)
	evidence := []string{}
	if candidate.Score != nil {
		evidence = append(evidence, "explicit score provided")
	} else {
		candidateText := strings.TrimSpace(candidate.Title + "\n" + candidate.Body + "\n" + strings.Join(candidate.Labels, " "))
		overlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(candidateText))
		titleOverlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(candidate.Title))
		labelOverlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(strings.Join(candidate.Labels, " ")))
		score = clampScore(0.55*overlap + 0.30*titleOverlap + 0.15*labelOverlap)
		evidence = append(evidence, fmt.Sprintf("token_overlap=%.2f", overlap), fmt.Sprintf("title_overlap=%.2f", titleOverlap), fmt.Sprintf("label_overlap=%.2f", labelOverlap))
	}
	selected := score >= threshold
	item := IssueOpsRemoteScoredItem{
		ID:        strings.TrimSpace(candidate.ID),
		Title:     strings.TrimSpace(candidate.Title),
		URL:       strings.TrimSpace(candidate.URL),
		Score:     score,
		Threshold: threshold,
		Selected:  selected,
		Evidence:  evidence,
	}
	if selected {
		item.ApplyHint = issueOpsRemoteIssueApplyHint(provider, item)
	} else {
		item.RejectReason = "score below threshold"
	}
	return item
}

func scoreIssueOpsRemoteLabel(provider string, threshold float64, issueTokens map[string]bool, candidate IssueOpsRemoteLabelCandidate) IssueOpsRemoteScoredItem {
	score := scorePtr(candidate.Score)
	evidence := []string{}
	if candidate.Score != nil {
		evidence = append(evidence, "explicit score provided")
	} else {
		labelText := strings.TrimSpace(candidate.Name + "\n" + candidate.Description + "\n" + strings.Join(candidate.Aliases, " "))
		overlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(labelText))
		heuristic := issueOpsRemoteLabelHeuristic(issueTokens, candidate)
		score = clampScore(0.65*overlap + 0.35*heuristic)
		evidence = append(evidence, fmt.Sprintf("token_overlap=%.2f", overlap), fmt.Sprintf("type_heuristic=%.2f", heuristic))
	}
	selected := score >= threshold
	item := IssueOpsRemoteScoredItem{
		Name:      strings.TrimSpace(candidate.Name),
		Score:     score,
		Threshold: threshold,
		Selected:  selected,
		Evidence:  evidence,
	}
	if selected {
		item.ApplyHint = issueOpsRemoteLabelApplyHint(provider, item.Name)
	} else {
		item.RejectReason = "score below threshold"
	}
	return item
}

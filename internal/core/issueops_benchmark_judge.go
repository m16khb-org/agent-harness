package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type IssueOpsAgyJudgeRequest struct {
	RepoRoot   string
	AgyCommand string
	Timeout    time.Duration
	Fixture    IssueOpsBenchmarkFixture
	Artifact   IssueOpsBenchmarkArtifact
}

func RunIssueOpsAgyJudge(req IssueOpsAgyJudgeRequest) (IssueOpsBenchmarkScore, error) {
	command := strings.TrimSpace(req.AgyCommand)
	if command == "" {
		command = "agy"
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	prompt, err := buildIssueOpsAgyJudgePrompt(req.Fixture, req.Artifact)
	if err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	cmd := exec.CommandContext(ctx, command, "-p", prompt)
	if strings.TrimSpace(req.RepoRoot) != "" {
		cmd.Dir = req.RepoRoot
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge timed out after %s", timeout)
	}
	if err != nil {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge failed: %s", boundedIssueOpsText(string(out)))
	}
	return decodeStrictIssueOpsBenchmarkScore(out)
}

func buildIssueOpsAgyJudgePrompt(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) (string, error) {
	payload, err := json.Marshal(struct {
		Fixture   IssueOpsBenchmarkFixture  `json:"fixture"`
		Artifact  IssueOpsBenchmarkArtifact `json:"artifact"`
		Rubric    []string                  `json:"rubric_dimensions"`
		Schema    string                    `json:"schema"`
		Strict    string                    `json:"strict_output"`
		MaxScores string                    `json:"max_scores"`
	}{
		Fixture:   fixture,
		Artifact:  artifact,
		Rubric:    issueOpsBenchmarkDimensions,
		Schema:    "Return one JSON object matching IssueOpsBenchmarkScore: ok, fixture_id, average_score, minimum_score, dimension_scores, deterministic_failures, judge_failures, critical_failures, passed.",
		Strict:    "Return JSON only. Do not include prose before or after the JSON object.",
		MaxScores: "Each dimension score is 0 to 5 and must include short evidence.",
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeStrictIssueOpsBenchmarkScore(out []byte) (IssueOpsBenchmarkScore, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge returned empty output")
	}
	if trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge output must be strict JSON object: %s", boundedIssueOpsText(string(trimmed)))
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var score IssueOpsBenchmarkScore
	if err := decoder.Decode(&score); err != nil {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("decode agy judge output: %w", err)
	}
	if decoder.More() {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge output contained multiple JSON values")
	}
	if err := ensureIssueOpsDecoderEOF(decoder); err != nil {
		return IssueOpsBenchmarkScore{}, err
	}
	if len(score.DimensionScores) == 0 {
		return IssueOpsBenchmarkScore{}, fmt.Errorf("agy judge output missing dimension_scores")
	}
	return score, nil
}

func ensureIssueOpsDecoderEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("agy judge output contained trailing JSON")
	} else if err != io.EOF {
		return fmt.Errorf("agy judge output contained trailing data: %w", err)
	}
	return nil
}

func boundedIssueOpsText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[:400] + "...[truncated]"
	}
	return s
}

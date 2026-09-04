package orca

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"issueops/internal/port"
)

func (c *Client) runJSON(ctx context.Context, cwd string, timeout time.Duration, argv []string, target any) (string, error) {
	output, err := c.runner.Run(ctx, cwd, timeout, argv)
	if err != nil {
		// 비영 종료여도 orca가 정상 ok:false envelope을 남겼다면 typed 오류
		// 코드를 복원한다 — command_failed로 뭉개면 소비처의 not_found 멱등
		// 정규화가 무력화되어 cleanup finish가 수렴하지 못한다(#97).
		// envelope 부재나 ok:true 모순은 원래 오류를 유지해 실패 신호를
		// 삼키지 않는다.
		if runErr, ok := errors.AsType[*port.OrcaError](err); ok && runErr.Code == "command_failed" {
			if runtimeID, envErr := decodeResult(output, nil); envErr != nil {
				if typed, typedOK := errors.AsType[*port.OrcaError](envErr); typedOK {
					return runtimeID, typed
				}
			}
		}
		return "", err
	}
	return decodeResult(output, target)
}

func (c *Client) runText(ctx context.Context, cwd string, timeout time.Duration, argv []string) (string, error) {
	output, err := c.runner.Run(ctx, cwd, timeout, argv)
	if err != nil {
		return "", err
	}
	if len(output.Stdout) > MaxEnvelopeBytes {
		return "", fmt.Errorf("Orca output exceeds %d bytes", MaxEnvelopeBytes)
	}
	return string(output.Stdout), nil
}

type worktreePayload struct {
	ID                string `json:"id"`
	InstanceID        string `json:"instanceId"`
	RepoID            string `json:"repoId"`
	Path              string `json:"path"`
	Head              string `json:"head"`
	Branch            string `json:"branch"`
	DisplayName       string `json:"displayName"`
	Comment           string `json:"comment"`
	BaseRef           string `json:"baseRef"`
	LinkedIssue       int    `json:"linkedIssue"`
	LinkedGitLabIssue *int   `json:"linkedGitLabIssue"`
	ParentWorktreeID  string `json:"parentWorktreeId"`
	Lineage           struct {
		Capture struct {
			Source     string `json:"source"`
			Confidence string `json:"confidence"`
		} `json:"capture"`
	} `json:"lineage"`
}

func (w worktreePayload) portValue() port.OrcaWorktree {
	branch := strings.TrimPrefix(strings.TrimSpace(w.Branch), "refs/heads/")
	return port.OrcaWorktree{
		ID: w.ID, InstanceID: w.InstanceID, RepoID: w.RepoID, Path: w.Path, Head: w.Head,
		Branch: branch, Name: w.DisplayName, Comment: w.Comment, BaseRef: w.BaseRef,
		Issue: w.LinkedIssue, GitLabIssue: w.LinkedGitLabIssue,
		ParentWorktreeID: w.ParentWorktreeID,
		LineageSource:    w.Lineage.Capture.Source, LineageConfidence: w.Lineage.Capture.Confidence,
	}
}

type terminalPayload struct {
	Handle        string `json:"handle"`
	PTYID         string `json:"ptyId"`
	WorktreeID    string `json:"worktreeId"`
	WorktreePath  string `json:"worktreePath"`
	TabID         string `json:"tabId"`
	LeafID        string `json:"leafId"`
	Title         string `json:"title"`
	Connected     bool   `json:"connected"`
	Writable      bool   `json:"writable"`
	PaneRuntimeID *int   `json:"paneRuntimeId"`
}

type visualLayoutPayload struct {
	Root struct {
		Tabs []struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		} `json:"tabs"`
	} `json:"root"`
}

type taskPayload struct {
	ID          string          `json:"id"`
	RunID       string          `json:"run_id"`
	TaskTitle   string          `json:"task_title"`
	DisplayName string          `json:"display_name"`
	Status      string          `json:"status"`
	CompletedAt string          `json:"completed_at"`
	Result      json.RawMessage `json:"result"`
}

func (t taskPayload) portValue() port.OrcaTask {
	return port.OrcaTask{RunID: t.RunID, ID: t.ID, Title: t.TaskTitle, DisplayName: t.DisplayName, Status: t.Status, CompletedAt: t.CompletedAt, HasResult: hasJSONValue(t.Result)}
}

type runPayload struct {
	ID        string `json:"id"`
	Objective string `json:"objective"`
}

func (r runPayload) portValue(runtimeID string) (port.OrcaRun, error) {
	id, err := validateRunID(r.ID)
	if err != nil || strings.TrimSpace(r.Objective) == "" || r.Objective != strings.TrimSpace(r.Objective) {
		return port.OrcaRun{}, &port.OrcaError{Code: "run_identity_incomplete", Invoked: true}
	}
	return port.OrcaRun{RuntimeID: runtimeID, ID: id, Objective: r.Objective}, nil
}

func hasJSONValue(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strings.TrimSpace(text) != ""
	}
	return true
}

func requireReturnedCount(kind string, length int, count *int) error {
	value := -1
	if count != nil {
		value = *count
	}
	if count == nil || value != length {
		return fmt.Errorf("Orca %s list is incomplete: count=%d returned=%d", kind, value, length)
	}
	return nil
}

func requireReturnedRunID(kind, expected string, returned *string) error {
	if returned == nil || *returned != expected {
		return &port.OrcaError{Code: kind + "_run_mismatch", Detail: "Orca " + kind + " list does not identify the requested Run", Invoked: true}
	}
	return nil
}

func requireCompleteList(kind string, length int, total *int, truncated bool) error {
	if total == nil {
		return &port.OrcaError{Code: "incomplete_list", Detail: fmt.Sprintf("Orca %s list completeness metadata is missing", kind), Invoked: true}
	}
	if truncated || *total != length {
		return &port.OrcaError{Code: "incomplete_list", Detail: fmt.Sprintf("Orca %s list is incomplete: totalCount=%d returned=%d truncated=%t", kind, *total, length, truncated), Invoked: true}
	}
	return nil
}

func (t terminalPayload) portValue() port.OrcaTerminal {
	return port.OrcaTerminal{Handle: t.Handle, PTYID: t.PTYID, WorktreeID: t.WorktreeID, WorktreePath: t.WorktreePath, TabID: t.TabID, LeafID: t.LeafID, Title: t.Title, Connected: t.Connected, Writable: t.Writable}
}

func stableVisualTabTitles(layouts []visualLayoutPayload) (map[string]string, error) {
	if len(layouts) > port.OrcaMaxBaselineIDs {
		return nil, fmt.Errorf("Orca visual layout inventory exceeds %d entries", port.OrcaMaxBaselineIDs)
	}
	result := make(map[string]string)
	totalTabs := 0
	for _, layout := range layouts {
		if len(layout.Root.Tabs) > port.OrcaMaxBaselineIDs {
			return nil, fmt.Errorf("Orca visual tab inventory exceeds %d entries", port.OrcaMaxBaselineIDs)
		}
		totalTabs += len(layout.Root.Tabs)
		if totalTabs > port.OrcaMaxBaselineIDs {
			return nil, fmt.Errorf("Orca visual tab inventory exceeds %d entries", port.OrcaMaxBaselineIDs)
		}
		for _, tab := range layout.Root.Tabs {
			tabID := strings.TrimSpace(tab.TabID)
			leafID := strings.TrimSpace(tab.ActiveLeafID)
			title := strings.TrimSpace(tab.Title)
			if tabID == "" || leafID == "" || title == "" {
				continue
			}
			if tabID != tab.TabID || leafID != tab.ActiveLeafID || title != tab.Title || len(tabID) > 1024 || len(leafID) > 1024 || len(title) > 4096 {
				return nil, fmt.Errorf("Orca visual tab identity is not canonical and bounded")
			}
			key := visualTabKey(tabID, leafID)
			if previous, ok := result[key]; ok && previous != title {
				return nil, fmt.Errorf("Orca visual tab identity has conflicting titles")
			}
			result[key] = title
		}
	}
	return result, nil
}

func visualTabKey(tabID, leafID string) string {
	if strings.TrimSpace(tabID) == "" || strings.TrimSpace(leafID) == "" {
		return ""
	}
	return strings.TrimSpace(tabID) + "\x00" + strings.TrimSpace(leafID)
}

func hostCommand(agent string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(agent)) {
	case "codex":
		return "codex", true
	case "claude":
		return "claude", true
	case "omo":
		return "omo", true
	default:
		return "", false
	}
}

func ownerAgentCommand(agent, model, reasoningEffort string, allowCodexHookTrustBypass bool) (string, bool) {
	model = strings.TrimSpace(model)
	reasoningEffort = strings.TrimSpace(reasoningEffort)
	if model == "" || strings.IndexByte(model, 0) >= 0 || strings.IndexByte(reasoningEffort, 0) >= 0 {
		return "", false
	}
	switch strings.TrimSpace(strings.ToLower(agent)) {
	case "codex":
		command := "codex --model " + shellSingleQuote(model)
		if reasoningEffort != "" {
			command += " -c model_reasoning_effort=" + shellSingleQuote(reasoningEffort)
		}
		if allowCodexHookTrustBypass {
			command += " --dangerously-bypass-hook-trust"
		}
		return command, true
	case "claude":
		command := "claude --model " + shellSingleQuote(model)
		if reasoningEffort != "" {
			command += " --effort " + shellSingleQuote(reasoningEffort)
		}
		return command, true
	case "omo":
		if reasoningEffort != "" {
			model += ":" + reasoningEffort
		}
		return "omo --model " + shellSingleQuote(model), true
	default:
		return "", false
	}
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func orcaIssueProvider(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "github"
	}
	return value, value == "github" || value == "gitlab"
}

func pathSelector(path string) string { return "path:" + strings.TrimSpace(path) }
func idSelector(id string) string     { return "id:" + strings.TrimSpace(id) }

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(strings.TrimSpace(left))
	rightAbs, rightErr := filepath.Abs(strings.TrimSpace(right))
	return leftErr == nil && rightErr == nil && filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func containsAllHelpFlags(value string, items []string) bool {
	present := map[string]struct{}{}
	for field := range strings.FieldsSeq(value) {
		field = strings.Trim(field, "[](),;:")
		if index := strings.IndexByte(field, '='); index >= 0 {
			field = field[:index]
		}
		if strings.HasPrefix(field, "--") {
			present[field] = struct{}{}
		}
	}
	for _, item := range items {
		if _, ok := present[item]; !ok {
			return false
		}
	}
	return true
}

type executionGateInventory struct {
	RuntimeID string
	Rows      []port.OrcaGate
}

var _ port.OrcaClient = (*Client)(nil)
var _ port.OrcaRunInventoryReader = (*Client)(nil)

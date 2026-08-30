package orca

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"agent-harness/internal/port"
)

func validateExecutionIntentRequest(req port.ExecutionOrcaIntentRequest) error {
	if req.Marker == "" || req.Marker != req.Probe.Marker {
		return fmt.Errorf("Orca intent marker does not match the sealed operation")
	}
	if err := validateExecutionPrepare(req.Workspace, req.Probe); err != nil {
		return err
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if req.Prepared != nil || req.Launch != nil || req.TerminalPTYID != "" || req.RunID != "" || req.RunBound || req.TaskID != "" {
			return fmt.Errorf("worktree intent contains a later-stage receipt")
		}
	case port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentRun, port.ExecutionOrcaIntentRunBind, port.ExecutionOrcaIntentTask, port.ExecutionOrcaIntentDispatch:
		if req.Prepared == nil {
			return fmt.Errorf("owner intent requires a sealed worktree receipt")
		}
		if err := validateExecutionOwnerLaunch(*req.Prepared, req.Probe, executionRequiredLaunch(req)); err != nil {
			return err
		}
		if req.Stage == port.ExecutionOrcaIntentTerminal && (req.TerminalPTYID != "" || req.RunID != "" || req.RunBound || req.TaskID != "") {
			return fmt.Errorf("terminal intent contains a later-stage receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentRun && (req.TerminalPTYID == "" || req.RunID != "" || req.RunBound || req.TaskID != "") {
			return fmt.Errorf("Run intent requires exactly one terminal receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentRunBind && (req.TerminalPTYID == "" || req.RunID == "" || req.RunBound || req.TaskID != "") {
			return fmt.Errorf("Run bind intent requires terminal and Run receipts")
		}
		if req.Stage == port.ExecutionOrcaIntentTask && (req.TerminalPTYID == "" || req.RunID == "" || !req.RunBound || req.TaskID != "") {
			return fmt.Errorf("task intent requires terminal and bound Run receipts")
		}
		if req.Stage == port.ExecutionOrcaIntentDispatch && (req.TerminalPTYID == "" || req.RunID == "" || !req.RunBound || req.TaskID == "") {
			return fmt.Errorf("dispatch intent requires terminal, bound Run, and task receipts")
		}
	default:
		return fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
	return nil
}

// validateExecutionIntentInspectionRequest는 외부 mutation 없이 Orca
// 인벤토리만 조회할 때의 봉인 메타데이터를 검증한다. worktree가 이미 정리된
// 복구 상황에서는 prompt/context 파일을 다시 읽을 수 없으므로 경로와 digest
// 봉인만 확인한다. InvokeIntent는 계속 validateExecutionIntentRequest를 사용해
// 실제 파일 내용까지 검증한다.
func validateExecutionIntentInspectionRequest(req port.ExecutionOrcaIntentRequest) error {
	if req.Marker == "" || req.Marker != req.Probe.Marker {
		return fmt.Errorf("Orca intent marker does not match the sealed operation")
	}
	if err := validateExecutionPrepare(req.Workspace, req.Probe); err != nil {
		return err
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		if req.Prepared != nil || req.Launch != nil || req.TerminalPTYID != "" || req.RunID != "" || req.RunBound || req.TaskID != "" {
			return fmt.Errorf("worktree intent contains a later-stage receipt")
		}
	case port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentRun, port.ExecutionOrcaIntentRunBind, port.ExecutionOrcaIntentTask, port.ExecutionOrcaIntentDispatch:
		if err := validateExecutionInspectionOwnerEnvelope(req); err != nil {
			return err
		}
		if req.Stage == port.ExecutionOrcaIntentTerminal && (req.TerminalPTYID != "" || req.RunID != "" || req.RunBound || req.TaskID != "") {
			return fmt.Errorf("terminal intent contains a later-stage receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentRun && (req.TerminalPTYID == "" || req.RunID != "" || req.RunBound || req.TaskID != "") {
			return fmt.Errorf("Run intent requires exactly one terminal receipt")
		}
		if req.Stage == port.ExecutionOrcaIntentRunBind && (req.TerminalPTYID == "" || req.RunID == "" || req.RunBound || req.TaskID != "") {
			return fmt.Errorf("Run bind intent requires terminal and Run receipts")
		}
		if req.Stage == port.ExecutionOrcaIntentTask && (req.TerminalPTYID == "" || req.RunID == "" || !req.RunBound || req.TaskID != "") {
			return fmt.Errorf("task intent requires terminal and bound Run receipts")
		}
		if req.Stage == port.ExecutionOrcaIntentDispatch && (req.TerminalPTYID == "" || req.RunID == "" || !req.RunBound || req.TaskID == "") {
			return fmt.Errorf("dispatch intent requires terminal, bound Run, and task receipts")
		}
	default:
		return fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
	return nil
}

func validateExecutionInspectionOwnerEnvelope(req port.ExecutionOrcaIntentRequest) error {
	if req.Prepared == nil || req.Launch == nil {
		return fmt.Errorf("owner intent requires sealed worktree and launch receipts")
	}
	prepared := req.Prepared
	if strings.TrimSpace(prepared.WorktreeID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" ||
		strings.TrimSpace(prepared.RepoID) == "" {
		return fmt.Errorf("owner intent worktree receipt is incomplete")
	}
	launch := req.Launch
	if !validExecutionSHA256(launch.PromptSHA256) || !validExecutionSHA256(launch.ContextPacketSHA256) ||
		!executionPathInsideRoot(req.Workspace.Root, launch.PromptPath) ||
		!executionPathInsideRoot(req.Workspace.Root, launch.ContextPacketPath) {
		return fmt.Errorf("owner intent launch receipt is incomplete")
	}
	return nil
}

func validExecutionSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func executionPathInsideRoot(root, path string) bool {
	root, path = filepath.Clean(strings.TrimSpace(root)), filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func executionRequiredLaunch(req port.ExecutionOrcaIntentRequest) port.ExecutionOrcaLaunchRequest {
	if req.Launch == nil {
		return port.ExecutionOrcaLaunchRequest{}
	}
	return *req.Launch
}

func validateExecutionIntentTerminal(terminal port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt, marker string) error {
	if err := validateExecutionTerminalReceipt(terminal, prepared); err != nil {
		return err
	}
	if strings.TrimSpace(terminal.Title) == marker || strings.TrimSpace(terminal.StableTabTitle) == marker {
		return nil
	}
	// 관측값을 함께 남긴다 — 기대값만 있으면 다음 사람이 같은 조사를 반복한다(#414).
	return fmt.Errorf(
		"Orca owner terminal does not match the sealed intent: tab title mismatch (stable_tab_title=%q title=%q expected=%q)",
		strings.TrimSpace(terminal.StableTabTitle), strings.TrimSpace(terminal.Title), marker)
}

// validateExecutionResolvedTerminal은 **create 응답의 exact PTY/handle로 골라낸**
// inventory 행에 적용하는 검증이다.
//
// 그 행은 executionSoleCreatedTerminal이 created identity로 고르고 둘 이상이면
// fail-closed하므로, 어느 terminal인지는 이미 확정돼 있다. 거기서 marker는
// 확정을 재확인하는 보조 축이지 유일한 근거가 아니다.
//
// 완화가 필요한 이유는 실측이다. relay 0.1.0+66c426c5173c는 모든 terminal의
// `stableTabTitle`을 null로 두고, live `title`은 Orca가 truncate한 뒤 에이전트가
// 자기 상태로 덮어쓴다(관측: stable_tab_title="" title="✳ Claude Code").
// 그 조합에서 marker 문자열 일치를 요구하면 얼마를 기다려도 성립하지 않아
// Orca 모드 prepare가 영구히 막힌다(#414, #169).
//
// 완화는 stable tab title이 **비어 있을 때만** 적용한다. 값이 있는데 다르면
// 다른 lifecycle의 terminal일 수 있으므로 계속 거부한다.
func validateExecutionResolvedTerminal(terminal port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt, marker string) error {
	if err := validateExecutionTerminalReceipt(terminal, prepared); err != nil {
		return err
	}
	if strings.TrimSpace(terminal.Title) == marker || strings.TrimSpace(terminal.StableTabTitle) == marker {
		return nil
	}
	if strings.TrimSpace(terminal.StableTabTitle) == "" {
		return nil
	}
	return fmt.Errorf(
		"Orca owner terminal does not match the sealed intent: tab title mismatch (stable_tab_title=%q title=%q expected=%q)",
		strings.TrimSpace(terminal.StableTabTitle), strings.TrimSpace(terminal.Title), marker)
}

// validateExecutionTerminalReceipt는 어긋난 축을 이름으로 보고한다.
//
// 예전에는 여섯 조건을 한 문구로 합쳐서, 실패했을 때 handle이 없는 것인지
// runtime이 다른 것인지 아직 연결되지 않은 것인지 구분할 수 없었다. 그 구분이
// 없으면 대기 상한을 늘려야 할지, 다른 식별자를 써야 할지 판단할 근거가 없다.
func validateExecutionTerminalReceipt(terminal port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt) error {
	switch {
	case strings.TrimSpace(terminal.Handle) == "":
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: handle is empty")
	case strings.TrimSpace(terminal.PTYID) == "":
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: pty id is empty")
	case strings.TrimSpace(prepared.RuntimeID) == "":
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: sealed runtime id is empty")
	case terminal.RuntimeID != prepared.RuntimeID:
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: runtime mismatch (observed=%q sealed=%q)",
			terminal.RuntimeID, prepared.RuntimeID)
	case terminal.WorktreeID != prepared.WorktreeID:
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: worktree mismatch (observed=%q sealed=%q)",
			terminal.WorktreeID, prepared.WorktreeID)
	case !terminal.Connected:
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: terminal is not connected")
	case !terminal.Writable:
		return fmt.Errorf("Orca owner terminal does not match the sealed intent: terminal is not writable")
	}
	return nil
}

func validateExecutionIntentRun(run port.OrcaRun, prepared port.ExecutionOrcaWorkspaceReceipt, objective string) error {
	if strings.TrimSpace(run.ID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" ||
		run.RuntimeID != prepared.RuntimeID || strings.TrimSpace(run.Objective) != strings.TrimSpace(objective) {
		return fmt.Errorf("Orca Run does not match the sealed runtime and intent")
	}
	return nil
}

func validateExecutionIntentTask(task port.OrcaTask, prepared port.ExecutionOrcaWorkspaceReceipt, runID, title, displayName string) error {
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" || task.RuntimeID != prepared.RuntimeID ||
		task.RunID != strings.TrimSpace(runID) ||
		strings.TrimSpace(task.Title) != strings.TrimSpace(title) || strings.TrimSpace(task.DisplayName) != strings.TrimSpace(displayName) {
		return fmt.Errorf("Orca owner task does not match the sealed runtime and launch identity")
	}
	return nil
}

func validateExecutionInvokedDispatch(dispatch port.OrcaDispatch, runtimeID, taskID, terminalHandle string, inject bool) error {
	if strings.TrimSpace(dispatch.ID) == "" || strings.TrimSpace(runtimeID) == "" || dispatch.RuntimeID != runtimeID || dispatch.TaskID != taskID ||
		strings.TrimSpace(terminalHandle) == "" || dispatch.AssigneeHandle != terminalHandle || dispatch.Injected != inject ||
		(!inject && strings.TrimSpace(dispatch.Preamble) == "") {
		return fmt.Errorf("Orca dispatch does not match the sealed task and terminal")
	}
	return nil
}

func validateExecutionObservedDispatch(dispatch port.OrcaDispatch, runtimeID, taskID string) error {
	if strings.TrimSpace(dispatch.ID) == "" || strings.TrimSpace(runtimeID) == "" || dispatch.RuntimeID != runtimeID || dispatch.TaskID != taskID ||
		strings.TrimSpace(dispatch.AssigneeHandle) == "" || strings.TrimSpace(dispatch.Status) == "" {
		return fmt.Errorf("Orca dispatch does not match the sealed task identity")
	}
	return nil
}

func validateExecutionInventoryRuntime(observed, sealed string) error {
	if strings.TrimSpace(observed) == "" || strings.TrimSpace(sealed) == "" || observed != sealed {
		return fmt.Errorf("Orca inventory runtime identity changed")
	}
	return nil
}

func validateExecutionPrepare(workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) error {
	if strings.TrimSpace(workspace.LifecycleID) == "" || !filepath.IsAbs(workspace.SourceRoot) || !filepath.IsAbs(workspace.Root) || strings.TrimSpace(workspace.Branch) == "" || strings.TrimSpace(workspace.BaseHead) == "" {
		return fmt.Errorf("Orca prepare requires an exact lifecycle and workspace identity")
	}
	if req.Host != "codex" && req.Host != "claude" && req.Host != "omo" || strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Marker) == "" {
		return fmt.Errorf("Orca prepare requires codex, claude, or omo with explicit model and marker")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != "github" && provider != "gitlab" {
		return fmt.Errorf("Orca prepare requires github or gitlab issue identity")
	}
	if req.Issue <= 0 {
		return fmt.Errorf("Orca %s prepare requires a positive issue number", provider)
	}
	markerProvider, providerOK := executionMarkerField(req.Marker, "provider")
	markerIssue, issueOK := executionMarkerField(req.Marker, "issue")
	if !providerOK || markerProvider != provider || !issueOK || markerIssue != strconv.Itoa(req.Issue) {
		return fmt.Errorf("Orca %s prepare marker does not seal the exact provider and issue number", provider)
	}
	if parent := strings.TrimSpace(workspace.ParentWorktree); parent != "" &&
		(!filepath.IsAbs(parent) || samePath(parent, workspace.SourceRoot) || samePath(parent, workspace.Root)) {
		return fmt.Errorf("Orca parent worktree must be an isolated absolute path")
	}
	return nil
}

func validateExecutionWorktree(row port.OrcaWorktree, workspace port.ExecutionWorkspaceRequest, req port.ExecutionOrcaProbeRequest) error {
	if strings.TrimSpace(row.RuntimeID) == "" || strings.TrimSpace(row.ID) == "" || strings.TrimSpace(row.RepoID) == "" || !samePath(row.Path, workspace.Root) || strings.TrimSpace(row.Branch) != workspace.Branch || strings.TrimSpace(row.Head) != workspace.BaseHead || strings.TrimSpace(row.Comment) != req.Marker {
		return fmt.Errorf("Orca worktree receipt does not match the canonical workspace identity")
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "github" && row.Issue != req.Issue {
		return fmt.Errorf("Orca worktree receipt does not match the linked GitHub issue")
	}
	// 공개 Orca CLI에는 GitLab IID 쓰기 flag가 없다. 정확한 comment marker를
	// 필수 봉인으로 사용하고, native 필드가 관찰되면 추가 교차검증한다.
	if provider == "gitlab" && row.GitLabIssue != nil && *row.GitLabIssue != req.Issue {
		return fmt.Errorf("Orca worktree receipt does not match the linked GitLab issue")
	}
	if strings.TrimSpace(workspace.ParentWorktree) != "" &&
		!explicitExecutionParentLineage(row, workspace.ParentWorktree) {
		return fmt.Errorf("Orca worktree receipt does not prove explicit parent lineage")
	}
	return nil
}

func explicitExecutionParentLineage(row port.OrcaWorktree, parentWorktree string) bool {
	if strings.TrimSpace(row.LineageConfidence) != "explicit" {
		return false
	}
	// create의 --parent-worktree는 explicit-cli-flag를, 이후 명시적 parent
	// 갱신은 manual-action을 기록한다. 둘 다 정확한 parent ID가 일치할 때만
	// 같은 명시적 lineage 증거로 인정한다.
	switch strings.TrimSpace(row.LineageSource) {
	case "explicit-cli-flag", "manual-action":
	default:
		return false
	}
	repoID, parentPath, ok := strings.Cut(strings.TrimSpace(row.ParentWorktreeID), "::")
	return ok && strings.TrimSpace(repoID) == strings.TrimSpace(row.RepoID) &&
		samePath(parentPath, parentWorktree)
}

func executionMarkerField(marker, name string) (string, bool) {
	prefix := name + "="
	value := ""
	seen := false
	for _, field := range strings.Fields(marker) {
		if !strings.HasPrefix(field, prefix) {
			continue
		}
		if seen {
			return "", false
		}
		seen = true
		value = strings.TrimPrefix(field, prefix)
		if value == "" {
			return "", false
		}
	}
	return value, seen
}

func validateExecutionLaunch(worktreeID, runID string, terminal port.OrcaTerminal, task port.OrcaTask, dispatch port.OrcaDispatch) error {
	if strings.TrimSpace(terminal.Handle) == "" || terminal.WorktreeID != worktreeID || !terminal.Connected || !terminal.Writable {
		return fmt.Errorf("Orca owner terminal receipt is incomplete")
	}
	if strings.TrimSpace(runID) == "" || task.RunID != runID || strings.TrimSpace(task.ID) == "" ||
		strings.TrimSpace(dispatch.ID) == "" || dispatch.TaskID != task.ID || dispatch.AssigneeHandle != terminal.Handle || !dispatch.Injected {
		return fmt.Errorf("Orca task or dispatch receipt is incomplete")
	}
	return nil
}

var executionPromptPlaceholder = regexp.MustCompile(`\{[A-Z][A-Z0-9_]*\}`)

func validateExecutionOwnerLaunch(prepared port.ExecutionOrcaWorkspaceReceipt, req port.ExecutionOrcaProbeRequest, launch port.ExecutionOrcaLaunchRequest) error {
	if strings.TrimSpace(prepared.WorktreeID) == "" || strings.TrimSpace(prepared.RuntimeID) == "" || strings.TrimSpace(prepared.RepoID) == "" {
		return fmt.Errorf("Orca workspace receipt is incomplete")
	}
	if req.Host != "codex" && req.Host != "claude" && req.Host != "omo" || strings.TrimSpace(req.Model) == "" {
		return fmt.Errorf("Orca owner launch requires an explicit first-party owner profile")
	}
	packet, err := readExecutionSealedFile(prepared.Workspace.Root, launch.ContextPacketPath)
	if err != nil {
		return fmt.Errorf("sealed context packet is invalid: %w", err)
	}
	if digestExecutionBytes(packet) != strings.ToLower(strings.TrimSpace(launch.ContextPacketSHA256)) {
		return fmt.Errorf("sealed context packet digest mismatch")
	}
	prompt, err := readExecutionSealedFile(prepared.Workspace.Root, launch.PromptPath)
	if err != nil {
		return fmt.Errorf("sealed owner prompt is invalid: %w", err)
	}
	if string(prompt) != launch.Prompt || digestExecutionBytes(prompt) != strings.ToLower(strings.TrimSpace(launch.PromptSHA256)) {
		return fmt.Errorf("sealed owner prompt digest mismatch")
	}
	if executionPromptPlaceholder.MatchString(launch.Prompt) || !strings.Contains(launch.Prompt, launch.ContextPacketPath) || !strings.Contains(launch.Prompt, launch.ContextPacketSHA256) {
		return fmt.Errorf("owner prompt is unresolved or does not bind the sealed packet")
	}
	return nil
}

func readExecutionSealedFile(root, path string) ([]byte, error) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("path must be inside the canonical worktree")
	}
	current := root
	parts := strings.Split(rel, string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("path must contain only real directories and a real file")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("path ancestor must be a directory")
		}
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() > 1<<20) {
			return nil, fmt.Errorf("file must be regular, private, and at most 1048576 bytes")
		}
	}
	return os.ReadFile(path)
}

func digestExecutionBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

var _ port.ExecutionOrcaProvisioner = (*ExecutionProvisioner)(nil)
var _ port.ExecutionOrcaOwnerInspector = (*ExecutionProvisioner)(nil)

package issueopsnext

import (
	"context"
	"fmt"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsnextdomain "issueops/internal/domain/issueopsnext"
)

type Service struct {
	ports Ports
}

func NewService(ports Ports) *Service { return &Service{ports: ports} }

// Next는 현재 단계를 판정한다. 읽기만 한다 — record를 쓰지 않고, fetch도
// provider 호출도 하지 않는다.
func (service *Service) Next(ctx context.Context, stateRoot, cwd, id string) (issueopsnextcontract.Result, error) {
	if service == nil || service.ports.ListCycles == nil || service.ports.ReadRecord == nil ||
		service.ports.Completion == nil || service.ports.Now == nil {
		return issueopsnextcontract.Result{OK: false}, fmt.Errorf("issueops next dependencies are required")
	}
	ports := service.ports
	sourceRoot := ""
	if ports.SourceRoot != nil {
		sourceRoot = ports.SourceRoot(cwd)
	}
	result := issueopsnextcontract.Result{
		OK:          true,
		GeneratedAt: ports.Now().UTC().Format(time.RFC3339),
		CWD:         cwd,
		CWDRole:     cwdRole(ports, cwd, sourceRoot),
		SourceRoot:  sourceRoot,
		Missing:     []string{},
	}
	actorHost, actorSession := "", ""
	if ports.Actor != nil {
		host, session, err := ports.Actor()
		if err != nil {
			result.Warnings = append(result.Warnings, "native session identity is unavailable: "+err.Error())
		} else {
			actorHost, actorSession = host, session
		}
	}
	if ports.PlannerDefaults != nil && actorHost != "" {
		if model, effort, ok := ports.PlannerDefaults(actorHost); ok {
			result.Review = issueopsnextcontract.Review{Model: model, Effort: effort}
		}
	}

	// 저장소 밖에서는 고를 사이클이 없다. repo 필터가 빈 값이면 목록이 모든
	// 사이클을 돌려주므로, 여기서 먼저 끊어야 무관한 사이클을 고르지 않는다.
	if !isRepository(ports, cwd) {
		result.CWDRole = issueopsnextcontract.CWDRoleOther
		result.SourceRoot = ""
		result.Warnings = append(result.Warnings, "not a git repository: "+cwd)
		return applyDecision(result, issueopsnextdomain.Classify(issueopsnextdomain.Input{})), nil
	}

	listing, err := ports.ListCycles(ctx, stateRoot, sourceRoot)
	if err != nil {
		return issueopsnextcontract.Result{OK: false}, err
	}
	selected, candidates := selectCycle(ports, listing.Entries, cwd, id)
	if len(candidates) > 0 {
		result.Stage = issueopsnextcontract.Stage{Key: issueopsnextcontract.StageAmbiguous}
		result.Candidates = candidates
		result.NextCommand = "issueops list --repo " + placeholder(sourceRoot, "<source_root>") + " --json"
		result.NextCommandKind = commandKind(result.NextCommand)
		return result, nil
	}
	if selected == nil {
		decision := issueopsnextdomain.Classify(issueopsnextdomain.Input{SourceRoot: sourceRoot})
		return applyDecision(result, decision), nil
	}
	result.Selected = entryOf(*selected)
	if selected.Invalid {
		result.Stage = issueopsnextcontract.Stage{Key: issueopsnextcontract.StageInvalid}
		result.NextCommand = "issueops status --id " + selected.ID + " --json"
		result.NextCommandKind = commandKind(result.NextCommand)
		return result, nil
	}
	record, err := ports.ReadRecord(stateRoot, selected.ID)
	if err != nil {
		result.Stage = issueopsnextcontract.Stage{Key: issueopsnextcontract.StageInvalid}
		result.NextCommand = "issueops status --id " + selected.ID + " --json"
		result.NextCommandKind = commandKind(result.NextCommand)
		result.Warnings = append(result.Warnings, "record is unreadable: "+err.Error())
		return result, nil
	}
	result.Selected = recordEntry(record)
	return applyDecision(result, issueopsnextdomain.Classify(service.buildInput(stateRoot, sourceRoot, record, listing.Entries, actorHost, actorSession))), nil
}

func (service *Service) buildInput(
	stateRoot, sourceRoot string,
	record issueopscontract.IssueOpsRecord,
	entries []issueopsinventorycontract.ListEntry,
	actorHost, actorSession string,
) issueopsnextdomain.Input {
	ports := service.ports
	input := issueopsnextdomain.Input{
		Record:         record,
		SourceRoot:     sourceRoot,
		ActorHost:      actorHost,
		ActorSessionID: actorSession,
		Completion: func(phase issueopscontract.IssueOpsPhase) issueopsnextdomain.Readiness {
			readiness := ports.Completion(record, phase)
			return issueopsnextdomain.Readiness{Ready: readiness.Ready, Missing: readiness.Missing}
		},
	}
	if ports.StagedArtifacts != nil {
		if names, err := ports.StagedArtifacts(stateRoot, record.ID); err == nil {
			for _, name := range names {
				if strings.TrimSpace(name) == "plan" {
					input.StagedPlan = true
				}
			}
		}
	}
	if ports.WriterlessCommand != nil {
		input.WriterlessRecovery = ports.WriterlessCommand(record)
	}
	if record.Execution == nil {
		input.RootConflictID = rootConflict(ports, record, sourceRoot, entries)
	} else {
		if ports.WorktreeState != nil {
			present, _, head := ports.WorktreeState(record.Execution.Workspace.Root)
			input.WorktreePresent, input.WorktreeHead = present, head
		}
		if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil && ports.ProcessLive != nil {
			input.HolderLive = ports.ProcessLive(*holder.SessionProcess)
		}
	}
	// local readiness는 fetch가 없어도 git을 읽으므로 실제로 필요한 두
	// phase에서만 부른다.
	if ports.LocalReadiness != nil &&
		(record.Phase == issueopscontract.IssueOpsPhaseAISlopClean || record.Phase == issueopscontract.IssueOpsPhaseFeedback) {
		readiness := ports.LocalReadiness(record)
		input.Local = &issueopsnextdomain.Readiness{Ready: readiness.Ready, Missing: readiness.Missing}
	}
	return input
}

// selectCycle은 설계의 선택 우선순위를 그대로 따른다. 정확히 하나로 좁혀지지
// 않으면 후보를 돌려주고 사용자가 --id로 고르게 한다.
func selectCycle(
	ports Ports,
	entries []issueopsinventorycontract.ListEntry,
	cwd, id string,
) (*issueopsinventorycontract.ListEntry, []issueopsnextcontract.Entry) {
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		for index := range entries {
			if entries[index].ID == trimmed {
				return &entries[index], nil
			}
		}
		return &issueopsinventorycontract.ListEntry{ID: trimmed, Invalid: true}, nil
	}
	if ports.Env != nil {
		if trimmed := strings.TrimSpace(ports.Env("ISSUEOPS_ID")); trimmed != "" {
			for index := range entries {
				if entries[index].ID == trimmed {
					return &entries[index], nil
				}
			}
			return &issueopsinventorycontract.ListEntry{ID: trimmed, Invalid: true}, nil
		}
	}
	if matched := matchOne(entries, func(entry issueopsinventorycontract.ListEntry) bool {
		return entry.WorkspaceRoot != "" && sameDir(ports, entry.WorkspaceRoot, cwd)
	}); matched != nil {
		return matched, nil
	}
	if ports.CurrentBranch != nil {
		if branch := strings.TrimSpace(ports.CurrentBranch(cwd)); branch != "" {
			if matched := matchOne(entries, func(entry issueopsinventorycontract.ListEntry) bool {
				return strings.TrimSpace(entry.Branch) == branch
			}); matched != nil {
				return matched, nil
			}
		}
	}
	var active []issueopsinventorycontract.ListEntry
	for _, entry := range entries {
		if entry.Phase != issueopscontract.IssueOpsPhaseDone {
			active = append(active, entry)
		}
	}
	switch len(active) {
	case 0:
		return nil, nil
	case 1:
		return &active[0], nil
	default:
		candidates := make([]issueopsnextcontract.Entry, 0, len(active))
		for _, entry := range active {
			candidates = append(candidates, *entryOf(entry))
		}
		return nil, candidates
	}
}

func matchOne(
	entries []issueopsinventorycontract.ListEntry,
	predicate func(issueopsinventorycontract.ListEntry) bool,
) *issueopsinventorycontract.ListEntry {
	var found *issueopsinventorycontract.ListEntry
	for index := range entries {
		if !predicate(entries[index]) {
			continue
		}
		if found != nil {
			return nil
		}
		found = &entries[index]
	}
	return found
}

// rootConflict는 이 사이클이 쓸 canonical worktree root를 이미 점유한 다른
// 사이클을 찾는다. EnsureRootUnclaimed가 phase·lease와 무관하게 거부하므로
// prepare 전에 알려 준다.
func rootConflict(
	ports Ports,
	record issueopscontract.IssueOpsRecord,
	sourceRoot string,
	entries []issueopsinventorycontract.ListEntry,
) string {
	branch := strings.TrimSpace(record.Branch)
	if branch == "" || strings.TrimSpace(sourceRoot) == "" {
		return ""
	}
	root := sourceRoot + ".worktrees/" + strings.ReplaceAll(branch, "/", "-")
	for _, entry := range entries {
		if entry.ID != record.ID && entry.WorkspaceRoot != "" && sameDir(ports, entry.WorkspaceRoot, root) {
			return entry.ID
		}
	}
	return ""
}

func applyDecision(result issueopsnextcontract.Result, decision issueopsnextdomain.Decision) issueopsnextcontract.Result {
	result.Stage = decision.Stage
	result.Lease = decision.Lease
	result.Missing = decision.Missing
	if result.Missing == nil {
		result.Missing = []string{}
	}
	result.NextCommand = decision.NextCommand
	result.NextCommandKind = decision.NextCommandKind
	result.Exits = decision.Exits
	result.Warnings = append(result.Warnings, decision.Warnings...)
	return result
}

func entryOf(entry issueopsinventorycontract.ListEntry) *issueopsnextcontract.Entry {
	return &issueopsnextcontract.Entry{
		ID:                entry.ID,
		Phase:             string(entry.Phase),
		Branch:            entry.Branch,
		WorkspaceRoot:     entry.WorkspaceRoot,
		RemoteArtifactURL: entry.RemoteArtifactURL,
	}
}

func recordEntry(record issueopscontract.IssueOpsRecord) *issueopsnextcontract.Entry {
	entry := &issueopsnextcontract.Entry{
		ID:       record.ID,
		Phase:    string(record.Phase),
		Branch:   record.Branch,
		IssueURL: record.IssueURL,
	}
	if record.Execution != nil {
		entry.WorkspaceRoot = record.Execution.Workspace.Root
	}
	if record.RemoteArtifact != nil {
		entry.RemoteArtifactURL = record.RemoteArtifact.URL
	}
	return entry
}

func cwdRole(ports Ports, cwd, sourceRoot string) string {
	if strings.TrimSpace(sourceRoot) == "" {
		return issueopsnextcontract.CWDRoleOther
	}
	if sameDir(ports, cwd, sourceRoot) {
		return issueopsnextcontract.CWDRoleSource
	}
	if strings.HasPrefix(cleanPath(ports, cwd)+"/", cleanPath(ports, sourceRoot)+".worktrees/") {
		return issueopsnextcontract.CWDRoleWorktree
	}
	return issueopsnextcontract.CWDRoleOther
}

func sameDir(ports Ports, left, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}
	return cleanPath(ports, left) == cleanPath(ports, right)
}

// isRepository는 관측기가 없으면 참으로 본다. 테스트 배선처럼 git 관측을
// 꽂지 않은 곳에서 모든 디렉터리를 저장소 밖으로 몰지 않기 위해서다.
func isRepository(ports Ports, cwd string) bool {
	if ports.WorktreeState == nil {
		return true
	}
	present, _, _ := ports.WorktreeState(cwd)
	return present
}

func cleanPath(ports Ports, path string) string {
	trimmed := strings.TrimSpace(path)
	if ports.CleanPath == nil {
		return strings.TrimSuffix(trimmed, "/")
	}
	return ports.CleanPath(trimmed)
}

func placeholder(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

func commandKind(command string) string {
	if strings.Contains(command, "<") || strings.Contains(command, "$ACTOR_FLAGS") {
		return issueopsnextcontract.NextCommandKindTemplate
	}
	return issueopsnextcontract.NextCommandKindExact
}

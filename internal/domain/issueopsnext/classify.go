// Package issueopsnext classifies which stage of the ten-stage IssueOps cycle a
// record is in. It is pure: no filesystem, git, process, or provider access —
// every observation arrives through Input, so the same record always yields the
// same decision and the rules are table-testable.
package issueopsnext

import (
	"strconv"
	"strings"

	issueopsnextcontract "agent-harness/internal/contract/issueopsnext"
	issueopsdomain "agent-harness/internal/domain/issueops"
)

// Readiness는 주입된 게이트 판정이다. adapter의 IssueOpsReadiness에서 이
// 분류기가 쓰는 두 필드만 남긴 것이다.
type Readiness struct {
	Ready   bool
	Missing []string
}

type Input struct {
	Record issueopsnextcontract.Record
	// Completion은 phase별 완료 판정이다(adapter IssueOpsPhaseCompletion).
	Completion func(phase issueopsnextcontract.Phase) Readiness
	// Local은 fetch 없는 PR readiness다. phase가 ai-slop-clean·feedback일 때만
	// 채운다. nil이면 5~8단계 규칙이 발화하지 않는다.
	Local *Readiness
	// StagedPlan은 `artifact stage --name plan`이 올라와 있는지다.
	StagedPlan bool
	// RootConflictID는 이 사이클의 canonical worktree root를 이미 점유한 다른
	// 사이클의 id다. 없으면 빈 문자열이다.
	RootConflictID string
	// WriterlessRecovery는 writer 없는 lease의 회복 명령이다(adapter가 렌더).
	// 비어 있으면 replace --preview로 떨어진다.
	WriterlessRecovery string
	ActorHost          string
	ActorSessionID     string
	// HolderLive는 holder 프로세스 관측 결과다. holder가 없거나 관측하지
	// 않았으면 nil이다.
	HolderLive      *bool
	WorktreePresent bool
	WorktreeHead    string
	SourceRoot      string
}

type Decision struct {
	Stage           issueopsnextcontract.Stage
	Lease           issueopsnextcontract.Lease
	Missing         []string
	NextCommand     string
	NextCommandKind string
	Exits           issueopsnextcontract.Exits
	Warnings        []string
}

// 1단계 게이트의 고정 순서다. missing은 알파벳 정렬로 돌아오므로 첫 항목을
// 그대로 쓰면 2단계 항목인 branch가 먼저 나온다.
var stageOneOrder = []string{
	"intent_contract", "raw_request", "interpreted_intent", "success_criteria",
	"domain_review", "split_decision",
	"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey",
	"issue_url",
}

var cleanCompletionKeys = []string{
	"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint",
	"cleanup_evidence", "verification_evidence",
}

var cleanLocalKeys = []string{"ai_slop_clean_stale", "ai_slop_clean_fingerprint", "current_fingerprint"}

var docsLocalKeys = []string{"project_docs_review", "project_docs_review_stale"}

var verifyLocalKeys = []string{
	"schema_evidence", "schema_evidence_stale",
	"implementation_review", "implementation_review_stale",
	"feedback_classification", "feedback_resolution", "contract_feedback_issue_update",
}

// commitPushLocalKeys는 8단계로 넘어가도 되는 잔여 missing이다. 이 둘은 커밋과
// 푸시가 해소하므로 검증 단계로 되돌릴 이유가 없다.
var commitPushLocalKeys = []string{"worktree_clean", "upstream"}

// Classify는 규칙을 위에서 아래로 본다. 먼저 맞은 규칙이 결과를 결정한다.
func Classify(in Input) Decision {
	decision := Decision{Lease: leaseView(in)}
	if strings.TrimSpace(in.Record.ID) == "" {
		decision.Stage = stage(issueopsnextcontract.StageNone, 0)
		decision.NextCommand = "agent-harness issueops start --repo " + placeholder(in.SourceRoot, "<source_root>") + " --json"
		return finish(in, decision)
	}
	decision.Exits = exits(in)
	decision.Warnings = warnings(in)

	index := phaseIndex(in.Record.Phase)
	execution := in.Record.Execution
	id := in.Record.ID

	if execution != nil && execution.Pending != nil {
		decision.Stage = stage(issueopsnextcontract.StageBlockedPending, index)
		decision.NextCommand = "agent-harness issueops execution reconcile --id " + id + " --preview"
		return finish(in, decision)
	}
	if execution == nil && phaseRank(in.Record.Phase) >= phaseRank(issueopsnextcontract.PhasePlan) &&
		strings.TrimSpace(in.RootConflictID) != "" {
		decision.Stage = stage(issueopsnextcontract.StageBlockedRoot, 3)
		decision.NextCommand = "agent-harness issueops list --repo " + placeholder(in.SourceRoot, "<source_root>") + " --json"
		decision.Warnings = append(decision.Warnings, "canonical worktree root is already claimed by cycle "+strings.TrimSpace(in.RootConflictID))
		return finish(in, decision)
	}
	if execution != nil {
		lease := execution.Lease
		holderOther := lease.Holder != nil && !holderIsSelf(in)
		if lease.Status == issueopsnextcontract.LeaseStatusActive && holderOther && live(in.HolderLive) {
			decision.Stage = stage(issueopsnextcontract.StageBlockedHolderLive, index)
			decision.NextCommand = "agent-harness issueops execution status --id " + id + " --json"
			return finish(in, decision)
		}
		if (lease.Status == issueopsnextcontract.LeaseStatusActive && holderOther && !live(in.HolderLive)) ||
			lease.Status == issueopsnextcontract.LeaseStatusRevoking {
			decision.Stage = stage(issueopsnextcontract.StageTakeover, index)
			decision.NextCommand = takeoverCommand(in)
			return finish(in, decision)
		}
		if (lease.Status == issueopsnextcontract.LeaseStatusClaimable || lease.Status == issueopsnextcontract.LeaseStatusReleased) &&
			execution.Completion == nil {
			decision.Stage = stage(issueopsnextcontract.StageClaim, index)
			decision.NextCommand = "agent-harness issueops execution status --id " + id + " --json"
			return finish(in, decision)
		}
	}
	if decided, ok := classifyPreparation(in, decision); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyImplementation(in, decision); ok {
		return finish(in, decided)
	}
	if decided, ok := classifyPublication(in, decision, index); ok {
		return finish(in, decided)
	}
	decision.Stage = stage(issueopsnextcontract.StageUnknown, index)
	decision.NextCommand = "agent-harness issueops status --id " + id + " --json"
	if in.Local != nil && len(in.Local.Missing) > 0 {
		decision.Missing = append([]string{}, in.Local.Missing...)
		decision.Warnings = append(decision.Warnings, "unmatched readiness gates: "+strings.Join(in.Local.Missing, ", "))
	}
	return finish(in, decision)
}

// classifyPreparation은 execution이 아직 없는 1~3단계를 가른다. phase가 아니라
// artifact로 가르는 이유는 link-issue가 phase를 plan으로 자동 전진시키기
// 때문이다 — 이슈만 만든 사이클도 phase는 plan이다.
func classifyPreparation(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Execution != nil {
		return decision, false
	}
	if !planningPhase(record.Phase) {
		return decision, false
	}
	if strings.TrimSpace(record.IssueURL) == "" &&
		(record.Phase == issueopsnextcontract.PhaseProblem || record.Phase == issueopsnextcontract.PhaseGrill) {
		decision.Stage = stage(issueopsnextcontract.StageIssue, 1)
		decision.Missing = stageOneMissing(in)
		decision.NextCommand = OwnerCommand(id, firstOr(decision.Missing, "intent_contract"))
		return decision, true
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return decision, false
	}
	if record.Phase == issueopsnextcontract.PhaseProblem || record.Phase == issueopsnextcontract.PhaseGrill ||
		record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseSHA) == "" {
		decision.Stage = stage(issueopsnextcontract.StagePrepare, 2)
		if record.Phase == issueopsnextcontract.PhaseProblem {
			decision.Missing = stageOneMissing(in)
			decision.NextCommand = "agent-harness issueops phase --id " + id + " --to grill --json"
			return decision, true
		}
		decision.Missing = []string{"branch_prepare"}
		decision.NextCommand = OwnerCommand(id, "branch_prepare")
		return decision, true
	}
	if !in.StagedPlan {
		decision.Stage = stage(issueopsnextcontract.StagePlanWrite, 3)
		decision.Missing = []string{"plan_artifact"}
		decision.NextCommand = OwnerCommand(id, "plan_artifact")
		return decision, true
	}
	if record.DesignReview == nil || !record.DesignReview.Approved {
		decision.Stage = stage(issueopsnextcontract.StagePlanDesign, 3)
		decision.Missing = []string{"design_review"}
		decision.NextCommand = OwnerCommand(id, "design_review")
		return decision, true
	}
	if devilsAdvocateMissing := devilsAdvocateGate(in); devilsAdvocateMissing != "" {
		decision.Stage = stage(issueopsnextcontract.StagePlanReview, 3)
		decision.Missing = []string{devilsAdvocateMissing}
		decision.NextCommand = devilsAdvocateCommand(in)
		return decision, true
	}
	decision.Stage = stage(issueopsnextcontract.StagePlanHandoff, 3)
	decision.NextCommand = "agent-harness issueops execution prepare --id " + id +
		" --mode auto --owner-host " + placeholder(in.ActorHost, "<host>") + " $ACTOR_FLAGS --json"
	return decision, true
}

// classifyImplementation은 홀더가 이 세션인 4~8단계를 가른다.
func classifyImplementation(in Input, decision Decision) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Execution != nil && record.Execution.Lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in) {
		switch record.Phase {
		case issueopsnextcontract.PhasePlan, issueopsnextcontract.PhaseCompatibilityReview:
			decision.Stage = stage(issueopsnextcontract.StageImplementEnter, 4)
			switch {
			case strings.TrimSpace(record.PlanPath) == "":
				decision.Missing = []string{"plan_path"}
				decision.NextCommand = OwnerCommand(id, "plan_path")
			case record.CompatibilityReview == nil || !record.CompatibilityReview.Approved:
				decision.Missing = []string{"compatibility_review"}
				decision.NextCommand = OwnerCommand(id, "compatibility_review")
			default:
				decision.NextCommand = "agent-harness issueops phase --id " + id + " --to implement $ACTOR_FLAGS --json"
			}
			return decision, true
		case issueopsnextcontract.PhaseImplement:
			decision.Stage = stage(issueopsnextcontract.StageImplement, 4)
			decision.NextCommand = "agent-harness issueops phase --id " + id + " --to ai-slop-clean $ACTOR_FLAGS --json"
			return decision, true
		}
	}
	if record.Phase == issueopsnextcontract.PhaseAISlopClean {
		if missing := intersect(in.Completion(issueopsnextcontract.PhaseAISlopClean).Missing, cleanCompletionKeys); len(missing) > 0 {
			decision.Stage = stage(issueopsnextcontract.StageClean, 5)
			decision.Missing = missing
			decision.NextCommand = OwnerCommand(id, missing[0])
			return decision, true
		}
	}
	if !postImplementPhase(record.Phase) || in.Local == nil {
		return decision, false
	}
	local := in.Local.Missing
	if missing := intersect(local, cleanLocalKeys); len(missing) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageClean, 5)
		decision.Missing = missing
		decision.NextCommand = OwnerCommand(id, missing[0])
		return decision, true
	}
	if missing := intersect(local, docsLocalKeys); len(missing) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageDocs, 6)
		decision.Missing = missing
		decision.NextCommand = OwnerCommand(id, missing[0])
		return decision, true
	}
	verify := intersect(local, verifyLocalKeys)
	verify = append(verify, prefixed(local, "gates_incomplete:")...)
	if len(verify) > 0 {
		decision.Stage = stage(issueopsnextcontract.StageVerify, 7)
		decision.Missing = verify
		decision.NextCommand = OwnerCommand(id, verify[0])
		return decision, true
	}
	if subset(local, commitPushLocalKeys) {
		decision.Stage = stage(issueopsnextcontract.StageCommitPush, 8)
		decision.Missing = append([]string{}, local...)
		decision.NextCommand = "agent-harness issueops pr-readiness --id " + id + " --strict --json"
		return decision, true
	}
	return decision, false
}

func classifyPublication(in Input, decision Decision, index int) (Decision, bool) {
	record := in.Record
	id := record.ID
	if record.Phase == issueopsnextcontract.PhasePR && record.Execution != nil {
		if record.RemoteArtifact == nil {
			decision.Stage = stage(issueopsnextcontract.StagePRCreate, 9)
			decision.Missing = []string{"remote_artifact"}
			decision.NextCommand = "agent-harness issueops remote create-pr --id " + id +
				" --expected-generation " + generationText(in) +
				" --title <TEXT> --head <BRANCH> --base <BRANCH> $ACTOR_FLAGS"
			return decision, true
		}
		if record.Execution.Completion == nil {
			decision.Stage = stage(issueopsnextcontract.StagePRComplete, 9)
			decision.NextCommand = "agent-harness issueops execution complete --id " + id +
				" --generation " + generationText(in) +
				" --final-head <SHA> --turing-report <PATH> --remote-artifact-url " + record.RemoteArtifact.URL +
				" --verification <TEXT> $ACTOR_FLAGS --confirm"
			return decision, true
		}
	}
	if record.Phase == issueopsnextcontract.PhaseDone {
		decision.Stage = stage(issueopsnextcontract.StageDone, 10)
		decision.NextCommand = "agent-harness issueops cleanup status --id " + id + " --merged --json"
		decision.Warnings = append(decision.Warnings, "merge state requires provider readback")
		return decision, true
	}
	_ = index
	return decision, false
}

func stage(key string, index int) issueopsnextcontract.Stage {
	return issueopsnextcontract.Stage{Key: key, Index: index}
}

// finish는 자리표시자가 남아 있는지로 next_command_kind를 정한다. 소유자를
// 하나로 두어 규칙마다 kind를 손으로 적는 실수를 없앤다.
func finish(in Input, decision Decision) Decision {
	_ = in
	if strings.TrimSpace(decision.NextCommand) == "" {
		return decision
	}
	if strings.Contains(decision.NextCommand, "<") || strings.Contains(decision.NextCommand, "$ACTOR_FLAGS") {
		decision.NextCommandKind = issueopsnextcontract.NextCommandKindTemplate
		return decision
	}
	decision.NextCommandKind = issueopsnextcontract.NextCommandKindExact
	return decision
}

func placeholder(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

// phaseIndex는 phase를 사용자 10단계 번호로 옮긴다. 규칙이 고정 index를 주지
// 않을 때(차단·인수·미분류) 쓴다.
func phaseIndex(phase issueopsnextcontract.Phase) int {
	switch phase {
	case issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill:
		return 1
	case issueopsnextcontract.PhasePlan:
		return 3
	case issueopsnextcontract.PhaseCompatibilityReview, issueopsnextcontract.PhaseImplement:
		return 4
	case issueopsnextcontract.PhaseAISlopClean:
		return 5
	case issueopsnextcontract.PhaseFeedback:
		return 7
	case issueopsnextcontract.PhasePR:
		return 9
	case issueopsnextcontract.PhaseDone:
		return 10
	default:
		return 0
	}
}

func phaseRank(phase issueopsnextcontract.Phase) int {
	return issueopsdomain.IssueOpsPhaseRank(phase)
}

// planningPhase는 execution이 아직 없어도 정상인 phase다. ai-slop-clean 이후에
// execution이 없으면 그것은 준비 단계가 아니라 이상 상태이므로 unknown이다.
func planningPhase(phase issueopsnextcontract.Phase) bool {
	switch phase {
	case issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill, issueopsnextcontract.PhasePlan,
		issueopsnextcontract.PhaseCompatibilityReview, issueopsnextcontract.PhaseImplement:
		return true
	default:
		return false
	}
}

func postImplementPhase(phase issueopsnextcontract.Phase) bool {
	return phase == issueopsnextcontract.PhaseAISlopClean || phase == issueopsnextcontract.PhaseFeedback
}

func holderIsSelf(in Input) bool {
	if in.Record.Execution == nil || in.Record.Execution.Lease.Holder == nil {
		return false
	}
	holder := in.Record.Execution.Lease.Holder
	return strings.EqualFold(strings.TrimSpace(holder.Host), strings.TrimSpace(in.ActorHost)) &&
		strings.TrimSpace(holder.SessionID) == strings.TrimSpace(in.ActorSessionID) &&
		strings.TrimSpace(in.ActorSessionID) != ""
}

// live는 관측하지 못한 홀더를 살아 있는 것으로 본다. 확인하지 않은 세션의
// lease를 빼앗으라고 권하지 않기 위해서다.
func live(observed *bool) bool {
	return observed == nil || *observed
}

func leaseView(in Input) issueopsnextcontract.Lease {
	if in.Record.Execution == nil {
		return issueopsnextcontract.Lease{}
	}
	lease := in.Record.Execution.Lease
	return issueopsnextcontract.Lease{
		Status:       string(lease.Status),
		Generation:   lease.Generation,
		HolderIsSelf: holderIsSelf(in),
		HolderLive:   in.HolderLive,
	}
}

func exits(in Input) issueopsnextcontract.Exits {
	id := in.Record.ID
	out := issueopsnextcontract.Exits{
		AbandonCommand: "agent-harness issueops cleanup abandon --id " + id + " --reason <TEXT> --preview",
	}
	if in.Record.Execution == nil {
		return out
	}
	lease := in.Record.Execution.Lease
	generation := generationText(in)
	if lease.Status == issueopsnextcontract.LeaseStatusActive && holderIsSelf(in) {
		out.PauseCommand = "agent-harness issueops execution release --id " + id +
			" --generation " + generation + " $ACTOR_FLAGS"
	}
	if (lease.Status == issueopsnextcontract.LeaseStatusActive && lease.Holder != nil && !holderIsSelf(in) && !live(in.HolderLive)) ||
		lease.Status == issueopsnextcontract.LeaseStatusRevoking {
		out.TakeoverCommand = "agent-harness issueops execution replace --id " + id +
			" --expected-generation " + generation + " --preview"
	}
	return out
}

func warnings(in Input) []string {
	execution := in.Record.Execution
	if execution == nil {
		return nil
	}
	var out []string
	if root := strings.TrimSpace(execution.Workspace.Root); root != "" && !in.WorktreePresent {
		out = append(out, "canonical worktree is missing at "+root)
	}
	if head := strings.TrimSpace(in.WorktreeHead); head != "" && execution.Completion != nil &&
		strings.TrimSpace(execution.Completion.FinalHead) != "" &&
		strings.TrimSpace(execution.Completion.FinalHead) != head {
		out = append(out, "worktree head "+head+" differs from the recorded completion head "+strings.TrimSpace(execution.Completion.FinalHead))
	}
	return out
}

func takeoverCommand(in Input) string {
	if command := strings.TrimSpace(in.WriterlessRecovery); command != "" {
		return command
	}
	return "agent-harness issueops execution replace --id " + in.Record.ID +
		" --expected-generation " + generationText(in) + " --preview"
}

func generationText(in Input) string {
	if in.Record.Execution == nil || in.Record.Execution.Lease.Generation == 0 {
		return "<N>"
	}
	return strconv.FormatUint(in.Record.Execution.Lease.Generation, 10)
}

// stageOneMissing은 1단계 게이트만 고정 순서로 남긴다. branch는 2단계 항목이라
// 여기서 빠진다.
func stageOneMissing(in Input) []string {
	if in.Completion == nil {
		return nil
	}
	present := map[string]bool{}
	for _, phase := range []issueopsnextcontract.Phase{issueopsnextcontract.PhaseProblem, issueopsnextcontract.PhaseGrill} {
		for _, key := range in.Completion(phase).Missing {
			present[key] = true
		}
	}
	var out []string
	for _, key := range stageOneOrder {
		if present[key] {
			out = append(out, key)
		}
	}
	return out
}

// devilsAdvocateGate는 판정 자체를 다시 계산하지 않는다. digest 비교는 adapter의
// 게이트가 이미 하므로 그 결과인 missing 키만 읽는다.
func devilsAdvocateGate(in Input) string {
	if in.Completion == nil {
		return ""
	}
	missing := in.Completion(issueopsnextcontract.PhaseImplement).Missing
	for _, key := range []string{"devils_advocate_review", "devils_advocate_review_stale"} {
		if contains(missing, key) {
			return key
		}
	}
	return ""
}

func devilsAdvocateCommand(in Input) string {
	review := in.Record.DevilsAdvocateReview
	if review != nil && strings.EqualFold(strings.TrimSpace(review.Verdict), "stop") && !review.Waived {
		return "agent-harness issueops regress --id " + in.Record.ID + " --reason <TEXT>"
	}
	return OwnerCommand(in.Record.ID, "devils_advocate_review")
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func intersect(list, keys []string) []string {
	var out []string
	for _, key := range keys {
		if contains(list, key) {
			out = append(out, key)
		}
	}
	return out
}

func prefixed(list []string, prefix string) []string {
	var out []string
	for _, item := range list {
		if strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}

func subset(list, allowed []string) bool {
	for _, item := range list {
		if !contains(allowed, item) {
			return false
		}
	}
	return true
}

func firstOr(list []string, fallback string) string {
	if len(list) > 0 {
		return list[0]
	}
	return fallback
}

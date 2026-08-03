package issueops

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/port"
)

//go:embed testdata/execution_owner_prompt.txt
var executionOwnerPromptTemplate string

var (
	executionAcceptanceID      = regexp.MustCompile(`\bAC-[0-9]{2,}\b`)
	executionPromptPlaceholder = regexp.MustCompile(`\{[A-Z][A-Z0-9_]*\}`)
	executionCommandValue      = regexp.MustCompile(`<[A-Z][A-Z0-9_-]*>`)
	executionSHA256            = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type ExecutionIssueSnapshotReadFunc func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error)

type executionOwnerIssue struct {
	URL        string `json:"url"`
	Body       string `json:"body"`
	BodySHA256 string `json:"body_sha256"`
}

type executionOwnerCommands struct {
	LeaseStatus          string `json:"lease_status"`
	Claim                string `json:"claim"`
	VerifyBranchLinkRead string `json:"verify_branch_link_read"`
	VerifyBranchLink     string `json:"verify_branch_link"`
	LinkPlan             string `json:"link_plan"`
	CompatibilityReview  string `json:"compatibility_review"`
	EnterImplement       string `json:"enter_implement"`
	AISlopCleanRecord    string `json:"ai_slop_clean_record"`
	EnterAISlopClean     string `json:"enter_ai_slop_clean"`
	RemoteCreate         string `json:"remote_create"`
	Complete             string `json:"complete"`
	ImplementationReview string `json:"implementation_review"`
	EnterPR              string `json:"enter_pr"`
}

type executionOwnerContextPacket struct {
	SchemaVersion    int                    `json:"schema_version"`
	LifecycleID      string                 `json:"lifecycle_id"`
	Mode             issueops.ExecutionMode `json:"mode"`
	SourceRoot       string                 `json:"source_root"`
	WorktreeRoot     string                 `json:"worktree_root"`
	WorktreeBase     string                 `json:"worktree_base"`
	Branch           string                 `json:"branch"`
	BaseHead         string                 `json:"base_head"`
	CurrentHead      string                 `json:"current_head"`
	LeaseGeneration  uint64                 `json:"lease_generation"`
	ClaimTokenFile   string                 `json:"claim_token_file"`
	Issue            executionOwnerIssue    `json:"issue"`
	OwnerHost        string                 `json:"owner_host"`
	OwnerModel       string                 `json:"owner_model"`
	OwnerEffort      string                 `json:"owner_effort,omitempty"`
	ReviewerModel    string                 `json:"reviewer_model,omitempty"`
	ReviewerEffort   string                 `json:"reviewer_effort,omitempty"`
	RequiredDocs     []string               `json:"required_docs"`
	RequiredSkills   []string               `json:"required_skills"`
	AcceptanceIDs    []string               `json:"acceptance_ids"`
	Verification     []string               `json:"verification_commands"`
	TuringReportPath string                 `json:"turing_report_path"`
	ArtifactManifest map[string]string      `json:"artifact_manifest,omitempty"`
	Commands         executionOwnerCommands `json:"commands"`
}

type executionOwnerSnapshot struct {
	issue                executionOwnerIssue
	requiredDocs         []string
	requiredSkills       []string
	acceptanceIDs        []string
	verificationCommands []string
}

type executionOwnerArtifacts struct {
	packetPath   string
	packetSHA256 string
	promptPath   string
	promptSHA256 string
	prompt       string
}

func readExecutionOwnerArtifact(root, path string) ([]byte, error) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("owner artifact must be inside the canonical worktree")
	}
	current := root
	parts := strings.Split(rel, string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("owner artifact path contains a missing entry or symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("owner artifact ancestor is not a directory")
		}
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > leasecontract.OwnerArtifactMaxBytes) {
			return nil, fmt.Errorf("owner artifact must be a private bounded regular file")
		}
	}
	return os.ReadFile(path)
}

func readExecutionOwnerSnapshot(ctx context.Context, record issueops.IssueOpsRecord, read ExecutionIssueSnapshotReadFunc) (executionOwnerSnapshot, error) {
	if read == nil {
		return executionOwnerSnapshot{}, fmt.Errorf("remote issue snapshot reader is unavailable")
	}
	provider := executionOwnerIssueProvider(record)
	if provider == "" || strings.TrimSpace(record.IssueURL) == "" {
		return executionOwnerSnapshot{}, fmt.Errorf("linked GitHub or GitLab issue is required before owner dispatch")
	}
	snapshot, err := read(ctx, provider, port.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return executionOwnerSnapshot{}, fmt.Errorf("read remote issue snapshot: %w", err)
	}
	if strings.TrimSpace(snapshot.URL) != strings.TrimSpace(record.IssueURL) || strings.TrimSpace(snapshot.Body) == "" || len(snapshot.Body) > leasecontract.OwnerArtifactMaxBytes/2 {
		return executionOwnerSnapshot{}, fmt.Errorf("remote issue snapshot identity or bounded body is invalid")
	}
	acceptance := uniqueExecutionOwnerValues(executionAcceptanceID.FindAllString(snapshot.Body, -1))
	verification := extractExecutionOwnerVerification(snapshot.Body)
	if len(acceptance) == 0 || len(verification) == 0 {
		return executionOwnerSnapshot{}, fmt.Errorf("remote issue must contain acceptance IDs and an exact verification command block")
	}
	return executionOwnerSnapshot{
		issue:                executionOwnerIssue{URL: strings.TrimSpace(snapshot.URL), Body: snapshot.Body, BodySHA256: digestExecutionOwnerBytes([]byte(snapshot.Body))},
		requiredDocs:         executionOwnerRequiredDocs(record.Repo),
		requiredSkills:       []string{"issueops", "turing", "atomic-commit-push"},
		acceptanceIDs:        acceptance,
		verificationCommands: verification,
	}, nil
}

func executionOwnerIssueProvider(record issueops.IssueOpsRecord) string {
	if record.BranchPrepare != nil {
		provider := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
		if provider == "github" || provider == "gitlab" {
			return provider
		}
	}
	if strings.Contains(record.IssueURL, "github") {
		return "github"
	}
	if strings.Contains(record.IssueURL, "gitlab") {
		return "gitlab"
	}
	return ""
}

func buildExecutionOwnerArtifacts(record issueops.IssueOpsRecord, req ExecutionPrepareRequest, snapshot executionOwnerSnapshot, artifactManifest map[string]string) (executionOwnerArtifacts, error) {
	if record.Execution == nil || record.Execution.Lease.Generation == 0 {
		return executionOwnerArtifacts{}, fmt.Errorf("execution identity is unavailable for owner packet")
	}
	packetPath, promptPath := executionOwnerArtifactPaths(record)
	// 구현 diff의 brooks 리뷰는 planner급 모델이 수행한다(설계 v5 WS5). 값은
	// 감사 기록이자 owner 프롬프트 지시일 뿐 게이트 조건이 아니다.
	reviewerModel, reviewerEffort, _ := port.IssueOpsPlannerDefaults(strings.ToLower(strings.TrimSpace(req.OwnerHost)))
	commands := executionOwnerCommandsFor(record, req, snapshot.issue.BodySHA256)
	packet := executionOwnerContextPacket{
		SchemaVersion: issueops.IssueOpsSchemaVersion, LifecycleID: record.ID, Mode: record.Execution.Mode,
		SourceRoot: record.Execution.Workspace.SourceRoot, WorktreeRoot: record.Execution.Workspace.Root,
		WorktreeBase: filepath.Dir(record.Execution.Workspace.Root), Branch: record.Execution.Workspace.Branch,
		BaseHead: record.Execution.Workspace.BaseHead, CurrentHead: record.Execution.Workspace.BaseHead,
		LeaseGeneration: record.Execution.Lease.Generation, ClaimTokenFile: claimTokenPath(record), Issue: snapshot.issue,
		OwnerHost: strings.ToLower(strings.TrimSpace(req.OwnerHost)), OwnerModel: strings.TrimSpace(req.OwnerModel), OwnerEffort: strings.TrimSpace(req.OwnerEffort),
		ReviewerModel: reviewerModel, ReviewerEffort: reviewerEffort,
		RequiredDocs: snapshot.requiredDocs, RequiredSkills: snapshot.requiredSkills, AcceptanceIDs: snapshot.acceptanceIDs,
		Verification: snapshot.verificationCommands, TuringReportPath: executionOwnerTuringReportPath(record), Commands: commands,
		ArtifactManifest: artifactManifest,
	}
	packetBytes, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return executionOwnerArtifacts{}, err
	}
	packetBytes = append(packetBytes, '\n')
	if len(packetBytes) > leasecontract.OwnerArtifactMaxBytes {
		return executionOwnerArtifacts{}, fmt.Errorf("owner context packet exceeds %d bytes", leasecontract.OwnerArtifactMaxBytes)
	}
	packetDigest := digestExecutionOwnerBytes(packetBytes)
	prompt, err := renderExecutionOwnerPrompt(packet, packetPath, packetDigest)
	if err != nil {
		return executionOwnerArtifacts{}, err
	}
	if err := validateExecutionOwnerCatalog(commands); err != nil {
		return executionOwnerArtifacts{}, err
	}
	if err := writeExecutionOwnerArtifact(record.Execution.Workspace.Root, packetPath, packetBytes); err != nil {
		return executionOwnerArtifacts{}, err
	}
	if err := writeExecutionOwnerArtifact(record.Execution.Workspace.Root, promptPath, []byte(prompt)); err != nil {
		return executionOwnerArtifacts{}, err
	}
	return executionOwnerArtifacts{
		packetPath: packetPath, packetSHA256: packetDigest, promptPath: promptPath,
		promptSHA256: digestExecutionOwnerBytes([]byte(prompt)), prompt: prompt,
	}, nil
}

func renderExecutionOwnerPrompt(packet executionOwnerContextPacket, packetPath, packetDigest string) (string, error) {
	if err := validateExecutionOwnerPromptInputs(packet, packetPath, packetDigest); err != nil {
		return "", err
	}
	values := map[string]string{
		"LIFECYCLE_ID": packet.LifecycleID, "MODE": string(packet.Mode), "SCHEMA_VERSION": strconv.Itoa(packet.SchemaVersion),
		"SOURCE_ROOT": packet.SourceRoot, "WORKTREE_ROOT": packet.WorktreeRoot, "WORKTREE_BASE": packet.WorktreeBase,
		"BRANCH": packet.Branch, "BASE_HEAD": packet.BaseHead, "LEASE_GENERATION": strconv.FormatUint(packet.LeaseGeneration, 10),
		"LEASE_STATUS_COMMAND": packet.Commands.LeaseStatus, "CLAIM_COMMAND": strings.ReplaceAll(packet.Commands.Claim, "<PACKET_SHA256>", packetDigest),
		"ISSUE_URL": packet.Issue.URL, "ISSUE_BODY_SHA256": packet.Issue.BodySHA256,
		"PACKET_PATH": packetPath, "PACKET_SHA256": packetDigest,
		"OWNER_HOST": packet.OwnerHost, "OWNER_MODEL": packet.OwnerModel, "OWNER_EFFORT": packet.OwnerEffort,
		"REVIEWER_MODEL": packet.ReviewerModel, "REVIEWER_EFFORT": packet.ReviewerEffort,
		"VERIFY_BRANCH_LINK_READ_COMMAND": packet.Commands.VerifyBranchLinkRead,
		"VERIFY_BRANCH_LINK_COMMAND":      packet.Commands.VerifyBranchLink,
		"LINK_PLAN_COMMAND":               packet.Commands.LinkPlan,
		"COMPATIBILITY_REVIEW_COMMAND":    packet.Commands.CompatibilityReview,
		"ENTER_IMPLEMENT_COMMAND":         packet.Commands.EnterImplement,
		"AI_SLOP_CLEAN_RECORD_COMMAND":    packet.Commands.AISlopCleanRecord,
		"ENTER_AI_SLOP_CLEAN_COMMAND":     packet.Commands.EnterAISlopClean,
		"IMPLEMENTATION_REVIEW_COMMAND":   packet.Commands.ImplementationReview,
		"ENTER_PR_COMMAND":                packet.Commands.EnterPR,
		"REQUIRED_DOCS":                   renderExecutionOwnerLines(packet.RequiredDocs), "REQUIRED_SKILLS": renderExecutionOwnerLines(packet.RequiredSkills),
		"ACCEPTANCE_IDS": strings.Join(packet.AcceptanceIDs, ", "), "VERIFICATION_COMMANDS": renderExecutionOwnerLines(packet.Verification),
		"TURING_REPORT_PATH": packet.TuringReportPath, "REMOTE_CREATE_COMMAND": packet.Commands.RemoteCreate, "COMPLETE_COMMAND": packet.Commands.Complete,
	}
	missing := ""
	prompt := executionPromptPlaceholder.ReplaceAllStringFunc(executionOwnerPromptTemplate, func(token string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(token, "{"), "}")
		value, ok := values[key]
		if !ok && missing == "" {
			missing = token
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("owner prompt placeholder %s has no renderer", missing)
	}
	if unresolved := executionPromptPlaceholder.FindString(prompt); unresolved != "" {
		return "", fmt.Errorf("owner prompt value introduced unresolved placeholder %s", unresolved)
	}
	if len(prompt) > leasecontract.OwnerArtifactMaxBytes {
		return "", fmt.Errorf("owner prompt exceeds %d bytes", leasecontract.OwnerArtifactMaxBytes)
	}
	return prompt, nil
}

func validateExecutionOwnerPromptInputs(packet executionOwnerContextPacket, packetPath, packetDigest string) error {
	scalars := []struct{ name, value string }{
		{"lifecycle_id", packet.LifecycleID}, {"mode", string(packet.Mode)}, {"source_root", packet.SourceRoot},
		{"worktree_root", packet.WorktreeRoot}, {"worktree_base", packet.WorktreeBase}, {"branch", packet.Branch},
		{"base_head", packet.BaseHead}, {"issue_url", packet.Issue.URL}, {"issue_body_sha256", packet.Issue.BodySHA256},
		{"packet_path", packetPath}, {"packet_sha256", packetDigest}, {"owner_host", packet.OwnerHost},
		{"owner_model", packet.OwnerModel}, {"owner_effort", packet.OwnerEffort}, {"turing_report_path", packet.TuringReportPath},
		{"lease_status_command", packet.Commands.LeaseStatus}, {"claim_command", packet.Commands.Claim},
		{"verify_branch_link_read_command", packet.Commands.VerifyBranchLinkRead},
		{"verify_branch_link_command", packet.Commands.VerifyBranchLink},
		{"link_plan_command", packet.Commands.LinkPlan}, {"compatibility_review_command", packet.Commands.CompatibilityReview},
		{"enter_implement_command", packet.Commands.EnterImplement},
		{"ai_slop_clean_record_command", packet.Commands.AISlopCleanRecord}, {"enter_ai_slop_clean_command", packet.Commands.EnterAISlopClean},
		{"remote_create_command", packet.Commands.RemoteCreate}, {"complete_command", packet.Commands.Complete},
		{"reviewer_model", packet.ReviewerModel}, {"reviewer_effort", packet.ReviewerEffort},
		{"implementation_review_command", packet.Commands.ImplementationReview}, {"enter_pr_command", packet.Commands.EnterPR},
	}
	for _, scalar := range scalars {
		if strings.ContainsAny(scalar.value, "\r\n") || executionPromptPlaceholder.MatchString(scalar.value) {
			return fmt.Errorf("owner prompt %s contains a line break or placeholder token", scalar.name)
		}
	}
	lists := []struct {
		name   string
		values []string
	}{
		{"required_docs", packet.RequiredDocs}, {"required_skills", packet.RequiredSkills},
		{"acceptance_ids", packet.AcceptanceIDs}, {"verification_commands", packet.Verification},
	}
	for _, list := range lists {
		for _, value := range list.values {
			if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") || executionPromptPlaceholder.MatchString(value) {
				return fmt.Errorf("owner prompt %s contains an empty, multiline, or placeholder value", list.name)
			}
		}
	}
	return nil
}

func executionOwnerCommandsFor(record issueops.IssueOpsRecord, req ExecutionPrepareRequest, issueBodySHA256 string) executionOwnerCommands {
	generation := record.Execution.Lease.Generation
	actorFlags := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>",
		"--session-pid", "<SESSION_PID>", "--session-started-at", "<SESSION_STARTED_AT>", "--session-executable", "<SESSION_EXECUTABLE>",
		"--cwd", quoteExecutionOwnerArg(record.Execution.Workspace.Root),
	}, " ")
	status := "agent-harness issueops execution status --id " + quoteExecutionOwnerArg(record.ID) + " --json"
	claim := "none"
	if record.Execution.Mode == issueops.ExecutionModeOrca {
		claim = "agent-harness issueops execution claim --id " + quoteExecutionOwnerArg(record.ID) +
			" --generation " + strconv.FormatUint(generation, 10) + " --claim-token-file " + quoteExecutionOwnerArg(claimTokenPath(record)) +
			" --issue-body-sha256 " + strings.TrimSpace(issueBodySHA256) + " --context-packet-sha256 <PACKET_SHA256> " + actorFlags + " --json"
	}
	shortActor := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>",
		"--cwd", quoteExecutionOwnerArg(record.Execution.Workspace.Root),
	}, " ")
	verifyBranchLinkRead := "none"
	verifyBranchLink := "none"
	if prepared := record.BranchPrepare; prepared != nil && !prepared.LinkVerified {
		if strings.EqualFold(strings.TrimSpace(prepared.Provider), "github") {
			projectKey := remote.ProjectKey(prepared.IssueURL, "github", "issue")
			issueNumber := remote.IssueNumber(prepared.IssueURL)
			repoSlug := strings.TrimPrefix(projectKey, "github.com/")
			if projectKey != "" && repoSlug != projectKey && issueNumber != "" {
				verifyBranchLinkRead = "gh issue develop --list " + issueNumber +
					" --repo " + quoteExecutionOwnerArg(repoSlug)
			}
		}
		verifyBranchLink = "agent-harness issueops branch prepare --id " + quoteExecutionOwnerArg(record.ID) +
			" --provider " + quoteExecutionOwnerArg(strings.ToLower(strings.TrimSpace(prepared.Provider))) +
			" --issue-url " + quoteExecutionOwnerArg(strings.TrimSpace(prepared.IssueURL)) +
			" --branch " + quoteExecutionOwnerArg(strings.TrimSpace(prepared.Branch)) +
			" --base-branch " + quoteExecutionOwnerArg(strings.TrimSpace(prepared.BaseBranch))
		for _, optional := range []struct{ flag, value string }{
			{"--base-sha", prepared.BaseSHA},
			{"--parent-worktree", prepared.ParentWorktree},
			{"--remote-branch-url", prepared.RemoteBranchURL},
		} {
			if value := strings.TrimSpace(optional.value); value != "" {
				verifyBranchLink += " " + optional.flag + " " + quoteExecutionOwnerArg(value)
			}
		}
		verifyBranchLink += " --link-verified " + shortActor + " --json"
	}
	planPath := filepath.Join(record.Execution.Workspace.Root, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
	linkPlan := "none"
	if strings.TrimSpace(record.PlanPath) == "" {
		linkPlan = "agent-harness issueops link-plan --id " + quoteExecutionOwnerArg(record.ID) +
			" --plan-path " + quoteExecutionOwnerArg(planPath) + " " + shortActor + " --json"
	}
	compatibilityReview := "agent-harness issueops compatibility review --id " + quoteExecutionOwnerArg(record.ID) +
		" --backward-compatibility " + quoteExecutionOwnerArg("<BACKWARD_COMPATIBILITY>") +
		" --side-effect " + quoteExecutionOwnerArg("<SIDE_EFFECT>") +
		" --rollback-plan " + quoteExecutionOwnerArg("<ROLLBACK_PLAN>") +
		" --verification " + quoteExecutionOwnerArg("<COMPATIBILITY_VERIFICATION>") + " --approved " +
		shortActor + " --json"
	enterImplement := "agent-harness issueops phase --id " + quoteExecutionOwnerArg(record.ID) +
		" --to implement " + shortActor + " --json"
	aiSlopCleanRecord := "agent-harness issueops ai-slop-clean record --id " + quoteExecutionOwnerArg(record.ID) +
		" --category <CLEANUP_CATEGORY> --verification <VERIFICATION_EVIDENCE> " + shortActor + " --json"
	enterAISlopClean := "agent-harness issueops phase --id " + quoteExecutionOwnerArg(record.ID) +
		" --to ai-slop-clean " + shortActor + " --json"
	base := ""
	if record.BranchPrepare != nil {
		base = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
	remote := "agent-harness issueops remote create-pr --id " + quoteExecutionOwnerArg(record.ID) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --title <PR_TITLE> --body-file <PR_BODY_FILE>" +
		" --head " + quoteExecutionOwnerArg(record.Execution.Workspace.Branch) + " --base " + quoteExecutionOwnerArg(base) +
		" --label <LABEL> --assignee <ASSIGNEE> " + actorFlags + " --confirm --json"
	complete := "agent-harness issueops execution complete --id " + quoteExecutionOwnerArg(record.ID) +
		" --generation " + strconv.FormatUint(generation, 10) + " --final-head <FINAL_HEAD> --turing-report " + quoteExecutionOwnerArg(executionOwnerTuringReportPath(record)) +
		" --remote-artifact-url <DRAFT_PR_OR_MR_URL> --verification <VERIFICATION_EVIDENCE> " + actorFlags + " --confirm --json"
	plannerModel, plannerEffort, _ := port.IssueOpsPlannerDefaults(strings.ToLower(strings.TrimSpace(req.OwnerHost)))
	implementationReview := "agent-harness issueops implementation-review record --id " + quoteExecutionOwnerArg(record.ID) +
		" --verdict <VERDICT> --finding <FINDING> --evidence <EVIDENCE> --reviewer-host " + strings.ToLower(strings.TrimSpace(req.OwnerHost)) +
		" --reviewer-model " + quoteExecutionOwnerArg(plannerModel)
	if strings.TrimSpace(plannerEffort) != "" {
		implementationReview += " --reviewer-effort " + quoteExecutionOwnerArg(plannerEffort)
	}
	implementationReview += " " + shortActor + " --json"
	enterPR := "agent-harness issueops phase --id " + quoteExecutionOwnerArg(record.ID) +
		" --to pr " + shortActor + " --json"
	return executionOwnerCommands{
		LeaseStatus: status, Claim: claim, VerifyBranchLinkRead: verifyBranchLinkRead,
		VerifyBranchLink: verifyBranchLink, LinkPlan: linkPlan,
		CompatibilityReview: compatibilityReview, EnterImplement: enterImplement,
		AISlopCleanRecord: aiSlopCleanRecord, EnterAISlopClean: enterAISlopClean,
		ImplementationReview: implementationReview, EnterPR: enterPR,
		RemoteCreate: remote, Complete: complete,
	}
}

func validateExecutionOwnerCatalog(commands executionOwnerCommands) error {
	for _, path := range []string{
		"execution status", "execution claim", "branch prepare", "link-plan", "compatibility review", "phase", "ai-slop-clean record",
		"implementation-review record", "remote create-pr", "execution complete",
	} {
		if _, _, _, ok := commandparse.IssueOpsCommandSpec(path); !ok {
			return fmt.Errorf("IssueOps v1 command catalog is not ready: missing %s", path)
		}
	}
	for _, path := range []string{"worktree prepare", "handoff start", "handoff claim", "handoff acknowledge"} {
		if _, _, _, ok := commandparse.IssueOpsCommandSpec(path); ok {
			return fmt.Errorf("IssueOps v1 command catalog still exposes legacy %s", path)
		}
	}
	checks := []struct{ command, path string }{
		{commands.LeaseStatus, "execution status"}, {commands.Claim, "execution claim"},
		{commands.VerifyBranchLink, "branch prepare"},
		{commands.LinkPlan, "link-plan"}, {commands.CompatibilityReview, "compatibility review"},
		{commands.EnterImplement, "phase"},
		{commands.AISlopCleanRecord, "ai-slop-clean record"}, {commands.EnterAISlopClean, "phase"},
		{commands.ImplementationReview, "implementation-review record"}, {commands.EnterPR, "phase"},
		{commands.RemoteCreate, "remote create-pr"}, {commands.Complete, "execution complete"},
	}
	for _, check := range checks {
		if check.command == "none" {
			continue
		}
		command := executionCommandValue.ReplaceAllString(check.command, "VALUE")
		parsed, ok := commandparse.ParseExactIssueOpsCommand(command)
		if !ok || parsed.Path != check.path {
			return fmt.Errorf("IssueOps v1 owner command does not match catalog path %s", check.path)
		}
		values, booleans, repeatable, _ := commandparse.IssueOpsCommandSpec(check.path)
		if _, ok := commandparse.ExactFlags(parsed, values, booleans, repeatable); !ok {
			return fmt.Errorf("IssueOps v1 owner command flags do not match catalog path %s", check.path)
		}
	}
	return nil
}

// SealedOwnerContextPacketPath는 현재 세대 봉인 packet의 경로를 돌려준다.
// 훅 가드가 봉인 실존을 확인하는 유일한 경로다 — 경로 규칙을 다른 계층에
// 복제하지 않기 위해 노출한다. execution이 없으면 빈 문자열이다.
func SealedOwnerContextPacketPath(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	packetPath, _ := executionOwnerArtifactPaths(record)
	return packetPath
}

func executionOwnerArtifactPaths(record issueops.IssueOpsRecord) (string, string) {
	key := digestExecutionOwnerBytes([]byte(record.ID))[:16]
	base := filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, "generation-"+strconv.FormatUint(record.Execution.Lease.Generation, 10))
	return filepath.Join(base, "context.json"), filepath.Join(base, "owner-prompt.txt")
}

func executionOwnerTuringReportPath(record issueops.IssueOpsRecord) string {
	key := digestExecutionOwnerBytes([]byte(record.ID))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "turing", "issueops-v1-"+key+".json")
}

func writeExecutionOwnerArtifact(root, path string, value []byte) error {
	if len(value) == 0 || len(value) > leasecontract.OwnerArtifactMaxBytes {
		return fmt.Errorf("owner artifact is empty or oversized")
	}
	if err := ensureExecutionOwnerArtifactDirectory(root, filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		current, readErr := os.ReadFile(path)
		info, statErr := os.Lstat(path)
		if readErr != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !strings.EqualFold(digestExecutionOwnerBytes(current), digestExecutionOwnerBytes(value)) {
			return fmt.Errorf("immutable owner artifact already exists with different identity")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = file.Write(value); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func ensureExecutionOwnerArtifactDirectory(root, target string) error {
	root, target = filepath.Clean(root), filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("owner artifact directory must be inside the canonical worktree")
	}
	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
				return mkdirErr
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("owner artifact path must contain only real directories")
		}
	}
	return nil
}

func executionOwnerRequiredDocs(root string) []string {
	candidates := []string{
		"AGENTS.md", ".agent-harness/CONSTITUTION.md", ".agent-harness/ARCHITECTURE.md", ".agent-harness/CONVENTIONS.md",
		".agent-harness/TESTING.md", ".agent-harness/CAUTIONS.md", ".agent-harness/TECH_STACK.md", ".agent-harness/ADR.md",
		".agent-harness/OPERATIONS.md", ".agent-harness/AGENT_WORKFLOW.md",
	}
	out := make([]string, 0, len(candidates))
	for _, rel := range candidates {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			out = append(out, rel)
		}
	}
	return out
}

func extractExecutionOwnerVerification(body string) []string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	inSection, inFence := false, false
	commands := []string{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## ") {
			heading := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			inSection = heading == "검증" || heading == "검증 명령" ||
				heading == "verification" || heading == "verification commands"
			inFence = false
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "```") {
			if inFence {
				break
			}
			inFence = true
			continue
		}
		if inFence && line != "" && !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}
	return uniqueExecutionOwnerValues(commands)
}

func uniqueExecutionOwnerValues(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func renderExecutionOwnerLines(values []string) string {
	if len(values) == 0 {
		return "- none"
	}
	rows := append([]string(nil), values...)
	for index := range rows {
		rows[index] = "- " + rows[index]
	}
	return strings.Join(rows, "\n")
}

func quoteExecutionOwnerArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func digestExecutionOwnerBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

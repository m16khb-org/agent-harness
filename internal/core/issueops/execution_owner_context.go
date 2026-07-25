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

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

const executionOwnerArtifactLimit = 1 << 20

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
	RemoteCreate         string `json:"remote_create"`
	Complete             string `json:"complete"`
	ImplementationReview string `json:"implementation_review"`
}

type executionOwnerContextPacket struct {
	SchemaVersion    int                    `json:"schema_version"`
	LifecycleID      string                 `json:"lifecycle_id"`
	Mode             model.ExecutionMode    `json:"mode"`
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

// validateExecutionClaimContext는 orca claim의 봉인 검증을 준비한다.
//
// 이 검증은 원래 generation 1에서만 동작했다. reseed가 packet을 재봉인하지 않던
// 시절에는 그 제약이 generation 2 이상 claim을 통과시키는 구멍이었다 — 봉인의
// 목적(owner가 읽는 스코프가 표류·조작되지 않았음을 보증)이 두 번째 세대부터
// 사라졌다. reseed가 재봉인을 수행하게 된 뒤로는 모든 세대가 자기 세대의 봉인을
// 검증할 수 있다.
func validateExecutionClaimContext(ctx context.Context, stateRoot string, req ExecutionClaimRequest, deps ExecutionClaimDependencies) (func(IssueOpsRecord) error, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return nil, err
	}
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca ||
		req.Generation == 0 || req.Generation != record.Execution.Lease.Generation {
		return nil, nil
	}
	issueDigest := strings.ToLower(strings.TrimSpace(req.IssueBodySHA256))
	packetDigest := strings.ToLower(strings.TrimSpace(req.ContextPacketSHA256))
	if !executionSHA256.MatchString(issueDigest) || !executionSHA256.MatchString(packetDigest) {
		return nil, fmt.Errorf("Orca claim requires sealed issue and context packet digests")
	}
	if err := validateExecutionClaimPacket(record, issueDigest, packetDigest); err != nil {
		return nil, err
	}
	if deps.ReadIssue == nil {
		return nil, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
	}
	snapshot, err := deps.ReadIssue(ctx, executionOwnerIssueProvider(record), port.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return nil, fmt.Errorf("read remote issue before claim: %w", err)
	}
	if url := strings.TrimSpace(snapshot.URL); url != strings.TrimSpace(record.IssueURL) {
		return nil, fmt.Errorf("remote issue snapshot url does not match the linked issue: observed=%s expected=%s", url, strings.TrimSpace(record.IssueURL))
	}
	// digest는 단방향 해시이고 대상은 공개 이슈다. 두 값을 병기하지 않으면 owner는
	// 무엇이 어떻게 달라졌는지 알 수 없고, 세션에서 해시를 직접 계산할 경로도
	// 없다. 병기가 곧 진단 표면이다.
	if observed := digestExecutionOwnerBytes([]byte(snapshot.Body)); observed != issueDigest {
		return nil, fmt.Errorf("remote issue body digest drifted from the sealed owner context: expected=%s observed=%s; reseal with `agent-harness issueops execution replace --reseed` after confirming the revision is intended", issueDigest, observed)
	}
	return func(current IssueOpsRecord) error {
		return validateExecutionClaimPacket(current, issueDigest, packetDigest)
	}, nil
}

func validateExecutionClaimPacket(record IssueOpsRecord, issueDigest, packetDigest string) error {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Lease.Generation == 0 {
		return fmt.Errorf("sealed owner context no longer matches an Orca execution generation")
	}
	generation := record.Execution.Lease.Generation
	packetPath, _ := executionOwnerArtifactPaths(record)
	data, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if observed := digestExecutionOwnerBytes(data); observed != packetDigest {
		return fmt.Errorf("sealed context packet digest mismatch: expected=%s observed=%s path=%s", packetDigest, observed, packetPath)
	}
	var packet executionOwnerContextPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	if packet.SchemaVersion != model.IssueOpsSchemaVersion || packet.LifecycleID != record.ID || packet.Mode != record.Execution.Mode ||
		!samePath(packet.SourceRoot, record.Execution.Workspace.SourceRoot) || !samePath(packet.WorktreeRoot, record.Execution.Workspace.Root) ||
		packet.Branch != record.Execution.Workspace.Branch || packet.BaseHead != record.Execution.Workspace.BaseHead ||
		packet.LeaseGeneration != generation ||
		packet.ClaimTokenFile != claimTokenPath(record) || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch: packet_generation=%d expected_generation=%d", packet.LeaseGeneration, generation)
	}
	if packet.Issue.BodySHA256 != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch: expected=%s observed=%s", issueDigest, packet.Issue.BodySHA256)
	}
	if observed := digestExecutionOwnerBytes([]byte(packet.Issue.Body)); observed != issueDigest {
		return fmt.Errorf("sealed context packet issue body does not hash to its sealed digest: expected=%s observed=%s", issueDigest, observed)
	}
	// artifact manifest 검증: materialize된 파일이 봉인 당시와 달라졌으면
	// drift로 read-only 잔류시킨다(설계 v5 WS2).
	for name, digest := range packet.ArtifactManifest {
		path := filepath.Join(record.Execution.Workspace.Root, filepath.FromSlash(IssueOpsArtifactDir), name+".md")
		data, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, path)
		if err != nil {
			return fmt.Errorf("read sealed artifact %s: %w", name, err)
		}
		if digestExecutionOwnerBytes(data) != digest {
			return fmt.Errorf("sealed artifact %s digest mismatch", name)
		}
	}
	return nil
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
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > executionOwnerArtifactLimit) {
			return nil, fmt.Errorf("owner artifact must be a private bounded regular file")
		}
	}
	return os.ReadFile(path)
}

func readExecutionOwnerSnapshot(ctx context.Context, record IssueOpsRecord, read ExecutionIssueSnapshotReadFunc) (executionOwnerSnapshot, error) {
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
	if strings.TrimSpace(snapshot.URL) != strings.TrimSpace(record.IssueURL) || strings.TrimSpace(snapshot.Body) == "" || len(snapshot.Body) > executionOwnerArtifactLimit/2 {
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
		requiredSkills:       []string{"issueops", "turing"},
		acceptanceIDs:        acceptance,
		verificationCommands: verification,
	}, nil
}

func executionOwnerIssueProvider(record IssueOpsRecord) string {
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

func buildExecutionOwnerArtifacts(record IssueOpsRecord, req ExecutionPrepareRequest, snapshot executionOwnerSnapshot, artifactManifest map[string]string) (executionOwnerArtifacts, error) {
	if record.Execution == nil || record.Execution.Lease.Generation == 0 {
		return executionOwnerArtifacts{}, fmt.Errorf("execution identity is unavailable for owner packet")
	}
	packetPath, promptPath := executionOwnerArtifactPaths(record)
	// 구현 diff의 brooks 리뷰는 planner급 모델이 수행한다(설계 v5 WS5). 값은
	// 감사 기록이자 owner 프롬프트 지시일 뿐 게이트 조건이 아니다.
	reviewerModel, reviewerEffort, _ := port.IssueOpsPlannerDefaults(strings.ToLower(strings.TrimSpace(req.OwnerHost)))
	commands := executionOwnerCommandsFor(record, req, snapshot.issue.BodySHA256)
	packet := executionOwnerContextPacket{
		SchemaVersion: model.IssueOpsSchemaVersion, LifecycleID: record.ID, Mode: record.Execution.Mode,
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
	if len(packetBytes) > executionOwnerArtifactLimit {
		return executionOwnerArtifacts{}, fmt.Errorf("owner context packet exceeds %d bytes", executionOwnerArtifactLimit)
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
		"IMPLEMENTATION_REVIEW_COMMAND": packet.Commands.ImplementationReview,
		"REQUIRED_DOCS":                 renderExecutionOwnerLines(packet.RequiredDocs), "REQUIRED_SKILLS": renderExecutionOwnerLines(packet.RequiredSkills),
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
	if len(prompt) > executionOwnerArtifactLimit {
		return "", fmt.Errorf("owner prompt exceeds %d bytes", executionOwnerArtifactLimit)
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
		{"remote_create_command", packet.Commands.RemoteCreate}, {"complete_command", packet.Commands.Complete},
		{"reviewer_model", packet.ReviewerModel}, {"reviewer_effort", packet.ReviewerEffort},
		{"implementation_review_command", packet.Commands.ImplementationReview},
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

func executionOwnerCommandsFor(record IssueOpsRecord, req ExecutionPrepareRequest, issueBodySHA256 string) executionOwnerCommands {
	generation := record.Execution.Lease.Generation
	actorFlags := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>",
		"--session-pid", "<SESSION_PID>", "--session-started-at", "<SESSION_STARTED_AT>", "--session-executable", "<SESSION_EXECUTABLE>",
		"--cwd", quoteExecutionOwnerArg(record.Execution.Workspace.Root),
	}, " ")
	status := "agent-harness issueops execution status --id " + quoteExecutionOwnerArg(record.ID) + " --json"
	claim := "none"
	if record.Execution.Mode == model.ExecutionModeOrca {
		claim = "agent-harness issueops execution claim --id " + quoteExecutionOwnerArg(record.ID) +
			" --generation " + strconv.FormatUint(generation, 10) + " --claim-token-file " + quoteExecutionOwnerArg(claimTokenPath(record)) +
			" --issue-body-sha256 " + strings.TrimSpace(issueBodySHA256) + " --context-packet-sha256 <PACKET_SHA256> " + actorFlags + " --json"
	}
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
	shortActor := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>",
		"--cwd", quoteExecutionOwnerArg(record.Execution.Workspace.Root),
	}, " ")
	plannerModel, plannerEffort, _ := port.IssueOpsPlannerDefaults(strings.ToLower(strings.TrimSpace(req.OwnerHost)))
	implementationReview := "agent-harness issueops implementation-review record --id " + quoteExecutionOwnerArg(record.ID) +
		" --verdict pass --finding <FINDING> --evidence <EVIDENCE> --reviewer-host " + strings.ToLower(strings.TrimSpace(req.OwnerHost)) +
		" --reviewer-model " + quoteExecutionOwnerArg(plannerModel)
	if strings.TrimSpace(plannerEffort) != "" {
		implementationReview += " --reviewer-effort " + quoteExecutionOwnerArg(plannerEffort)
	}
	implementationReview += " " + shortActor + " --json"
	return executionOwnerCommands{LeaseStatus: status, Claim: claim, RemoteCreate: remote, Complete: complete, ImplementationReview: implementationReview}
}

func validateExecutionOwnerCatalog(commands executionOwnerCommands) error {
	for _, path := range []string{"execution status", "execution claim", "execution complete", "remote create-pr", "implementation-review record"} {
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
		{commands.RemoteCreate, "remote create-pr"}, {commands.Complete, "execution complete"},
		{commands.ImplementationReview, "implementation-review record"},
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

func executionOwnerArtifactPaths(record IssueOpsRecord) (string, string) {
	key := digestExecutionOwnerBytes([]byte(record.ID))[:16]
	base := filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, "generation-"+strconv.FormatUint(record.Execution.Lease.Generation, 10))
	return filepath.Join(base, "context.json"), filepath.Join(base, "owner-prompt.txt")
}

func executionOwnerTuringReportPath(record IssueOpsRecord) string {
	key := digestExecutionOwnerBytes([]byte(record.ID))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "turing", "issueops-v1-"+key+".json")
}

func writeExecutionOwnerArtifact(root, path string, value []byte) error {
	if len(value) == 0 || len(value) > executionOwnerArtifactLimit {
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
			inSection = heading == "검증 명령" || heading == "verification commands"
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

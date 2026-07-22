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

const executionV1OwnerArtifactLimit = 1 << 20

//go:embed testdata/execution_v1_owner_prompt.txt
var executionV1OwnerPromptTemplate string

var (
	executionV1AcceptanceID      = regexp.MustCompile(`\bAC-[0-9]{2,}\b`)
	executionV1PromptPlaceholder = regexp.MustCompile(`\{[A-Z][A-Z0-9_]*\}`)
	executionV1CommandValue      = regexp.MustCompile(`<[A-Z][A-Z0-9_-]*>`)
	executionV1SHA256            = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type ExecutionIssueSnapshotReadFuncV1 func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error)

type executionOwnerIssueV1 struct {
	URL        string `json:"url"`
	Body       string `json:"body"`
	BodySHA256 string `json:"body_sha256"`
}

type executionOwnerCommandsV1 struct {
	LeaseStatus  string `json:"lease_status"`
	Claim        string `json:"claim"`
	RemoteCreate string `json:"remote_create"`
	Complete     string `json:"complete"`
}

type executionOwnerContextPacketV1 struct {
	SchemaVersion    int                      `json:"schema_version"`
	LifecycleID      string                   `json:"lifecycle_id"`
	Mode             model.ExecutionModeV1    `json:"mode"`
	SourceRoot       string                   `json:"source_root"`
	WorktreeRoot     string                   `json:"worktree_root"`
	WorktreeBase     string                   `json:"worktree_base"`
	Branch           string                   `json:"branch"`
	BaseHead         string                   `json:"base_head"`
	CurrentHead      string                   `json:"current_head"`
	LeaseGeneration  uint64                   `json:"lease_generation"`
	ClaimTokenFile   string                   `json:"claim_token_file"`
	Issue            executionOwnerIssueV1    `json:"issue"`
	OwnerHost        string                   `json:"owner_host"`
	OwnerModel       string                   `json:"owner_model"`
	OwnerEffort      string                   `json:"owner_effort,omitempty"`
	RequiredDocs     []string                 `json:"required_docs"`
	RequiredSkills   []string                 `json:"required_skills"`
	AcceptanceIDs    []string                 `json:"acceptance_ids"`
	Verification     []string                 `json:"verification_commands"`
	TuringReportPath string                   `json:"turing_report_path"`
	Commands         executionOwnerCommandsV1 `json:"commands"`
}

type executionOwnerSnapshotV1 struct {
	issue                executionOwnerIssueV1
	requiredDocs         []string
	requiredSkills       []string
	acceptanceIDs        []string
	verificationCommands []string
}

type executionOwnerArtifactsV1 struct {
	packetPath   string
	packetSHA256 string
	promptPath   string
	promptSHA256 string
	prompt       string
}

func validateExecutionInitialClaimContextV1(ctx context.Context, stateRoot string, req ExecutionClaimRequestV1, deps ExecutionClaimDependenciesV1) (func(IssueOpsRecord) error, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return nil, err
	}
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || req.Generation != 1 {
		return nil, nil
	}
	issueDigest := strings.ToLower(strings.TrimSpace(req.IssueBodySHA256))
	packetDigest := strings.ToLower(strings.TrimSpace(req.ContextPacketSHA256))
	if !executionV1SHA256.MatchString(issueDigest) || !executionV1SHA256.MatchString(packetDigest) {
		return nil, fmt.Errorf("initial Orca claim requires sealed issue and context packet digests")
	}
	if err := validateExecutionClaimPacketV1(record, issueDigest, packetDigest); err != nil {
		return nil, err
	}
	if deps.ReadIssue == nil {
		return nil, fmt.Errorf("remote issue snapshot reader is unavailable for initial Orca claim")
	}
	snapshot, err := deps.ReadIssue(ctx, executionOwnerIssueProviderV1(record), port.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return nil, fmt.Errorf("read remote issue before claim: %w", err)
	}
	if strings.TrimSpace(snapshot.URL) != strings.TrimSpace(record.IssueURL) || digestExecutionOwnerBytesV1([]byte(snapshot.Body)) != issueDigest {
		return nil, fmt.Errorf("remote issue body digest drifted from the sealed owner context")
	}
	return func(current IssueOpsRecord) error {
		return validateExecutionClaimPacketV1(current, issueDigest, packetDigest)
	}, nil
}

func validateExecutionClaimPacketV1(record IssueOpsRecord, issueDigest, packetDigest string) error {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Lease.Generation != 1 {
		return fmt.Errorf("sealed owner context no longer matches the initial Orca generation")
	}
	packetPath, _ := executionOwnerArtifactPathsV1(record)
	data, err := readExecutionOwnerArtifactV1(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if digestExecutionOwnerBytesV1(data) != packetDigest {
		return fmt.Errorf("sealed context packet digest mismatch")
	}
	var packet executionOwnerContextPacketV1
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	if packet.SchemaVersion != model.IssueOpsSchemaVersion || packet.LifecycleID != record.ID || packet.Mode != record.Execution.Mode ||
		!samePath(packet.SourceRoot, record.Execution.Workspace.SourceRoot) || !samePath(packet.WorktreeRoot, record.Execution.Workspace.Root) ||
		packet.Branch != record.Execution.Workspace.Branch || packet.BaseHead != record.Execution.Workspace.BaseHead || packet.LeaseGeneration != 1 ||
		packet.ClaimTokenFile != claimTokenPath(record) || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch")
	}
	if packet.Issue.BodySHA256 != issueDigest || digestExecutionOwnerBytesV1([]byte(packet.Issue.Body)) != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch")
	}
	return nil
}

func readExecutionOwnerArtifactV1(root, path string) ([]byte, error) {
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
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > executionV1OwnerArtifactLimit) {
			return nil, fmt.Errorf("owner artifact must be a private bounded regular file")
		}
	}
	return os.ReadFile(path)
}

func readExecutionOwnerSnapshotV1(ctx context.Context, record IssueOpsRecord, read ExecutionIssueSnapshotReadFuncV1) (executionOwnerSnapshotV1, error) {
	if read == nil {
		return executionOwnerSnapshotV1{}, fmt.Errorf("remote issue snapshot reader is unavailable")
	}
	provider := executionOwnerIssueProviderV1(record)
	if provider == "" || strings.TrimSpace(record.IssueURL) == "" {
		return executionOwnerSnapshotV1{}, fmt.Errorf("linked GitHub or GitLab issue is required before owner dispatch")
	}
	snapshot, err := read(ctx, provider, port.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return executionOwnerSnapshotV1{}, fmt.Errorf("read remote issue snapshot: %w", err)
	}
	if strings.TrimSpace(snapshot.URL) != strings.TrimSpace(record.IssueURL) || strings.TrimSpace(snapshot.Body) == "" || len(snapshot.Body) > executionV1OwnerArtifactLimit/2 {
		return executionOwnerSnapshotV1{}, fmt.Errorf("remote issue snapshot identity or bounded body is invalid")
	}
	acceptance := uniqueExecutionOwnerValuesV1(executionV1AcceptanceID.FindAllString(snapshot.Body, -1))
	verification := extractExecutionOwnerVerificationV1(snapshot.Body)
	if len(acceptance) == 0 || len(verification) == 0 {
		return executionOwnerSnapshotV1{}, fmt.Errorf("remote issue must contain acceptance IDs and an exact verification command block")
	}
	return executionOwnerSnapshotV1{
		issue:                executionOwnerIssueV1{URL: strings.TrimSpace(snapshot.URL), Body: snapshot.Body, BodySHA256: digestExecutionOwnerBytesV1([]byte(snapshot.Body))},
		requiredDocs:         executionOwnerRequiredDocsV1(record.Repo),
		requiredSkills:       []string{"issueops", "turing"},
		acceptanceIDs:        acceptance,
		verificationCommands: verification,
	}, nil
}

func executionOwnerIssueProviderV1(record IssueOpsRecord) string {
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

func buildExecutionOwnerArtifactsV1(record IssueOpsRecord, req ExecutionPrepareRequestV1, snapshot executionOwnerSnapshotV1) (executionOwnerArtifactsV1, error) {
	if record.Execution == nil || record.Execution.Lease.Generation == 0 {
		return executionOwnerArtifactsV1{}, fmt.Errorf("execution identity is unavailable for owner packet")
	}
	packetPath, promptPath := executionOwnerArtifactPathsV1(record)
	commands := executionOwnerCommandsForV1(record, req, snapshot.issue.BodySHA256)
	packet := executionOwnerContextPacketV1{
		SchemaVersion: model.IssueOpsSchemaVersion, LifecycleID: record.ID, Mode: record.Execution.Mode,
		SourceRoot: record.Execution.Workspace.SourceRoot, WorktreeRoot: record.Execution.Workspace.Root,
		WorktreeBase: filepath.Dir(record.Execution.Workspace.Root), Branch: record.Execution.Workspace.Branch,
		BaseHead: record.Execution.Workspace.BaseHead, CurrentHead: record.Execution.Workspace.BaseHead,
		LeaseGeneration: record.Execution.Lease.Generation, ClaimTokenFile: claimTokenPath(record), Issue: snapshot.issue,
		OwnerHost: strings.ToLower(strings.TrimSpace(req.OwnerHost)), OwnerModel: strings.TrimSpace(req.OwnerModel), OwnerEffort: strings.TrimSpace(req.OwnerEffort),
		RequiredDocs: snapshot.requiredDocs, RequiredSkills: snapshot.requiredSkills, AcceptanceIDs: snapshot.acceptanceIDs,
		Verification: snapshot.verificationCommands, TuringReportPath: executionOwnerTuringReportPathV1(record), Commands: commands,
	}
	packetBytes, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return executionOwnerArtifactsV1{}, err
	}
	packetBytes = append(packetBytes, '\n')
	if len(packetBytes) > executionV1OwnerArtifactLimit {
		return executionOwnerArtifactsV1{}, fmt.Errorf("owner context packet exceeds %d bytes", executionV1OwnerArtifactLimit)
	}
	packetDigest := digestExecutionOwnerBytesV1(packetBytes)
	prompt, err := renderExecutionOwnerPromptV1(packet, packetPath, packetDigest)
	if err != nil {
		return executionOwnerArtifactsV1{}, err
	}
	if err := validateExecutionOwnerCatalogV1(commands); err != nil {
		return executionOwnerArtifactsV1{}, err
	}
	if err := writeExecutionOwnerArtifactV1(record.Execution.Workspace.Root, packetPath, packetBytes); err != nil {
		return executionOwnerArtifactsV1{}, err
	}
	if err := writeExecutionOwnerArtifactV1(record.Execution.Workspace.Root, promptPath, []byte(prompt)); err != nil {
		return executionOwnerArtifactsV1{}, err
	}
	return executionOwnerArtifactsV1{
		packetPath: packetPath, packetSHA256: packetDigest, promptPath: promptPath,
		promptSHA256: digestExecutionOwnerBytesV1([]byte(prompt)), prompt: prompt,
	}, nil
}

func renderExecutionOwnerPromptV1(packet executionOwnerContextPacketV1, packetPath, packetDigest string) (string, error) {
	if err := validateExecutionOwnerPromptInputsV1(packet, packetPath, packetDigest); err != nil {
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
		"REQUIRED_DOCS": renderExecutionOwnerLinesV1(packet.RequiredDocs), "REQUIRED_SKILLS": renderExecutionOwnerLinesV1(packet.RequiredSkills),
		"ACCEPTANCE_IDS": strings.Join(packet.AcceptanceIDs, ", "), "VERIFICATION_COMMANDS": renderExecutionOwnerLinesV1(packet.Verification),
		"TURING_REPORT_PATH": packet.TuringReportPath, "REMOTE_CREATE_COMMAND": packet.Commands.RemoteCreate, "COMPLETE_COMMAND": packet.Commands.Complete,
	}
	missing := ""
	prompt := executionV1PromptPlaceholder.ReplaceAllStringFunc(executionV1OwnerPromptTemplate, func(token string) string {
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
	if unresolved := executionV1PromptPlaceholder.FindString(prompt); unresolved != "" {
		return "", fmt.Errorf("owner prompt value introduced unresolved placeholder %s", unresolved)
	}
	if len(prompt) > executionV1OwnerArtifactLimit {
		return "", fmt.Errorf("owner prompt exceeds %d bytes", executionV1OwnerArtifactLimit)
	}
	return prompt, nil
}

func validateExecutionOwnerPromptInputsV1(packet executionOwnerContextPacketV1, packetPath, packetDigest string) error {
	scalars := []struct{ name, value string }{
		{"lifecycle_id", packet.LifecycleID}, {"mode", string(packet.Mode)}, {"source_root", packet.SourceRoot},
		{"worktree_root", packet.WorktreeRoot}, {"worktree_base", packet.WorktreeBase}, {"branch", packet.Branch},
		{"base_head", packet.BaseHead}, {"issue_url", packet.Issue.URL}, {"issue_body_sha256", packet.Issue.BodySHA256},
		{"packet_path", packetPath}, {"packet_sha256", packetDigest}, {"owner_host", packet.OwnerHost},
		{"owner_model", packet.OwnerModel}, {"owner_effort", packet.OwnerEffort}, {"turing_report_path", packet.TuringReportPath},
		{"lease_status_command", packet.Commands.LeaseStatus}, {"claim_command", packet.Commands.Claim},
		{"remote_create_command", packet.Commands.RemoteCreate}, {"complete_command", packet.Commands.Complete},
	}
	for _, scalar := range scalars {
		if strings.ContainsAny(scalar.value, "\r\n") || executionV1PromptPlaceholder.MatchString(scalar.value) {
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
			if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") || executionV1PromptPlaceholder.MatchString(value) {
				return fmt.Errorf("owner prompt %s contains an empty, multiline, or placeholder value", list.name)
			}
		}
	}
	return nil
}

func executionOwnerCommandsForV1(record IssueOpsRecord, req ExecutionPrepareRequestV1, issueBodySHA256 string) executionOwnerCommandsV1 {
	generation := record.Execution.Lease.Generation
	actorFlags := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>", "--agent-id", "<AGENT_ID_OR_NONE>",
		"--session-pid", "<SESSION_PID>", "--session-started-at", "<SESSION_STARTED_AT>", "--session-executable", "<SESSION_EXECUTABLE>",
		"--cwd", quoteExecutionOwnerArgV1(record.Execution.Workspace.Root),
	}, " ")
	status := "agent-harness issueops execution status --id " + quoteExecutionOwnerArgV1(record.ID) + " --json"
	claim := "none"
	if record.Execution.Mode == model.ExecutionModeOrca {
		claim = "agent-harness issueops execution claim --id " + quoteExecutionOwnerArgV1(record.ID) +
			" --generation " + strconv.FormatUint(generation, 10) + " --claim-token-file " + quoteExecutionOwnerArgV1(claimTokenPath(record)) +
			" --issue-body-sha256 " + strings.TrimSpace(issueBodySHA256) + " --context-packet-sha256 <PACKET_SHA256> " + actorFlags + " --json"
	}
	base := ""
	if record.BranchPrepare != nil {
		base = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
	remote := "agent-harness issueops remote create-pr --id " + quoteExecutionOwnerArgV1(record.ID) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --title <PR_TITLE> --body-file <PR_BODY_FILE>" +
		" --head " + quoteExecutionOwnerArgV1(record.Execution.Workspace.Branch) + " --base " + quoteExecutionOwnerArgV1(base) +
		" --label <LABEL> --assignee <ASSIGNEE> " + actorFlags + " --confirm --json"
	complete := "agent-harness issueops execution complete --id " + quoteExecutionOwnerArgV1(record.ID) +
		" --generation " + strconv.FormatUint(generation, 10) + " --final-head <FINAL_HEAD> --turing-report " + quoteExecutionOwnerArgV1(executionOwnerTuringReportPathV1(record)) +
		" --remote-artifact-url <DRAFT_PR_OR_MR_URL> --verification <VERIFICATION_EVIDENCE> " + actorFlags + " --confirm --json"
	return executionOwnerCommandsV1{LeaseStatus: status, Claim: claim, RemoteCreate: remote, Complete: complete}
}

func validateExecutionOwnerCatalogV1(commands executionOwnerCommandsV1) error {
	for _, path := range []string{"execution status", "execution claim", "execution complete", "remote create-pr"} {
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
	}
	for _, check := range checks {
		if check.command == "none" {
			continue
		}
		command := executionV1CommandValue.ReplaceAllString(check.command, "VALUE")
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

func executionOwnerArtifactPathsV1(record IssueOpsRecord) (string, string) {
	key := digestExecutionOwnerBytesV1([]byte(record.ID))[:16]
	base := filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, "generation-"+strconv.FormatUint(record.Execution.Lease.Generation, 10))
	return filepath.Join(base, "context.json"), filepath.Join(base, "owner-prompt.txt")
}

func executionOwnerTuringReportPathV1(record IssueOpsRecord) string {
	key := digestExecutionOwnerBytesV1([]byte(record.ID))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "turing", "issueops-v1-"+key+".json")
}

func writeExecutionOwnerArtifactV1(root, path string, value []byte) error {
	if len(value) == 0 || len(value) > executionV1OwnerArtifactLimit {
		return fmt.Errorf("owner artifact is empty or oversized")
	}
	if err := ensureExecutionOwnerArtifactDirectoryV1(root, filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		current, readErr := os.ReadFile(path)
		info, statErr := os.Lstat(path)
		if readErr != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !strings.EqualFold(digestExecutionOwnerBytesV1(current), digestExecutionOwnerBytesV1(value)) {
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

func ensureExecutionOwnerArtifactDirectoryV1(root, target string) error {
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

func executionOwnerRequiredDocsV1(root string) []string {
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

func extractExecutionOwnerVerificationV1(body string) []string {
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
	return uniqueExecutionOwnerValuesV1(commands)
}

func uniqueExecutionOwnerValuesV1(values []string) []string {
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

func renderExecutionOwnerLinesV1(values []string) string {
	if len(values) == 0 {
		return "- none"
	}
	rows := append([]string(nil), values...)
	for index := range rows {
		rows[index] = "- " + rows[index]
	}
	return strings.Join(rows, "\n")
}

func quoteExecutionOwnerArgV1(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func digestExecutionOwnerBytesV1(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

package feedbackcleanup

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	port "agent-harness/internal/port"

	orphancontract "agent-harness/internal/contract/issueopsorphancleanup"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type Deps struct {
	ParseFlags   func(fs *flag.FlagSet, args []string) (bool, error)
	PrintResult  func(record issueopscontract.IssueOpsRecord, jsonOut bool, err error) error
	PrintJSON    func(value any) error
	PrintError   func(err error) error
	VerifyMerged func(issueopscontract.IssueOpsRemoteArtifactVerification) error
	// ObserveArtifactMerged는 cleanup abandon 전용이다. VerifyMerged는 조회
	// 실패와 미병합을 모두 error로 뭉개는데, 미병합을 요구하는 게이트는 둘을
	// 구분해야 한다. 관측에 성공한 경우에만 (merged, nil)이 오고, 조회가
	// 실패하면 error가 오며 그때 병합 여부는 미상이다. nil이면 abandon의
	// artifact 게이트는 통과가 아니라 거부다(#342, fail-closed).
	ObserveArtifactMerged func(issueopscontract.IssueOpsRemoteArtifactVerification) (bool, error)
	// VerifyMergedHead는 cleanup remote-branch 전용이다. 게이트 ⑧·⑨·⑩이
	// 같은 readback을 공유해야 하므로 머지 여부와 head ref 정체를 함께 받는다.
	VerifyMergedHead func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error)
	Provider         func(provider string) (port.IssueProvider, error)
	// CleanupFinishGit and InspectCleanupProcesses expose the finish oracle's
	// read-only local observers to status tests without weakening production
	// defaults. Both remain nil in the live composition root.
	CleanupFinishGit        func(dir string, args ...string) (int, string)
	InspectCleanupProcesses func(root string) ([]string, error)
	OrphanPreview           func(context.Context, orphancontract.Request) (orphancontract.Result, error)
	OrphanApply             func(context.Context, orphancontract.Request, orphancontract.ApplyRequest) (orphancontract.Result, error)
	// RemoveOrcaWorktree는 cleanup finish의 ② 단계(orca 회수, force=false)다.
	// "이미 없음"은 wiring에서 성공으로 정규화한다(멱등 계약).
	RemoveOrcaWorktree func(ctx context.Context, worktreeID string) error
	// OrcaIntent는 cleanup abandon의 pending_intent_safe 게이트가 sealed
	// marker로 orca 인벤토리를 실조회하는 표면이다. nil이면 그 게이트는 통과가
	// 아니라 거부다(#106, fail-closed).
	OrcaIntent port.ExecutionOrcaProvisioner
	// OrcaOwner는 cleanup abandon의 orca_resources_absent 게이트가 자원 잔여를
	// 실조회하는 표면이다. OrcaIntent와 같은 이유로 nil이면 통과가 아니라
	// 거부다 — 다만 orca 바인딩이 있는 레코드에서만 요구된다(#136).
	OrcaOwner  port.ExecutionOrcaOwnerInspector
	Provenance provenanceport.Observer
}

func RunFeedback(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops feedback add --id ID --source TEXT --body TEXT --host HOST --session-id SESSION --cwd PATH [--agent-id ID] [--classification TEXT] [--json]\n       agent-harness issueops feedback mark-issue-updated --id ID --host HOST --session-id SESSION --cwd PATH [--agent-id ID] [--json]")
		return nil
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("issueops feedback add", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		source := fs.String("source", "", "feedback source")
		body := fs.String("body", "", "feedback body")
		classification := fs.String("classification", "", "optional feedback classification, such as contract_change, defect, question, or noise")
		host := fs.String("host", "", "native actor host")
		sessionID := fs.String("session-id", "", "native actor session id")
		agentID := fs.String("agent-id", "", "native actor agent id")
		cwd := fs.String("cwd", "", "canonical actor cwd")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := cleanupDeps.AddIssueOpsFeedbackWithActor(cleanupDeps.IssueOpsStateRoot(), *id, *source, *body, *classification, localActor(*host, *sessionID, *agentID, *cwd))
		return deps.PrintResult(record, *jsonOut, err)
	case "mark-issue-updated":
		fs := flag.NewFlagSet("issueops feedback mark-issue-updated", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		host := fs.String("host", "", "native actor host")
		sessionID := fs.String("session-id", "", "native actor session id")
		agentID := fs.String("agent-id", "", "native actor agent id")
		cwd := fs.String("cwd", "", "canonical actor cwd")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := cleanupDeps.MarkIssueOpsContractFeedbackIssueUpdatedWithActor(cleanupDeps.IssueOpsStateRoot(), *id, localActor(*host, *sessionID, *agentID, *cwd))
		return deps.PrintResult(record, *jsonOut, err)
	default:
		return fmt.Errorf("unknown issueops feedback subcommand")
	}
}

func localActor(host, sessionID, agentID, cwd string) issueopscontract.IssueOpsActor {
	ancestry, _ := cleanupDeps.ObserveNativeProcessAncestry(os.Getpid())
	return issueopscontract.IssueOpsActor{
		Host: host, SessionID: sessionID, AgentID: agentID, CWD: cwd,
		NativeProcessAncestry: ancestry,
	}
}

func RunCleanup(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops cleanup status --id ID [--merged] [--json]\n       agent-harness issueops cleanup close-children --id ID --merged [--confirm] [--json]\n       agent-harness issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME --provider github|gitlab --kind pr|mr --artifact-url URL [--apply --confirm --fingerprint SHA256] [--json]\n       agent-harness issueops cleanup remote-branch --id ID (--preview | --apply --confirm --fingerprint SHA256) [--json]\n       agent-harness issueops cleanup finish --id ID [--provider github|gitlab] (--preview | --apply --confirm --fingerprint SHA256) [--json]\n       agent-harness issueops cleanup abandon --id ID --reason TEXT (--preview | --apply --confirm --fingerprint SHA256) [--json]")
		return nil
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("issueops cleanup status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		merged := fs.Bool("merged", false, "confirm the remote PR/MR was verified merged before cleanup")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		status, err := cleanupStatus(*id, *merged, deps)
		if err != nil {
			if *jsonOut {
				if printErr := deps.PrintError(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *jsonOut {
			return deps.PrintJSON(status)
		}
		fmt.Printf("ready: %v\n", status.Ready)
		for _, missing := range status.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		if len(status.Choices) > 0 {
			fmt.Println("선택지:")
			for _, choice := range status.Choices {
				fmt.Println(choice)
			}
		}
		return nil
	case "remote-branch":
		return runCleanupRemoteBranch(args[1:], deps)
	case "finish":
		return runCleanupFinish(args[1:], deps)
	case "abandon":
		return runCleanupAbandon(args[1:], deps)
	case "close-children":
		fs := flag.NewFlagSet("issueops cleanup close-children", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		merged := fs.Bool("merged", false, "confirm child PR/MR merge into the parent work branch before closing child tasks")
		confirm := fs.Bool("confirm", false, "execute remote child close and record verification; without this, dry-run preview only")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		verifiedMerged := CleanupMerged(*id, *merged, deps)
		// 요청 여부와 검증 결과를 함께 넘긴다. 위상 규약 이전의 우산 레코드는
		// 자체 PR이 없어 verifiedMerged가 항상 false가 되는데, 그 구간에서만
		// core가 자식의 원격 closed 상태를 대체 증거로 조회한다(#129).
		result, err := cleanupDeps.CloseIssueOpsChildren(cleanupDeps.IssueOpsStateRoot(), *id, issueopscontract.IssueOpsCloseChildrenRequest{
			Merged:                 verifiedMerged,
			MergeEvidenceRequested: *merged,
			Confirm:                *confirm,
		}, deps.Provider)
		if err != nil {
			if *jsonOut {
				if printErr := deps.PrintError(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *jsonOut {
			return deps.PrintJSON(result)
		}
		fmt.Printf("closed children: %d\n", result.ClosedCount)
		for _, child := range result.Children {
			if child.Preview != "" {
				fmt.Println(child.Preview)
			} else {
				fmt.Printf("- %s closed=%t state=%s\n", child.URL, child.Closed, child.State)
			}
		}
		return nil
	case "orphan":
		return runOrphanCleanup(args[1:], deps)
	default:
		return fmt.Errorf("unknown issueops cleanup subcommand")
	}
}

func cleanupStatus(id string, mergedRequested bool, deps Deps) (issueopscontract.IssueOpsCleanupStatus, error) {
	record, err := cleanupDeps.ReadIssueOps(cleanupDeps.IssueOpsStateRoot(), id)
	if err != nil {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, err
	}
	structural := cleanupDeps.IssueOpsCleanupStatusForRecord(record, issueopscontract.IssueOpsCleanupStatusRequest{})
	if !mergedRequested || record.Phase != issueopscontract.IssueOpsPhaseDone || len(cleanupDeps.IssueOpsRemoteArtifactMissing(record)) > 0 {
		return structural, nil
	}

	providerName := cleanupDeps.ResolveRecordProvider(record)
	if providerName == "" {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, fmt.Errorf("cannot determine provider from IssueOps record")
	}
	prov, err := deps.Provider(providerName)
	if err != nil {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, err
	}
	if deps.VerifyMergedHead == nil {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, fmt.Errorf("merge verification is not configured")
	}
	mergedArtifact, err := deps.VerifyMergedHead(*record.RemoteArtifact)
	if err != nil {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, fmt.Errorf("merge evidence readback failed (refusing to continue): %w", err)
	}
	snapshot, err := cleanupDeps.ReadRemoteIssueSnapshot(context.Background(), prov, port.ExecutionIssueSnapshotRequest{
		Repo: record.Repo, URL: record.IssueURL,
	})
	if err != nil {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, fmt.Errorf("issue readback failed (refusing to continue): %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, fmt.Errorf("cannot resolve current directory (refusing cleanup status): %w", err)
	}
	result, finishErr := cleanupDeps.CleanupFinish(context.Background(), cleanupDeps.IssueOpsStateRoot(), cleanupFinishRequest(
		record, snapshot, mergedArtifact, cwd, false, false, "",
	), deps, prov)
	if finishErr != nil && (result.ID != id || len(result.Missing) == 0) {
		return issueopscontract.IssueOpsCleanupStatus{OK: false, ID: id}, finishErr
	}
	status := issueopscontract.IssueOpsCleanupStatus{
		OK:                true,
		Ready:             result.OK && len(result.Missing) == 0,
		ID:                result.ID,
		Merged:            true,
		Missing:           append([]string(nil), result.Missing...),
		Warnings:          append([]string(nil), result.WorkspaceProcesses...),
		WorktreePath:      result.WorktreePath,
		Branch:            result.Branch,
		RemoteArtifactURL: structural.RemoteArtifactURL,
	}
	return cleanupDeps.FinalizeIssueOpsCleanupStatus(status), nil
}

func cleanupFinishRequest(record issueopscontract.IssueOpsRecord, snapshot port.ExecutionIssueSnapshot, mergedArtifact issueopscontract.CleanupRemoteBranchArtifactHead, cwd string, apply, confirm bool, fingerprint string) issueopscontract.CleanupFinishRequest {
	return issueopscontract.CleanupFinishRequest{
		ID:                  record.ID,
		CWD:                 cwd,
		Merged:              true,
		CompletionReflected: strings.Contains(snapshot.Body, port.IssueBodyCompletionStartMarker),
		IssueClosed:         strings.EqualFold(strings.TrimSpace(snapshot.State), "closed"),
		MergedBaseBranch:    mergedArtifact.BaseRefName,
		Apply:               apply,
		Confirm:             confirm,
		Fingerprint:         fingerprint,
	}
}

func runOrphanCleanup(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops cleanup orphan", flag.ContinueOnError)
	id := fs.String("id", "", "missing IssueOps lifecycle id expected to have no record")
	repo := fs.String("repo", "", "exact canonical repository root")
	worktree := fs.String("worktree", "", "exact recordless worktree path")
	branch := fs.String("branch", "", "exact local branch checked out by the worktree")
	provider := fs.String("provider", "", "remote artifact provider: github or gitlab")
	kind := fs.String("kind", "", "remote artifact kind: pr or mr")
	artifactURL := fs.String("artifact-url", "", "merged remote pull or merge request URL")
	apply := fs.Bool("apply", false, "remove only the confirmed local worktree and local branch")
	confirm := fs.Bool("confirm", false, "confirm the exact preview fingerprint for --apply")
	fingerprint := fs.String("fingerprint", "", "ready preview fingerprint required for --apply")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	if *confirm && !*apply {
		return fmt.Errorf("orphan cleanup --confirm requires --apply")
	}
	if strings.TrimSpace(*fingerprint) != "" && !*apply {
		return fmt.Errorf("orphan cleanup --fingerprint requires --apply")
	}
	request := orphancontract.Request{
		ID:           *id,
		RepoRoot:     *repo,
		WorktreePath: *worktree,
		Branch:       *branch,
		Artifact: issueopscontract.IssueOpsRemoteArtifactVerification{
			Provider: *provider,
			Kind:     *kind,
			URL:      *artifactURL,
		},
	}
	var (
		result orphancontract.Result
		err    error
	)
	if *apply {
		if !*confirm {
			return fmt.Errorf("orphan cleanup apply requires --confirm")
		}
		if deps.OrphanApply == nil {
			return fmt.Errorf("orphan cleanup apply is unavailable")
		}
		result, err = deps.OrphanApply(context.Background(), request, orphancontract.ApplyRequest{Confirm: *confirm, Fingerprint: *fingerprint})
	} else {
		if deps.OrphanPreview == nil {
			return fmt.Errorf("orphan cleanup preview is unavailable")
		}
		result, err = deps.OrphanPreview(context.Background(), request)
	}
	if *jsonOut {
		if printErr := deps.PrintJSON(result); printErr != nil {
			return printErr
		}
	} else {
		printOrphanCleanupResult(result)
	}
	return err
}

func printOrphanCleanupResult(result orphancontract.Result) {
	fmt.Printf("ready: %v\n", result.Ready)
	fmt.Printf("head: %s\n", result.HeadSHA)
	fmt.Printf("recovery path: %s\n", result.RecoveryPath)
	for _, missing := range result.Missing {
		fmt.Printf("- missing: %s\n", missing)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("- warning: %s\n", warning)
	}
	if result.Fingerprint != "" {
		fmt.Printf("fingerprint: %s\n", result.Fingerprint)
	}
	if result.RemoteBranchDeletion != "" {
		fmt.Printf("remote branch: %s\n", result.RemoteBranchDeletion)
	}
}

// runCleanupFinish는 record-backed 머지 후 정리를 실행한다. merged·completion
// 반영·이슈 close는 전부 원격 readback으로 판정하고, readback 실패는 강등 없이
// 거부한다(fail-closed — 설계 v5 WS3).
func runCleanupFinish(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops cleanup finish", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	preview := fs.Bool("preview", false, "evaluate gates and issue a fingerprint without mutating")
	apply := fs.Bool("apply", false, "run the destructive cleanup steps")
	confirm := fs.Bool("confirm", false, "confirm the destructive apply")
	fingerprint := fs.String("fingerprint", "", "fingerprint issued by the latest --preview")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	// --preview와 --apply는 배타 모드다: 파괴 실행 요청에 preview가 섞이면
	// 안전한 쪽이 아니라 명시적 거부를 택한다(C2-F3).
	if *preview && *apply {
		return fmt.Errorf("cleanup finish --preview and --apply are mutually exclusive")
	}
	// 모드 명시 규율: usage가 (--preview | --apply …)를 요구하므로 조용한
	// 기본 동작 대신 명시를 요구한다.
	if !*preview && !*apply {
		return fmt.Errorf("cleanup finish requires exactly one mode: --preview or --apply --confirm --fingerprint SHA256")
	}
	record, err := cleanupDeps.ReadIssueOps(cleanupDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return printCleanupFinishError(deps, *jsonOut, err)
	}
	providerName := *providerOverride
	if providerName == "" {
		providerName = cleanupDeps.ResolveRecordProvider(record)
	}
	if providerName == "" {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("cannot determine provider from IssueOps record; pass --provider"))
	}
	prov, err := deps.Provider(providerName)
	if err != nil {
		return printCleanupFinishError(deps, *jsonOut, err)
	}
	if record.RemoteArtifact == nil {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("cleanup finish requires a verified remote artifact"))
	}
	// 머지 여부와 base ref는 반드시 같은 readback에서 나와야 한다: 다른 시점의
	// 관측을 섞으면 "머지된 시점의 base"를 판정할 근거가 사라진다.
	if deps.VerifyMergedHead == nil {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("merge verification is not configured"))
	}
	mergedArtifact, err := deps.VerifyMergedHead(*record.RemoteArtifact)
	if err != nil {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("merge evidence readback failed (refusing to continue): %w", err))
	}
	snapshot, err := cleanupDeps.ReadRemoteIssueSnapshot(context.Background(), prov, port.ExecutionIssueSnapshotRequest{
		Repo: record.Repo, URL: record.IssueURL,
	})
	if err != nil {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("issue readback failed (refusing to continue): %w", err))
	}
	cwd, err := os.Getwd()
	if err != nil {
		// Getwd 실패의 대표 원인이 "현재 디렉토리 삭제"다 — 자기파괴 방지
		// 가드를 여는 대신 fail-closed로 거부한다(C2-F4).
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("cannot resolve current directory (refusing destructive cleanup): %w", err))
	}
	req := cleanupFinishRequest(record, snapshot, mergedArtifact, cwd, *apply, *confirm, *fingerprint)
	result, err := cleanupDeps.CleanupFinish(context.Background(), cleanupDeps.IssueOpsStateRoot(), req, deps, prov)
	var bindErr error
	result.NextCommand, bindErr = bindCleanupNextCommand(result.NextCommand, cleanupExecutionGeneration(record), deps.Provenance)
	if bindErr != nil {
		return printCleanupFinishError(deps, *jsonOut, bindErr)
	}
	if err != nil {
		if *jsonOut {
			if printErr := deps.PrintJSON(result); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	if result.RecordDeleted {
		fmt.Printf("cleanup finished: worktree=%v branch=%v record deleted\n", result.WorktreeRemoved, result.BranchDeleted)
	} else {
		fmt.Printf("fingerprint: %s\n", result.Fingerprint)
		if result.NextCommand != "" {
			fmt.Printf("next: %s\n", result.NextCommand)
		}
	}
	return nil
}

// runCleanupRemoteBranch는 머지 검증된 사이클의 원격 브랜치를 typed 경로로
// 삭제한다. 원격 삭제 자체는 git 직접 호출이고, provider는 감사 라인 반영에만
// 쓰인다(#116 부속 변경 — brooks M12).
func runCleanupRemoteBranch(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops cleanup remote-branch", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	preview := fs.Bool("preview", false, "evaluate gates and issue a fingerprint without mutating")
	apply := fs.Bool("apply", false, "delete the merged remote branch")
	confirm := fs.Bool("confirm", false, "confirm the destructive apply")
	fingerprint := fs.String("fingerprint", "", "fingerprint issued by the latest --preview")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	// finish/abandon과 동일한 모드 배타 규율(C2-F3).
	if *preview && *apply {
		return fmt.Errorf("cleanup remote-branch --preview and --apply are mutually exclusive")
	}
	if !*preview && !*apply {
		return fmt.Errorf("cleanup remote-branch requires exactly one mode: --preview or --apply --confirm --fingerprint SHA256")
	}
	record, err := cleanupDeps.ReadIssueOps(cleanupDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return printCleanupFinishError(deps, *jsonOut, err)
	}
	providerName := cleanupDeps.ResolveRecordProvider(record)
	if providerName == "" {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("cannot determine provider from IssueOps record"))
	}
	prov, err := deps.Provider(providerName)
	if err != nil {
		return printCleanupFinishError(deps, *jsonOut, err)
	}
	if deps.VerifyMergedHead == nil {
		return printCleanupFinishError(deps, *jsonOut, fmt.Errorf("merge verification is not configured"))
	}
	result, err := cleanupDeps.CleanupRemoteBranch(context.Background(), cleanupDeps.IssueOpsStateRoot(), issueopscontract.CleanupRemoteBranchRequest{
		ID:          *id,
		Apply:       *apply,
		Confirm:     *confirm,
		Fingerprint: *fingerprint,
	}, deps, prov)
	var bindErr error
	result.NextCommand, bindErr = bindCleanupNextCommand(result.NextCommand, cleanupExecutionGeneration(record), deps.Provenance)
	if bindErr != nil {
		return printCleanupFinishError(deps, *jsonOut, bindErr)
	}
	if err != nil {
		if *jsonOut {
			if printErr := deps.PrintJSON(result); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	switch {
	case result.Deleted:
		fmt.Printf("remote branch deleted: branch=%s oid=%s at=%s\n", result.Branch, result.RemoteOID, result.DeletedAt)
	case result.AlreadyAbsent:
		fmt.Printf("remote branch already absent: branch=%s\n", result.Branch)
	default:
		fmt.Printf("fingerprint: %s\n", result.Fingerprint)
		if result.NextCommand != "" {
			fmt.Printf("next: %s\n", result.NextCommand)
		}
	}
	return nil
}

// runCleanupAbandon은 폐기된 비-done 사이클의 로컬 worktree, branch, record
// 수명을 종료한다.
// finish와 달리 provider를 resolve조차 하지 않는다 — 이 경로는 원격(이슈 본문·
// PR/MR·원격 브랜치)을 어떤 단계에서도 읽거나 쓰지 않는다(#106).
func runCleanupAbandon(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops cleanup abandon", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	reason := fs.String("reason", "", "why this cycle is abandoned (required, max 512 bytes); control characters and active shell characters are rejected because the lease guard parses this command exactly")
	preview := fs.Bool("preview", false, "evaluate gates and issue a fingerprint without mutating")
	apply := fs.Bool("apply", false, "remove the sealed local worktree and branch, then delete the record and its external intent rows")
	confirm := fs.Bool("confirm", false, "confirm the destructive apply")
	fingerprint := fs.String("fingerprint", "", "fingerprint issued by the latest --preview")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	// finish와 동일한 모드 배타 규율(C2-F3): 파괴 실행 요청에 preview가 섞이면
	// 안전한 쪽을 고르는 대신 명시적으로 거부한다.
	if *preview && *apply {
		return fmt.Errorf("cleanup abandon --preview and --apply are mutually exclusive")
	}
	if !*preview && !*apply {
		return fmt.Errorf("cleanup abandon requires exactly one mode: --preview or --apply --confirm --fingerprint SHA256")
	}
	result, err := cleanupDeps.CleanupAbandon(context.Background(), cleanupDeps.IssueOpsStateRoot(), issueopscontract.CleanupAbandonRequest{
		ID:               *id,
		Reason:           *reason,
		Apply:            *apply,
		Confirm:          *confirm,
		Fingerprint:      *fingerprint,
		ArtifactUnmerged: cleanupArtifactUnmerged(*id, deps),
	}, deps)
	if result.NextCommand != "" {
		record, readErr := cleanupDeps.ReadIssueOps(cleanupDeps.IssueOpsStateRoot(), *id)
		if readErr != nil {
			return printCleanupFinishError(deps, *jsonOut, readErr)
		}
		result.NextCommand, readErr = bindCleanupNextCommand(result.NextCommand, cleanupExecutionGeneration(record), deps.Provenance)
		if readErr != nil {
			return printCleanupFinishError(deps, *jsonOut, readErr)
		}
	}
	if err != nil {
		if *jsonOut {
			// 게이트 결과와 레코드 전문이 담긴 result를 그대로 흘린다 —
			// 이 JSON이 유일한 감사 채널이다.
			if printErr := deps.PrintJSON(result); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	if result.RecordDeleted {
		fmt.Printf("cleanup abandoned: worktree_removed=%t branch_deleted=%t record_deleted=%t intent_rows=%d at=%s\n",
			result.WorktreeRemoved, result.BranchDeleted, result.RecordDeleted, len(result.IntentRowsDeleted), result.AbandonedAt)
	} else {
		fmt.Printf("fingerprint: %s\n", result.Fingerprint)
		if result.NextCommand != "" {
			fmt.Printf("next: %s\n", result.NextCommand)
		}
	}
	return nil
}

func cleanupExecutionGeneration(record issueopscontract.IssueOpsRecord) uint64 {
	if record.Execution == nil {
		return 0
	}
	return record.Execution.Lease.Generation
}

func printCleanupFinishError(deps Deps, jsonOut bool, err error) error {
	if jsonOut {
		if printErr := deps.PrintError(err); printErr != nil {
			return printErr
		}
	}
	return err
}

// cleanupArtifactUnmerged는 레코드의 remote artifact가 병합되지 않았음을 실제로
// 관측했을 때만 true다. artifact가 없으면 abandon의 artifact 게이트 자체가
// 적용되지 않으므로 값은 무의미하고, 조회 수단이 없거나 조회가 실패하면 false로
// 남아 게이트가 닫힌 채 유지된다(#342).
func cleanupArtifactUnmerged(id string, deps Deps) bool {
	if deps.ObserveArtifactMerged == nil {
		return false
	}
	record, err := cleanupDeps.ReadIssueOps(cleanupDeps.IssueOpsStateRoot(), id)
	if err != nil || record.RemoteArtifact == nil {
		return false
	}
	merged, err := deps.ObserveArtifactMerged(*record.RemoteArtifact)
	if err != nil {
		return false
	}
	return !merged
}

func CleanupMerged(id string, requested bool, deps Deps) bool {
	if !requested {
		return false
	}
	record, err := cleanupDeps.ReadIssueOps(cleanupDeps.IssueOpsStateRoot(), id)
	if err != nil || record.RemoteArtifact == nil {
		return false
	}
	return deps.VerifyMerged(*record.RemoteArtifact) == nil
}

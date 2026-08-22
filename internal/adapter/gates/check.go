// Package gates는 태스크 게이트 ledger의 파일 I/O와 policy 게이트 실행을 소유한다.
//
// unlazy gate-check.mjs의 실행 의미를 그대로 따르되, CHECK 명령은 raw shell이
// 아니라 command policy 경로로 실행한다: argv 토큰화, workspace 경계, env
// allowlist, secret redaction, timeout, audit log. EXPECT가 있으면 출력 매치가
// 판정하고(exit code는 무관), 없으면 exit code가 판정한다.
package gates

import (
	gatescontract "agent-harness/internal/contract/gates"
	policycontract "agent-harness/internal/contract/policy"
	gatesdomain "agent-harness/internal/domain/gates"
	"agent-harness/internal/domain/shelltoken"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrUnmetGates는 게이트가 아직 미충족일 때의 결과 오류이다(unlazy exit 1).
type ErrUnmetGates struct {
	Unmet int
}

func (e ErrUnmetGates) Error() string {
	return fmt.Sprintf("%d unmet gate(s) remain", e.Unmet)
}

// Check는 게이트 파일들을 평가하고, StatusOnly가 아니면 미충족 게이트의 CHECK
// 명령을 실행해 체크박스와 증거를 파일에 기록한다.
func Check(req gatescontract.CheckRequest) (gatescontract.CheckResult, error) {
	root := strings.TrimSpace(req.WorkspaceRoot)
	cwd := strings.TrimSpace(req.CWD)
	if root == "" {
		root = cwd
	}
	if cwd == "" {
		cwd = root
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = gatescontract.TimeoutDefaultSeconds
	}
	files := req.Files
	if len(files) == 0 {
		discovered, err := DiscoverGateFiles(cwd)
		if err != nil {
			return gatescontract.CheckResult{}, err
		}
		if len(discovered) == 0 {
			return gatescontract.CheckResult{}, gatescontract.ErrNoGateFiles
		}
		files = discovered
	}

	result := gatescontract.CheckResult{
		OK:            true,
		SchemaVersion: gatescontract.SchemaVersion,
		StatusOnly:    req.StatusOnly,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	for _, file := range files {
		fileResult, warnings := checkFile(root, cwd, req, file)
		result.Files = append(result.Files, fileResult)
		result.Warnings = append(result.Warnings, warnings...)
		result.TotalGates += fileResult.GateCount
		result.TotalMet += fileResult.Met
		result.TotalUnmet += fileResult.Unmet
		result.TotalAbandoned += fileResult.Abandoned
		if fileResult.Error != "" {
			result.OK = false
		}
	}
	result.Complete = result.TotalUnmet == 0 && result.OK
	return result, nil
}

func checkFile(root, cwd string, req gatescontract.CheckRequest, file string) (gatescontract.FileResult, []string) {
	fileResult := gatescontract.FileResult{File: file}
	warnings := []string{}
	data, err := os.ReadFile(file)
	if err != nil {
		fileResult.Error = err.Error()
		return fileResult, warnings
	}
	ledger := gatesdomain.Parse(string(data))
	if len(ledger.Gates) == 0 {
		fileResult.Error = "no gates found"
		return fileResult, warnings
	}
	changed := false
	for i := range ledger.Gates {
		gate := &ledger.Gates[i]
		gateResult := gatescontract.GateResult{
			ID:            gate.ID,
			Title:         gate.Title,
			Checked:       gate.Checked,
			HasCheck:      strings.TrimSpace(gate.CheckCmd) != "",
			Evidence:      gate.Evidence,
			AbandonReason: gate.AbandonReason,
		}
		if !gate.Abandoned && !req.StatusOnly && gateResult.HasCheck && needsRun(gate) {
			outcome := runGateCheck(root, cwd, req, gate)
			gateResult.PolicyDenied = outcome.policyDenied
			gateResult.AuditLogID = outcome.auditLogID
			if outcome.passed {
				gatesdomain.MarkPass(&ledger, i, outcome.evidence)
				gate.Evidence = outcome.evidence
				changed = true
			} else {
				gateResult.CheckError = outcome.checkError
				if outcome.policyDenied {
					warnings = append(warnings, fmt.Sprintf("%s %s: check command denied by policy", file, gate.ID))
				}
			}
		}
		gateResult.State = gatesdomain.State(*gate)
		gateResult.Checked = gate.Checked
		gateResult.Evidence = gate.Evidence
		fileResult.Gates = append(fileResult.Gates, gateResult)
	}
	summary := gatesdomain.Summarize(ledger.Gates)
	fileResult.GateCount = summary.Total
	fileResult.Met = summary.Met
	fileResult.Unmet = summary.Unmet
	fileResult.Abandoned = summary.Abandoned
	fileResult.Complete = summary.Complete
	if changed {
		mode := os.FileMode(0o644)
		if info, statErr := os.Stat(file); statErr == nil {
			mode = info.Mode().Perm()
		}
		if writeErr := os.WriteFile(file, []byte(gatesdomain.Render(ledger)), mode); writeErr != nil {
			fileResult.Error = writeErr.Error()
			warnings = append(warnings, fmt.Sprintf("%s: failed to write updated ledger: %v", file, writeErr))
		}
	}
	return fileResult, warnings
}

// needsRun는 unlazy 규칙이다: 미체크이거나, 체크됐지만 증거가 pending인 게이트의
// CHECK를 (재)실행한다.
func needsRun(gate *gatesdomain.Gate) bool {
	return !gate.Checked || gatesdomain.EvidencePending(gate.Evidence)
}

type gateCheckOutcome struct {
	passed       bool
	evidence     string
	checkError   string
	policyDenied bool
	auditLogID   string
}

func runGateCheck(root, cwd string, req gatescontract.CheckRequest, gate *gatesdomain.Gate) gateCheckOutcome {
	outcome := gateCheckOutcome{}
	argv := shelltoken.SplitCommandTokens(gate.CheckCmd)
	if len(argv) == 0 {
		outcome.checkError = "empty CHECK command"
		return outcome
	}
	policyReq := policycontract.CommandPolicyRequest{
		WorkspaceRoot:  root,
		CWD:            cwd,
		Argv:           argv,
		Timeout:        fmt.Sprintf("%ds", req.TimeoutSeconds),
		EnvAllowlist:   req.EnvAllowlist,
		WriteAllowed:   req.WriteAllowed,
		NetworkAllowed: req.NetworkAllowed,
	}
	evaluation := EvaluateCommandPolicy(policyReq)
	if !evaluation.Allowed {
		outcome.policyDenied = true
		outcome.auditLogID = evaluation.AuditLogID
		outcome.checkError = "check denied by policy: " + strings.Join(evaluation.DenyReasons, "; ")
		return outcome
	}
	run := RunCommand(policyReq)
	outcome.auditLogID = run.Policy.AuditLogID
	output := strings.TrimRight(run.Stdout, "\n")
	if run.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimRight(run.Stderr, "\n")
	}
	evidence := gatesdomain.EvidenceTail(output, 200)
	if strings.TrimSpace(gate.Expect) != "" {
		if gatesdomain.ExpectMatches(gate.Expect, output) {
			outcome.passed = true
			outcome.evidence = evidence
			return outcome
		}
		if run.TimedOut {
			outcome.checkError = "check timed out: " + evidence
		} else {
			outcome.checkError = "expect not matched: " + evidence
		}
		return outcome
	}
	if run.TimedOut {
		outcome.checkError = "check timed out: " + evidence
		return outcome
	}
	if run.ExitCode == 0 && run.Executed {
		outcome.passed = true
		outcome.evidence = evidence
		return outcome
	}
	outcome.checkError = fmt.Sprintf("exit code %d: %s", run.ExitCode, evidence)
	return outcome
}

// DiscoverGateFiles는 unlazy 기본 파일 규칙을 따른다: root의 GATES.md와
// root/gates/*.md를 이름순으로 반환한다.
func DiscoverGateFiles(root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	files := []string{}
	if info, err := os.Stat(filepath.Join(root, "GATES.md")); err == nil && !info.IsDir() {
		files = append(files, filepath.Join(root, "GATES.md"))
	}
	entries, err := os.ReadDir(filepath.Join(root, "gates"))
	if err == nil {
		names := []string{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			files = append(files, filepath.Join(root, "gates", name))
		}
	}
	return files, nil
}

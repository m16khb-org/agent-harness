package gates

import (
	gatescontract "agent-harness/internal/contract/gates"
	gatesdomain "agent-harness/internal/domain/gates"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Init은 게이트 파일을 스캐폴드한다. 이미 있는 파일은 덮어쓰지 않는다.
// gate spec 형식: "G1: outcome | CHECK: command | EXPECT: expectation".
func Init(req gatescontract.InitRequest) (gatescontract.InitResult, error) {
	result := gatescontract.InitResult{SchemaVersion: gatescontract.SchemaVersion}
	file := strings.TrimSpace(req.File)
	if file == "" {
		file = "GATES.md"
	}
	result.File = file
	if len(req.Gates) == 0 {
		return result, fmt.Errorf("at least one --gate spec is required")
	}
	if info, err := os.Stat(file); err == nil && !info.IsDir() {
		return result, fmt.Errorf("gate file already exists: %s", file)
	}
	var body strings.Builder
	fmt.Fprintf(&body, "# Gates: %s\n\n", strings.TrimSpace(req.Scope))
	for _, spec := range req.Gates {
		gateText, err := renderGateSpec(spec)
		if err != nil {
			return result, err
		}
		body.WriteString(gateText)
		body.WriteString("\n")
	}
	if dir := filepath.Dir(file); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return result, err
		}
	}
	if err := os.WriteFile(file, []byte(body.String()), 0o644); err != nil {
		return result, err
	}
	ledger := gatesdomain.Parse(body.String())
	result.OK = true
	result.Created = true
	result.GateCount = len(ledger.Gates)
	return result, nil
}

func renderGateSpec(spec string) (string, error) {
	parts := strings.Split(spec, "|")
	heading := strings.TrimSpace(parts[0])
	if heading == "" {
		return "", fmt.Errorf("gate spec %q has no title", spec)
	}
	lines := []string{"- [ ] " + heading}
	sawEvidence := false
	for _, raw := range parts[1:] {
		segment := strings.TrimSpace(raw)
		upper := strings.ToUpper(segment)
		switch {
		case strings.HasPrefix(upper, "CHECK:"):
			lines = append(lines, "  CHECK: "+strings.TrimSpace(segment[len("CHECK:"):]))
		case strings.HasPrefix(upper, "EXPECT:"):
			lines = append(lines, "  EXPECT: "+strings.TrimSpace(segment[len("EXPECT:"):]))
		case strings.HasPrefix(upper, "EVIDENCE:"):
			lines = append(lines, "  EVIDENCE: "+strings.TrimSpace(segment[len("EVIDENCE:"):]))
			sawEvidence = true
		default:
			return "", fmt.Errorf("gate spec %q has unknown segment %q (want CHECK:, EXPECT:, or EVIDENCE:)", spec, segment)
		}
	}
	if !sawEvidence {
		lines = append(lines, "  EVIDENCE: pending")
	}
	return strings.Join(lines, "\n"), nil
}

// Abandon은 게이트 파일에 ABANDON 라인을 기록한다(unlazy의 정직한 탈출).
func Abandon(req gatescontract.AbandonRequest) (gatescontract.AbandonResult, error) {
	result := gatescontract.AbandonResult{SchemaVersion: gatescontract.SchemaVersion, File: req.File, GateID: req.GateID}
	if strings.TrimSpace(req.File) == "" {
		req.File = "GATES.md"
		result.File = req.File
	}
	if strings.TrimSpace(req.GateID) == "" {
		return result, fmt.Errorf("--gate is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return result, fmt.Errorf("--reason is required")
	}
	data, err := os.ReadFile(req.File)
	if err != nil {
		return result, err
	}
	ledger := gatesdomain.Parse(string(data))
	found := false
	for i := range ledger.Gates {
		if ledger.Gates[i].ID == req.GateID {
			if ledger.Gates[i].Abandoned {
				return result, fmt.Errorf("gate %s is already abandoned", req.GateID)
			}
			found = true
			break
		}
	}
	if !found {
		return result, fmt.Errorf("gate %s not found in %s", req.GateID, req.File)
	}
	gatesdomain.AppendAbandon(&ledger, req.GateID, req.Reason)
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(req.File); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(req.File, []byte(gatesdomain.Render(ledger)), mode); err != nil {
		return result, err
	}
	result.OK = true
	result.Recorded = true
	result.GateID = req.GateID
	return result, nil
}

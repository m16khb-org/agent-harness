package branchprepare

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

// GitLab 안내에 결함이 셋 있었다. 근거는 추측이 아니라 상류 소스다 — `glab`이
// 로컬에 없다는 것은 계약을 확인할 수 없다는 뜻이 아니다(#180).
//
// ① `ref`가 base 브랜치 이름을 받으면 GitLab이 그 시점 브랜치 HEAD에서 브랜치를
// 만들어 orca가 봉인한 base와 갈린다. `#176`이 GitHub에서 고친 것과 같은 결함이고,
// GitLab 공식 문서가 `ref`를 "Branch name or commit SHA"로 정의한다.
//
// ② MCP `ToolArguments`가 `endpoint`/`method`/`field`를 최상위에 두었다. `glab_api`의
// input schema에는 그런 키가 없다 — gitlab-org/cli의
// internal/commands/mcp/serve/server.go에서 buildToolFromCommand가 모든 도구에
// 같은 네 키만 만든다:
//
//	inputSchema := map[string]any{"type": "object", "properties": map[string]any{
//	    argsParam: ..., flagsParam: ..., limitParam: ..., offsetParam: ...}}
//
// 즉 `args`(문자열 배열), `flags`(객체), `limit`, `offset`뿐이다.
//
// ③ `field`와 `raw-field`는 별개 플래그다. 같은 저장소의
// internal/commands/api/api.go가 둘을 따로 등록한다:
//
//	fl.StringArrayVarP(&opts.magicFields, "field", "F", nil, "Add a parameter of inferred type. ...")
//	fl.StringArrayVarP(&opts.rawFields, "raw-field", "f", nil, "Add a string parameter.")
//
// 우리 CLI 폴백은 `-f`(= `--raw-field`)를 쓰는데 MCP 단계는 `field`라고 적어 두
// 경로가 서로 다른 플래그를 지시했다. 브랜치 이름과 ref는 문자열이므로
// `raw_field`가 맞고, MCP 스키마의 키는 하이픈이 밑줄로 치환된 형태다
// (buildToolFromCommand의 strings.ReplaceAll(flag.Name, "-", "_")).

const (
	gitlabIssueURL = "https://gitlab.example.com/acme/repo/-/issues/16"
	gitlabBranch   = "16-demo"
	gitlabBaseSHA  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// ① baseSHA를 주면 두 경로 모두 그 SHA를 ref로 넘겨야 한다. base 브랜치 이름을
// 넘기면 봉인 base와 갈릴 수 있다.
func TestGitLabPinsRefToTheSealedBaseSHA(t *testing.T) {
	steps := Steps("gitlab", gitlabIssueURL, gitlabBranch, "main", gitlabBaseSHA)
	if len(steps) == 0 {
		t.Fatal("gitlab steps must exist")
	}

	mcp := gitlabMCPStep(t, steps)
	rawFields := gitlabRawFieldValues(t, mcp.ToolArguments)
	if !containsValue(rawFields, "ref="+gitlabBaseSHA) {
		t.Fatalf("MCP 단계는 ref를 봉인 base SHA에 못박아야 한다: %v", rawFields)
	}
	if containsValue(rawFields, "ref=main") {
		t.Fatalf("MCP 단계가 base 브랜치 이름을 넘기면 봉인 base와 갈린다: %v", rawFields)
	}

	fallback := gitlabFallbackStep(t, steps)
	joined := strings.Join(fallback.Command, " ")
	if !strings.Contains(joined, "ref="+gitlabBaseSHA) {
		t.Fatalf("CLI 폴백도 ref를 봉인 base SHA에 못박아야 한다: %q", joined)
	}
	if strings.Contains(joined, "ref=main") {
		t.Fatalf("CLI 폴백이 base 브랜치 이름을 넘기면 봉인 base와 갈린다: %q", joined)
	}
}

// baseSHA가 없으면 못박을 값이 없다. 종전 경로로 떨어지되 왜 고정하지 못하는지
// 밝혀야 한다 — GitHub 분기와 같은 계약이다.
func TestGitLabWithoutBaseSHAExplainsWhyItCannotPin(t *testing.T) {
	steps := Steps("gitlab", gitlabIssueURL, gitlabBranch, "main", "")

	fallback := gitlabFallbackStep(t, steps)
	joined := strings.Join(fallback.Command, " ")
	if !strings.Contains(joined, "ref=main") {
		t.Fatalf("baseSHA가 없으면 base 브랜치 이름으로 떨어진다: %q", joined)
	}

	var descriptions string
	for _, step := range steps {
		descriptions += step.Description + "\n"
	}
	if !strings.Contains(descriptions, "base-sha") {
		t.Fatalf("왜 못박지 못하는지와 무엇을 주면 되는지 밝혀야 한다:\n%s", descriptions)
	}
}

// ② MCP 인자는 glab_api의 실제 스키마를 따라야 한다. 최상위 endpoint/method/field는
// 그 스키마에 없어 검증에서 실패한다.
func TestGitLabMCPArgumentsMatchTheGlabAPISchema(t *testing.T) {
	steps := Steps("gitlab", gitlabIssueURL, gitlabBranch, "main", gitlabBaseSHA)
	mcp := gitlabMCPStep(t, steps)

	if mcp.Tool != "mcp__glab.glab_api" {
		t.Fatalf("도구 이름은 glab_api다(HasAnnotation opt-in을 통과한다): %q", mcp.Tool)
	}

	for _, stale := range []string{"endpoint", "method", "field"} {
		if _, ok := mcp.ToolArguments[stale]; ok {
			t.Fatalf("최상위 %q는 glab_api 스키마에 없다: %v", stale, mcp.ToolArguments)
		}
	}

	args, ok := mcp.ToolArguments["args"].([]string)
	if !ok || len(args) != 1 {
		t.Fatalf("args는 endpoint 하나를 담은 문자열 배열이다: %v", mcp.ToolArguments["args"])
	}
	if args[0] != "projects/:fullpath/repository/branches" {
		t.Fatalf("endpoint는 위치 인자다: %q", args[0])
	}

	flags, ok := mcp.ToolArguments["flags"].(map[string]any)
	if !ok {
		t.Fatalf("flags는 객체다: %v", mcp.ToolArguments["flags"])
	}
	if flags["method"] != "POST" {
		t.Fatalf("method는 flags 안에 있다: %v", flags["method"])
	}
}

// ③ MCP 단계와 CLI 폴백이 같은 플래그를 지시해야 한다. field는 추론형이고
// raw-field가 문자열 파라미터다.
func TestGitLabMCPAndFallbackAgreeOnRawField(t *testing.T) {
	steps := Steps("gitlab", gitlabIssueURL, gitlabBranch, "main", gitlabBaseSHA)

	mcp := gitlabMCPStep(t, steps)
	flags, ok := mcp.ToolArguments["flags"].(map[string]any)
	if !ok {
		t.Fatalf("flags는 객체다: %v", mcp.ToolArguments["flags"])
	}
	if _, wrong := flags["field"]; wrong {
		t.Fatalf("field는 추론형이다. 문자열 파라미터는 raw_field다: %v", flags)
	}
	if _, ok := flags["raw_field"]; !ok {
		t.Fatalf("raw_field가 있어야 한다(스키마 키는 하이픈이 밑줄로 치환된다): %v", flags)
	}

	fallback := gitlabFallbackStep(t, steps)
	if !containsValue(fallback.Command, "-f") {
		t.Fatalf("CLI 폴백은 -f(= --raw-field)를 쓴다: %v", fallback.Command)
	}
	if containsValue(fallback.Command, "-F") {
		t.Fatalf("-F는 --field이고 MCP 단계와 어긋난다: %v", fallback.Command)
	}
}

// 세 결함을 고쳐도 단계 수와 전략 어휘는 그대로다. GitLab은 사전 조회가 필요 없다.
func TestGitLabKeepsThreeStepsAndItsStrategies(t *testing.T) {
	steps := Steps("gitlab", gitlabIssueURL, gitlabBranch, "main", gitlabBaseSHA)
	if len(steps) != 3 {
		t.Fatalf("GitLab은 사전 조회가 없어 3단계다: %d", len(steps))
	}
	want := []string{"mcp", "fallback_api", "fail"}
	for i, step := range steps {
		if step.Strategy != want[i] {
			t.Fatalf("전략 %d은 %q여야 한다: %q", i+1, want[i], step.Strategy)
		}
		if step.Order != i+1 {
			t.Fatalf("Order는 연속이다: %d at index %d", step.Order, i)
		}
	}
}

func gitlabMCPStep(t *testing.T, steps []model.IssueOpsBranchPrepareStep) model.IssueOpsBranchPrepareStep {
	t.Helper()
	for _, step := range steps {
		if step.Strategy == "mcp" {
			return step
		}
	}
	t.Fatal("gitlab 경로에는 mcp 단계가 있다")
	return model.IssueOpsBranchPrepareStep{}
}

func gitlabFallbackStep(t *testing.T, steps []model.IssueOpsBranchPrepareStep) model.IssueOpsBranchPrepareStep {
	t.Helper()
	for _, step := range steps {
		if step.Strategy == "fallback_api" {
			return step
		}
	}
	t.Fatal("gitlab 경로에는 fallback_api 단계가 있다")
	return model.IssueOpsBranchPrepareStep{}
}

func gitlabRawFieldValues(t *testing.T, arguments map[string]any) []string {
	t.Helper()
	flags, ok := arguments["flags"].(map[string]any)
	if !ok {
		t.Fatalf("flags는 객체다: %v", arguments["flags"])
	}
	values, ok := flags["raw_field"].([]string)
	if !ok {
		t.Fatalf("raw_field는 문자열 배열이다: %v", flags["raw_field"])
	}
	return values
}

func containsValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

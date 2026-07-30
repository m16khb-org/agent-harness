# GitLab MCP Snapshot과 Provider-neutral VCS Recipe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Host가 제공한 일반 `glab_api` 결과를 GitLab IssueOps snapshot으로 안전하게 주입하고, CLI fallback과 provider-neutral `.agent-harness/VCS.md` 재사용 계약을 Codex와 Claude Code에 동일하게 제공한다.

**Architecture:** Host agent는 trusted tool catalog에서 server namespace와 무관하게 `glab_api` leaf를 발견하고 bounded evidence를 공용 execution DTO에 넣는다. Core는 injected evidence와 provider CLI 결과를 같은 GitLab issue identity 경계에서 검증하며, optional `VCS.md`는 bootstrap/doctor required 목록과 분리한다. Hook은 read/record 절차만 안내하고 shared 문서를 직접 수정하지 않는다.

**Tech Stack:** Go 1.26.3, MCP JSON schema, standard `flag`/`encoding/json`/`os`, existing IssueOps execution core, project-doc SHA-CAS, Go tests와 contract golden.

## Global Constraints

- 주석은 한글로 작성한다.
- MCP server namespace, `glab-mcp-wrapper` 이름·경로, credential profile, token/env/config를 코드·결과·recipe identity에 넣지 않는다.
- Injected snapshot은 `provider=gitlab`, `source=glab_mcp`, canonical `web_url`, bounded `body`, `state=opened|closed`만 허용한다.
- `/issues/:iid`와 `/work_items/:iid`는 authority, project path, IID가 같을 때만 같은 GitLab identity다.
- Supplied evidence가 invalid면 CLI로 fallback하지 않고 fail-closed한다. Evidence가 없을 때만 일반 `glab api` adapter를 사용한다.
- Snapshot transport 실패는 Orca readiness 실패나 `mode=direct` 전환 사유가 아니다.
- `.agent-harness/VCS.md`는 read/update/route 가능한 optional doc이며 bootstrap과 doctor의 required 목록에는 포함하지 않는다.
- GitHub recipe는 exact `gh issue view` 또는 실제 호출해 검증한 MCP schema만 기록한다. 공통 GitHub MCP 이름을 추측하지 않는다.
- Hook은 network read, git mutation, shared-doc write, cross-worktree queue handoff를 수행하지 않는다.
- Active IssueOps 문서 변경은 canonical holder worktree에서 main agent가 `project_docs_read` 후 SHA-CAS `project_docs_update`로 수행한다.
- 사용자의 명시적 자원 제한에 따라 `go test ./...`, 전체 `-race`, full self-verify를 실행하지 않는다.
- OpenWiki 자동 update를 실행하지 않는다.
- 구현은 메인 에이전트가 직접 수행하고, sub-agent는 read-only 검토에만 사용한다.

---

### Task 1: Optional VCS project-doc registry와 provider-neutral hook routing

**Files:**

- Modify: `internal/core/projectdoc/constants.go`
- Modify: `internal/core/projectdoc/path.go`
- Modify: `internal/core/projectdoc/meta.go`
- Modify: `internal/core/projectdocs/project_docs_route.go`
- Modify: `internal/core/hookprompt/hook_prompt.go`
- Modify: `internal/core/hookprompt/render_labels.go`
- Create: `internal/core/projectdoc/path_test.go`
- Test: `internal/core/projectbootstrap/project_docs_test.go`
- Test: `internal/core/projectdocs/projectdocs_test.go`
- Test: `internal/core/doctor/doctor_test.go`
- Test: `internal/core/hookprompt/hook_prompt_test.go`

**Interfaces:**

- Produces: `OptionalProjectDocNames() []string`
- Produces: `AllowedProjectDocNames() []string`
- Preserves: `ProjectDocNames() []string` and `PrefixedProjectDocNames() []string` as required-only catalogs
- Produces: VCS remote prompt hint that routes `VCS.md`, generic `glab_api`, and provider CLI fallback without naming an MCP server

- [ ] **Step 1: Write the failing registry and bootstrap tests**

```go
func TestOptionalVCSProjectDocIsAllowedButNotRequired(t *testing.T) {
	if contains(ProjectDocNames(), "VCS.md") {
		t.Fatal("VCS.md must not become a required project doc")
	}
	if !contains(AllowedProjectDocNames(), "VCS.md") {
		t.Fatal("VCS.md must be readable and writable on demand")
	}
	if got, err := NormalizeRelPath(".agent-harness/VCS.md"); err != nil || got != ".agent-harness/VCS.md" {
		t.Fatalf("NormalizeRelPath(VCS.md) = %q, %v", got, err)
	}
}
```

`TestBootstrapProjectDocsDryRunAndWrite`에는 다음 검증을 추가한다.

```go
if projectPlanContainsRel(dry.Files, ".agent-harness/VCS.md") {
	t.Fatalf("optional VCS.md must not be created by bootstrap: %+v", dry.Files)
}
```

Doctor fixture가 모든 required docs를 가진 상태에서 `VCS.md`가 없어도
`project_docs_missing` issue를 만들지 않는 assertion을 추가한다.

```go
if _, err := os.Stat(filepath.Join(repo, ".agent-harness", "VCS.md")); !os.IsNotExist(err) {
	t.Fatalf("bootstrap unexpectedly created optional VCS.md: %v", err)
}
if hasHarnessDoctorIssue(result.Issues, "project_docs_missing") {
	t.Fatalf("doctor treated optional VCS.md as required: %+v", result.Issues)
}
```

`internal/core/projectdocs/projectdocs_test.go`에서는 on-demand create/read를
검증한다.

```go
created, err := UpdateProjectDoc(ProjectDocsUpdateRequest{
	RepoRoot: root,
	RelPath:  ".agent-harness/VCS.md",
	Content:  "# VCS\n\n## GitHub\n",
	Summary:  "record verified provider recipe",
	Confirm:  true,
})
if err != nil || created.Action != "create" {
	t.Fatalf("create optional VCS.md: result=%#v err=%v", created, err)
}
read, err := ReadProjectDoc(root, ".agent-harness/VCS.md")
if err != nil || !read.Exists || !strings.Contains(read.Content, "## GitHub") {
	t.Fatalf("read optional VCS.md: result=%#v err=%v", read, err)
}
```

- [ ] **Step 2: Run the focused tests and confirm RED**

Run:

```bash
go test ./internal/core/projectdoc ./internal/core/projectbootstrap -run 'OptionalVCS|BootstrapProjectDocs' -count=1
```

Expected: `AllowedProjectDocNames`가 없거나 `VCS.md`가 unsupported라서 FAIL.

- [ ] **Step 3: Split required and optional catalogs**

`internal/core/projectdoc/constants.go`에 required 목록은 그대로 두고 optional 목록을 추가한다.

```go
var requiredProjectDocNames = []string{
	"ARCHITECTURE.md",
	"CAUTIONS.md",
	"COMMIT_POLICY.md",
	"CONSTITUTION.md",
	"CONVENTIONS.md",
	"TECH_STACK.md",
	"TESTING.md",
	"OPEN_API_SPEC.md",
	"ADR.md",
	"OPERATIONS.md",
	"AGENT_WORKFLOW.md",
}

var optionalProjectDocNames = []string{"VCS.md"}

func ProjectDocNames() []string {
	return append([]string(nil), requiredProjectDocNames...)
}

func OptionalProjectDocNames() []string {
	return append([]string(nil), optionalProjectDocNames...)
}

func AllowedProjectDocNames() []string {
	out := ProjectDocNames()
	return append(out, optionalProjectDocNames...)
}
```

`PrefixedProjectDocNames`는 `ProjectDocNames()`만 순회하고, `NormalizeRelPath`의 allowlist와 오류 메시지는 `AllowedProjectDocNames()`를 사용한다. `docMetaDescriptions`에는 다음 항목을 추가한다.

```go
"VCS.md": "Verified VCS provider capabilities, request recipes, identity checks, and CLI fallbacks.",
```

- [ ] **Step 4: Write failing route and hook tests**

```go
func TestRouteProjectDocsIncludesOptionalVCSForRemoteWork(t *testing.T) {
	root := t.TempDir()
	route, err := RouteProjectDocs(root, "GitHub issue와 GitLab MR remote 작업")
	if err != nil {
		t.Fatal(err)
	}
	if !routeContains(route.Docs, ".agent-harness/VCS.md") {
		t.Fatalf("VCS route missing: %+v", route.Docs)
	}
}
```

GitLab repo profile hook test에는 다음 문자열 검증을 추가한다.

```go
for _, want := range []string{
	"read .agent-harness/VCS.md when present",
	"glab_api",
	"server namespace",
	"glab api fallback",
	"record a successful exact-identity recipe with project_docs_read/project_docs_update",
} {
	if !strings.Contains(got.AdditionalContext, want) {
		t.Fatalf("VCS capability hint missing %q:\n%s", want, got.AdditionalContext)
	}
}
```

- [ ] **Step 5: Run the route/hook tests and confirm RED**

Run:

```bash
go test ./internal/core/projectdocs ./internal/core/hookprompt -run 'VCS|GitLabUsecaseFromRepoProfile' -count=1
```

Expected: route와 hook output에 VCS/capability 안내가 없어 FAIL.

- [ ] **Step 6: Add minimal provider-neutral routing**

`routeDocsForTask`의 첫 provider-remote 분기에 `VCS.md`를 추가한다. 대상 keyword는 `gitlab`, `github`, `glab`, `gh issue`, `vcs`, `merge request`, `pull request`, `remote issue`로 제한한다.

`BuildUserPromptMCPHints`는 repo profile이 있고 remote 작업 prompt일 때 다음 두 action을 추가한다.

```go
addPriority(
	"project_docs_read/project_docs_update",
	"Read .agent-harness/VCS.md when present; after a successful exact-identity provider read, record only the portable recipe with SHA-CAS in the canonical worktree.",
	hintPriorityAction,
)
if strings.EqualFold(repoProfile.VCS.Provider, "gitlab") {
	addPriority(
		"glab_api",
		"Discover a trusted host tool by the glab_api leaf, never by server namespace; validate exact URL identity, then use glab api fallback only when no valid MCP evidence exists.",
		hintPriorityAction,
	)
}
```

`compactHintLabel`은 VCS reason을 보존하는 두 label을 추가한다.

```go
case "project_docs_read/project_docs_update":
	if strings.Contains(h.Reason, ".agent-harness/VCS.md") {
		return "read .agent-harness/VCS.md when present; record a successful exact-identity recipe with project_docs_read/project_docs_update"
	}
	return "refresh project docs only if evidence changed"
case "glab_api":
	return "discover glab_api by leaf, not server namespace; validate exact identity; use glab api fallback only without valid MCP evidence"
```

- [ ] **Step 7: Run focused project-doc and hook tests**

Run:

```bash
go test ./internal/core/projectdoc ./internal/core/projectdocs ./internal/core/projectbootstrap ./internal/core/doctor ./internal/core/hookprompt -count=1
```

Expected: PASS, and bootstrap file count remains based on required `ProjectDocNames()`.

- [ ] **Step 8: Commit the optional document boundary**

```bash
git add internal/core/projectdoc internal/core/projectdocs internal/core/projectbootstrap/project_docs_test.go internal/core/doctor internal/core/hookprompt
git commit -m "feat(project-docs): add optional VCS recipes"
```

### Task 2: Core GitLab snapshot evidence validation and source observation

**Files:**

- Modify: `internal/port/execution_workspace.go`
- Modify: `internal/core/issueops/execution_api.go`
- Modify: `internal/core/issueops/execution_prepare.go`
- Modify: `internal/core/issueops/execution_lease.go`
- Modify: `internal/core/issueops/execution_reconcile.go`
- Create: `internal/core/issueops/execution_issue_snapshot.go`
- Create: `internal/core/issueops/execution_issue_snapshot_test.go`
- Modify: `internal/adapter/provider/gitlab/issue_snapshot.go`
- Test: `internal/adapter/provider/gitlab/issue_snapshot_test.go`

**Interfaces:**

- Produces:

```go
type ExecutionIssueSnapshotEvidence struct {
	Provider string `json:"provider"`
	Source   string `json:"source"`
	WebURL   string `json:"web_url"`
	Body     string `json:"body"`
	State    string `json:"state"`
}
```

- Extends: `port.ExecutionIssueSnapshot` with `Source string`
- Extends: `issueops.ExecutionActionRequest` with `IssueSnapshot *port.ExecutionIssueSnapshotEvidence`
- Produces: `executionIssueSnapshotReader(stateRoot string, req ExecutionActionRequest, fallback ExecutionIssueSnapshotReadFunc) (ExecutionIssueSnapshotReadFunc, func() string, error)`
- Produces: `withExecutionIssueSnapshotSource(result any, source string) any`

- [ ] **Step 1: Write the failing injected-evidence tests**

Build a GitLab Orca fixture with linked URL
`https://gitlab.example.com/acme/repo/-/work_items/69` and evidence URL
`https://gitlab.example.com/acme/repo/-/issues/69`.

```go
evidence := &port.ExecutionIssueSnapshotEvidence{
	Provider: "gitlab",
	Source:   "glab_mcp",
	WebURL:   "https://gitlab.example.com/acme/repo/-/issues/69",
	Body:     "## Acceptance\n- AC-69\n\n```bash\ngo test ./internal/core/issueops\n```",
	State:    "opened",
}
fallbackCalls := 0
fallback := func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	fallbackCalls++
	return port.ExecutionIssueSnapshot{}, errors.New("must not run")
}
```

Assertions:

- `prepare --mode orca --confirm` seals the evidence body digest.
- `fallbackCalls == 0`.
- result `IssueSnapshotSource == "glab_mcp"`.
- `/work_items/69` linked URL accepts `/issues/69` evidence.
- changed authority, port, project, IID, provider, source, state, empty body, and body over 512 KiB each fail before fallback.
- `status`, `release`, `complete`, non-reseed `replace`, reconcile preview, and
  non-`worktree_create` reconcile reject supplied evidence.

- [ ] **Step 2: Run the new core test and confirm RED**

Run:

```bash
go test ./internal/core/issueops -run 'ExecutionIssueSnapshot' -count=1
```

Expected: evidence DTO/source field/validator가 없어 compile FAIL.

- [ ] **Step 3: Add the port DTO and result source fields**

`port.ExecutionIssueSnapshot`에 다음 필드를 추가한다.

```go
Source string `json:"source,omitempty"`
```

`ExecutionActionRequest`에는 pointer evidence를 추가한다.

```go
IssueSnapshot *port.ExecutionIssueSnapshotEvidence `json:"issue_snapshot,omitempty"`
```

다음 결과 구조체에 같은 JSON 필드를 추가한다.

```go
IssueSnapshotSource string `json:"issue_snapshot_source,omitempty"`
```

대상은 `ExecutionPrepareResult`, `ExecutionResult`, `ExecutionReplaceResult`,
`ExecutionReconcileResult`다.

- [ ] **Step 4: Implement one core validation boundary**

`execution_issue_snapshot.go`에서:

1. Evidence가 있으면 허용 action인지 먼저 확인한다.
2. `ReadIssueOps`로 exact record provider와 linked URL을 읽는다.
3. `provider=gitlab`, `source=glab_mcp`를 exact match한다.
4. Private `parseGitLabIssueSnapshotIdentity`가 canonical HTTPS URL을
   `authority`(explicit port 포함), escaped project path, IID로 분해하고,
   authority는 host case만 정규화한 뒤 port/project/IID를 exact compare한다.
   마지막 resource segment만 `issues|work_items`를 동등 취급한다.
5. canonical HTTPS, no userinfo/query/fragment/control, non-empty <= 512 KiB body,
   `opened|closed` state를 검증한다.
6. Valid evidence reader는 fallback을 호출하지 않고, semantic identity 검증 뒤
   returned `ExecutionIssueSnapshot.URL`을 record의 linked URL로 정규화해 기존
   owner-packet exact string fence와 호환한다.
7. Evidence가 없으면 fallback을 감싸고 GitLab 성공 결과를 같은 validator에 통과시킨
   뒤 `Source="glab_cli"`를 붙인다.
8. GitLab fallback 오류는 `gitlab_issue_snapshot_unavailable` code text로 감싼다.

`ExecuteExecution`은 reader 호출을 감싸 실제 관찰 source를 보존한다. Supplied
evidence는 dispatch 전에 이미 검증됐으므로 preview에서도 `glab_mcp`를 결과에
표시한다.

```go
reader, snapshotSource, err := executionIssueSnapshotReader(stateRoot, req, deps.ReadIssue)
if err != nil {
	return nil, err
}
deps.ReadIssue = reader
result, err := executeExecutionAction(ctx, stateRoot, req, deps)
if err != nil {
	return nil, err
}
return withExecutionIssueSnapshotSource(result, snapshotSource()), nil
```

기존 switch body는 private `executeExecutionAction`으로 이동해 validation과 dispatch를
분리한다. Getter는 injected evidence면 처음부터 `glab_mcp`를 반환하고, fallback
reader가 성공하면 closure 내부 값을 `glab_cli`로 바꿔 action 종료 후 관찰값을
반환한다.

- [ ] **Step 5: Set CLI adapter source and verify fallback**

GitLab provider 성공 return을 다음처럼 바꾼다.

```go
return port.ExecutionIssueSnapshot{
	URL: issueURL, Body: payload.Description, State: payload.State, Source: "glab_cli",
}, nil
```

Provider test는 exact URL, work-item equivalence, wrong port/project/IID rejection,
`Source == "glab_cli"`를 검증한다.

- [ ] **Step 6: Run core and GitLab adapter tests**

Run:

```bash
go test ./internal/core/issueops ./internal/adapter/provider/gitlab -run 'ExecutionIssueSnapshot|IssueSnapshot' -count=1
```

Expected: PASS, invalid evidence에서 fallback call count는 0.

- [ ] **Step 7: Commit the core evidence boundary**

```bash
git add internal/port/execution_workspace.go internal/core/issueops internal/adapter/provider/gitlab
git commit -m "feat(issueops): accept validated GitLab MCP snapshots"
```

### Task 3: MCP nested snapshot schema and adapter parity

**Files:**

- Modify: `internal/adapter/mcp/issueops_catalog.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_execution.go`
- Modify: `cmd/harness/mcpcli/mcp_tool_issueops_execution_test.go`
- Modify: `internal/adapter/mcp/issueops_catalog_test.go`
- Update: `cmd/harness/testdata/mcp_tools.golden.json`
- Update: `cmd/harness/testdata/response_contracts.golden.json`

**Interfaces:**

- Consumes: `port.ExecutionIssueSnapshotEvidence`
- Produces: nested MCP `issue_snapshot` object with required
  `provider`, `source`, `web_url`, `body`, `state`
- Produces: `executionIssueSnapshotFromMCP(args map[string]any) (*port.ExecutionIssueSnapshotEvidence, error)`

- [ ] **Step 1: Write the failing schema and request mapping tests**

```go
snapshot, ok := properties["issue_snapshot"].(map[string]any)
if !ok {
	t.Fatalf("issue_snapshot schema = %#v", properties["issue_snapshot"])
}
if got := snapshot["required"]; !reflect.DeepEqual(got, []string{"provider", "source", "web_url", "body", "state"}) {
	t.Fatalf("issue_snapshot required = %#v", got)
}
```

Adapter test:

```go
req, err := executionActionRequestFromMCPWithAncestry(map[string]any{
	"action": "prepare",
	"id":     "io-aaaaaaaaaaaa",
	"issue_snapshot": map[string]any{
		"provider": "gitlab",
		"source":   "glab_mcp",
		"web_url":  "https://gitlab.example.com/acme/repo/-/issues/69",
		"body":     "AC-69",
		"state":    "opened",
	},
}, nil)
if err != nil || req.IssueSnapshot == nil || req.IssueSnapshot.Source != "glab_mcp" {
	t.Fatalf("nested snapshot mapping failed: req=%#v err=%v", req, err)
}
```

Wrong object type must return an error instead of silently omitting evidence.

- [ ] **Step 2: Run MCP tests and confirm RED**

Run:

```bash
go test ./internal/adapter/mcp ./cmd/harness/mcpcli -run 'IssueOps.*Snapshot|ExecutionActionRequestFromMCP' -count=1
```

Expected: schema property와 parser가 없어 FAIL.

- [ ] **Step 3: Add the closed nested schema**

`issueOpsExecutionSchema` properties에 다음 객체를 추가한다.

```go
"issue_snapshot": map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"provider", "source", "web_url", "body", "state"},
	"properties": map[string]any{
		"provider": map[string]any{"type": "string", "enum": []string{"gitlab"}},
		"source":   map[string]any{"type": "string", "enum": []string{"glab_mcp"}},
		"web_url":  map[string]any{"type": "string"},
		"body":     map[string]any{"type": "string", "maxLength": 524288},
		"state":    map[string]any{"type": "string", "enum": []string{"opened", "closed"}},
	},
},
```

- [ ] **Step 4: Parse nested evidence without silent fallback**

`executionActionRequestFromMCPWithAncestry` 반환을
`(issueops.ExecutionActionRequest, error)`로 바꾸고, present-but-not-object와
non-string field를 error로 반환한다. Handler는 parse error를
`mcpToolErrorPayload(issueOpsMCPErrorPayload(err))`로 응답한다.

- [ ] **Step 5: Run MCP package and golden tests**

Run:

```bash
go test ./internal/adapter/mcp ./cmd/harness/mcpcli -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
```

If golden tests report only the intentional nested property/result-field drift, run:

```bash
go test ./cmd/harness/contractgolden -run Golden -update -count=1
go test ./cmd/harness/harnessapp -run Golden -update -count=1
git diff -- cmd/harness/testdata/mcp_tools.golden.json cmd/harness/testdata/response_contracts.golden.json
```

Then rerun the three non-update test commands and reject any unrelated golden
change.

- [ ] **Step 6: Commit the MCP surface**

```bash
git add internal/adapter/mcp/issueops_catalog.go cmd/harness/mcpcli cmd/harness/testdata
git commit -m "feat(mcp): expose GitLab issue snapshot evidence"
```

### Task 4: Secure CLI snapshot file ingestion

**Files:**

- Modify: `cmd/harness/issueopscli/executioncmd/execution.go`
- Create: `cmd/harness/issueopscli/executioncmd/snapshot_file.go`
- Create: `cmd/harness/issueopscli/executioncmd/snapshot_file_test.go`
- Modify: `cmd/harness/issueopscli/issueops_execution_cli_test.go`
- Update: `cmd/harness/testdata/usage.golden.txt`

**Interfaces:**

- Consumes: the same JSON shape as MCP `issue_snapshot`
- Produces: `readExecutionIssueSnapshotFile(path string) (*port.ExecutionIssueSnapshotEvidence, error)`
- Adds: `--issue-snapshot-file PATH` to prepare, claim, replace, and reconcile

- [ ] **Step 1: Write failing secure-file tests**

Use one valid `0600` regular file and table cases for:

- symlink
- directory/non-regular
- permission `0644`
- payload over 1 MiB
- invalid JSON
- unknown JSON field
- trailing second JSON value

```go
got, err := readExecutionIssueSnapshotFile(path)
if err != nil || got == nil || got.Source != "glab_mcp" {
	t.Fatalf("valid snapshot file failed: got=%#v err=%v", got, err)
}
```

- [ ] **Step 2: Run CLI helper tests and confirm RED**

Run:

```bash
go test ./cmd/harness/issueopscli/executioncmd -run 'SnapshotFile' -count=1
```

Expected: helper가 없어 compile FAIL.

- [ ] **Step 3: Implement bounded race-aware file reading**

`readExecutionIssueSnapshotFile`은:

1. 빈 path면 `nil, nil`.
2. `os.Lstat` 결과가 symlink가 아닌 regular file이고 exact mode `0600`인지 확인.
3. `os.Open` 후 `file.Stat`과 `os.SameFile`로 Lstat/Open 사이 교체를 거부.
4. `io.LimitReader(file, 1<<20+1)`로 읽어 1 MiB 초과를 거부.
5. `json.Decoder.DisallowUnknownFields()`로 exact DTO를 decode.
6. 두 번째 decode가 `io.EOF`인지 확인해 trailing value를 거부.
7. Core가 provider/source/URL/body/state semantic validation을 최종 수행.

- [ ] **Step 4: Wire the flag into snapshot-capable actions**

각 subcommand에서 parse 후 helper를 호출하고 다음 필드로 전달한다.

```go
IssueSnapshot: issueSnapshot,
```

`replace`는 모든 replace action에서 flag를 parse하되 core가 `reseed` 이외 사용을
fail-closed한다. `status`, `release`, `complete`에는 flag를 추가하지 않는다.

- [ ] **Step 5: Run focused CLI parity tests**

Run:

```bash
go test ./cmd/harness/issueopscli/executioncmd ./cmd/harness/issueopscli -run 'Snapshot|ExecutionCLI' -count=1
```

Expected: valid file maps to the same evidence DTO and unsafe files fail before provider CLI execution.

- [ ] **Step 6: Commit the CLI surface**

```bash
git add cmd/harness/issueopscli/executioncmd cmd/harness/issueopscli/issueops_execution_cli_test.go cmd/harness/testdata/usage.golden.txt
git commit -m "feat(cli): read bounded IssueOps snapshots"
```

### Task 5: Shared skill/docs contract and provider-neutral recipe guidance

**Files:**

- Modify: `skills/gitlab-usecase/SKILL.md`
- Modify: `skills/issueops/references/execution.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/AGENT_WORKFLOW.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `docs/superpowers/specs/2026-07-28-gitlab-mcp-cli-snapshot-design.md`
- Modify: `docs/superpowers/plans/2026-07-28-gitlab-mcp-cli-vcs-recipe.md`
- Test: `internal/core/skillcontract/skill_contract_test.go`

**Interfaces:**

- Documents: capability-first `glab_api` discovery, exact endpoint/input/response,
  invalid-evidence fail-closed, CLI fallback, VCS recipe read/record
- Preserves: wrapper use by semantic capability while excluding wrapper identity

- [ ] **Step 1: Write the failing skill contract assertions**

Add exact assertions that the shared skills contain:

```go
for _, want := range []string{
	"glab_api",
	"server namespace",
	"issue_snapshot",
	"--issue-snapshot-file",
	".agent-harness/VCS.md",
	"project_docs_read",
	"project_docs_update",
	"glab api",
} {
	if !strings.Contains(content, want) {
		t.Fatalf("portable GitLab snapshot contract missing %q", want)
	}
}
```

Also assert the files do not contain `/Users/habin`, `glab-mcp-wrapper` path,
profile names, or token variable names as required behavior.

- [ ] **Step 2: Run skill contract tests and confirm RED**

Run:

```bash
go test ./internal/core/skillcontract -run 'GitLab|IssueOps' -count=1
```

Expected: new snapshot/VCS recipe language is missing.

- [ ] **Step 3: Update the shared instructions**

`gitlab-usecase`에 다음 deterministic sequence를 document한다.

1. Current repo의 `.agent-harness/VCS.md`를 읽는다.
2. Trusted host catalog에서 leaf `glab_api`를 찾고 server namespace를 선택 기준으로
   쓰지 않는다.
3. `projects/<escaped-project>/issues/<iid>`와 `flags.hostname`으로 bounded read한다.
4. `web_url`, `description`, `state`와 exact identity를 확인한다.
5. `issue_snapshot` evidence를 MCP에 전달하거나 private JSON file을 CLI flag로
   전달한다.
6. 후보 부재나 호출 실패 뒤에도 successful exact-identity evidence가 없을 때만
   `glab api` CLI fallback을 쓴다. 이미 공급한 invalid evidence는 fallback하지
   않는다.
7. Successful recipe가 새로 검증됐으면 canonical worktree에서 VCS.md를 SHA-CAS
   update한다.

GitHub 항목은 exact `gh issue view <url> --json url,body,state` recipe와 “실제
검증한 MCP schema만 추가” 규칙만 기록한다.

- [ ] **Step 4: Update project operating docs**

`OPERATIONS.md`에는 installed MCP/CLI smoke, `AGENT_WORKFLOW.md`에는 successful
capability 후 VCS recipe 기록, `TESTING.md`에는 먼저 실행할 targeted verification
set만 기록한다. 사용자 full-test waiver는 이 plan/spec에만 유지하고 전역 testing
policy를 바꾸지 않는다. OpenWiki 자동 update 문구나 실행 step을 추가하지 않는다.

- [ ] **Step 5: Run skill and docs tests**

Run:

```bash
python3 scripts/validate-skill.py skills/gitlab-usecase
python3 scripts/validate-skill.py skills/issueops
go test ./internal/core/skillcontract -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the host-neutral workflow**

```bash
git add skills/gitlab-usecase skills/issueops/references/execution.md .agent-harness docs/superpowers
git commit -m "docs(issueops): document portable VCS snapshots"
```

### Task 6: Targeted verification, install refresh, live GitLab smoke, and atomic push

**Files:**

- Verify only; no planned source edits

**Interfaces:**

- Verifies: changed package behavior, race/vet/build, CLI/MCP contracts, installed
  Codex/Claude schema, current generic `glab_api`, #2609 Orca preview

- [ ] **Step 1: Run targeted package tests**

```bash
go test ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/hookprompt ./internal/core/projectdoc ./internal/core/projectdocs ./internal/core/projectbootstrap ./internal/core/doctor -count=1
go test ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli ./cmd/harness/hookcli ./internal/adapter/mcp ./internal/core/skillcontract -count=1
```

Expected: PASS.

- [ ] **Step 2: Run bounded race and vet**

```bash
go test -race ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/projectdoc ./internal/core/projectdocs -count=1
go vet ./internal/core/issueops ./internal/adapter/provider/gitlab ./internal/core/hookprompt ./internal/core/projectdoc ./internal/core/projectdocs ./cmd/harness/issueopscli/... ./cmd/harness/mcpcli ./cmd/harness/hookcli ./internal/adapter/mcp
```

Expected: PASS. Do not expand either command to `./...`.

- [ ] **Step 3: Run build and contract golden**

```bash
go build -o bin/agent-harness ./cmd/harness
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
git diff --check
```

Expected: PASS and no whitespace error.

- [ ] **Step 4: Refresh installed hosts**

```bash
ah update --json
./bin/agent-harness daemon status --json
codex mcp get agent_harness
claude mcp list
```

Restart or refresh the daemon only through the repository’s existing update/status
contract. Do not run OpenWiki update.

- [ ] **Step 5: Verify installed MCP schema**

Read the installed `issueops_execution` tool catalog from both Codex and Claude
surfaces and assert the nested object exposes exactly:

```text
provider, source, web_url, body, state
```

No field may expose server namespace, wrapper path, profile, token, or config.

- [ ] **Step 6: Run live generic capability smoke**

From `/Users/habin/workspace/api-servers`, use the currently exposed trusted
`glab_api` leaf that successfully reads GitLab project `bubble-team/backend-team/api-servers`
issue IID `2609`. Verify response `web_url`, project, IID, description, and state,
then pass only the normalized evidence to:

```text
issueops_execution action=prepare id=io-92a6dbd2d761 mode=auto confirm=false
```

Expected:

```text
resolved_mode=orca
issue_snapshot_source=glab_mcp
```

Do not create the #2609 worktree or claim the lease in this smoke.

- [ ] **Step 7: Record the successful recipe only in the authorized worktree**

If #2609 still has no active canonical holder worktree, do not write
`api-servers/.agent-harness/VCS.md` from its source checkout. Revalidate and record
the recipe when the #2609 worktree is created. For the current `agent-harness`
repo, create/update `.agent-harness/VCS.md` only if this task actually performed a
successful GitHub provider read.

- [ ] **Step 8: Run atomic commit/push preflight**

Use `$atomic-commit-push` to inspect:

```bash
git status --short --branch
git diff --check
git log --oneline --decorate -8
git rev-list --left-right --count origin/main...main
```

Stage only this feature’s files, preserve unrelated changes, and ensure every
commit uses the repository’s Conventional Commit subject plus Lore body.

- [ ] **Step 9: Push and verify remote synchronization**

```bash
git push origin main
git rev-list --left-right --count origin/main...main
git status --short --branch
```

Expected: divergence `0 0`, clean worktree, and no OpenWiki auto-update.

## Out-of-scope handoff

사용자가 요청한 후속 hexagonal architecture refactor는 이 snapshot/VCS feature
plan에서 구현하지 않는다. Feature 검증·push가 끝난 뒤 별도
`117-hexagonal-architecture-migration` lifecycle과 그 자체의 plan/lease를 다시
읽고 독립적으로 재개한다. 이 문서의 완료 판정에는 해당 refactor 변경을 포함하지
않는다.

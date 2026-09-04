# IssueOps devil's-advocate fail-closed loop — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the design-review devil's-advocate an enforced IssueOps gate whose `stop` findings are written back to the remote issue before the cycle regresses to re-plan.

**Architecture:** Add a first-class `IssueOpsDevilsAdvocateReview` record + implement-entry readiness gate (mirroring `compatibilityreview`), a new `UpdateIssueBodySection` provider method that idempotently splices a delimited section into the issue body, a `reflect-devils-advocate` command that stamps `IssueReflectedAt`, and a regress precondition tying it together. Everything follows the existing compatibility-review and provider adapter patterns.

**Tech Stack:** Go 1.26, stdlib `flag`, `gh`/`glab` CLIs via `os/exec`, golden-file contract tests.

## Global Constraints

- Go module `issueops`; run all commands from repo root.
- gofmt clean (`gofmt -l $(git ls-files '*.go')` empty) + `go build ./...` + `go vet` before every commit.
- Commit messages: Conventional Commit subject; do NOT put `go build`/`go vet`/`go test ./...` slash-literals in the message body (say "targeted package tests" etc.) — avoids the `.ckignore` scout-block.
- Golden regen only via `-update`; the diff must be intentional-only (CAUTIONS §27).
- Freeform user text is redacted via `policy.RedactFreeform` (see `compatibilityreview.Validate`).
- No `--json` is ever passed to `gh/glab` *create*; issue-body reads use `gh issue view --json body` / `glab api` (CAUTIONS §25).
- Provider mutations are `Confirm`-gated with a dry-run `Preview` when `Confirm=false`.

---

## File Structure

**Create:**
- `internal/core/issueops/devilsadvocate/devils_advocate.go` — core recorder + validation (mirror `compatibilityreview/compatibility_review.go`).
- `internal/core/issueops/devilsadvocate/devils_advocate_test.go`
- `internal/core/issueops/issuebodysection.go` — pure delimited-section splice helper (shared by both adapters via a small exported func in a neutral package; see Task 6).
- `cmd/issueops/issueopscli/issueops_devilsadvocate_cli.go` — CLI `issueops devils-advocate review` (mirror `issueops_compatibility_cli.go`).
- `cmd/issueops/issueopscli/issueops_reflect_cli.go` — CLI `issueops remote reflect-devils-advocate` (or fold into `remotecmd`; see Task 9).

**Modify:**
- `internal/core/issueops/model/types.go` — new record type + `DevilsAdvocateReview` field.
- `internal/core/issueops/issueops_readiness.go` — gate.
- `internal/core/issueops/issueops_facade.go` (or `package.go`) — re-export recorder + reflect.
- `internal/core/issueops/issueops_regress.go` — precondition + clear-on-regress.
- `internal/port/provider.go` — `UpdateIssueBodySection` + request/result types on `IssueProvider`.
- `internal/adapter/provider/github/provider.go` + `_test.go`
- `internal/adapter/provider/gitlab/provider.go` + `_test.go`
- `cmd/issueops/issueopscli/issueops.go` — register `devils-advocate` subcommand.
- `cmd/issueops/mcpcli/mcp_tool_issueops.go` + `mcp_tool_issueops_handlers.go` — 2 handlers.
- `internal/adapter/mcp/issueops_catalog.go` — 2 catalog entries.
- `cmd/issueops/testdata/mcp_tools.golden.json`, `cmd/issueops/testdata/response_contracts.golden.json`
- `skills/issueops/SKILL.md`, `.issueops/ADR.md`

---

## Task 1: Core record + recorder + validation

**Files:**
- Modify: `internal/core/issueops/model/types.go`
- Create: `internal/core/issueops/devilsadvocate/devils_advocate.go`
- Create: `internal/core/issueops/devilsadvocate/devils_advocate_test.go`
- Modify: `internal/core/issueops/issueops_facade.go` (re-export)

**Interfaces:**
- Produces: `model.IssueOpsDevilsAdvocateReview{Verdict, Findings, Waived, WaiverRationale, ReviewerPattern, RecordedAt, IssueReflectedAt}`, `model.IssueOpsDevilsAdvocateReviewRequest{Verdict, Findings, Waived, WaiverRationale}`, record field `DevilsAdvocateReview *IssueOpsDevilsAdvocateReview`, `devilsadvocate.Record(store, stateRoot, id, req)`, `devilsadvocate.Validate(req)`, facade `core.RecordIssueOpsDevilsAdvocateReview(stateRoot, id, req)`.

- [ ] **Step 1: Write the failing test** in `devils_advocate_test.go`:

```go
package devilsadvocate

import (
	"testing"

	"issueops/internal/core/issueops/model"
)

func TestValidateVerdicts(t *testing.T) {
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"}); err != nil {
		t.Fatalf("pass should validate: %v", err)
	}
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "bogus"}); err == nil {
		t.Fatal("unknown verdict must fail")
	}
	// stop needs findings OR waiver
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "stop"}); err == nil {
		t.Fatal("stop without findings/waiver must fail")
	}
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "stop", Findings: []string{"gold-plating"}}); err != nil {
		t.Fatalf("stop with findings should validate: %v", err)
	}
	// waiver needs rationale
	if _, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "revise", Waived: true}); err == nil {
		t.Fatal("waive without rationale must fail")
	}
	got, err := Validate(model.IssueOpsDevilsAdvocateReviewRequest{Verdict: "revise", Waived: true, WaiverRationale: "scoped follow-up issue filed"})
	if err != nil || !got.Waived || got.ReviewerPattern != "devils-advocate-review" {
		t.Fatalf("waived revise should validate with reviewer pattern: %+v %v", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — Run: `go test ./internal/core/issueops/devilsadvocate/ -run TestValidateVerdicts` — Expected: build failure (types/pkg undefined).

- [ ] **Step 3: Add the model types** to `model/types.go` (place beside `IssueOpsCompatibilityReview`):

```go
type IssueOpsDevilsAdvocateReview struct {
	Verdict          string   `json:"verdict"` // pass | revise | stop
	Findings         []string `json:"findings,omitempty"`
	Waived           bool     `json:"waived,omitempty"`
	WaiverRationale  string   `json:"waiver_rationale,omitempty"`
	ReviewerPattern  string   `json:"reviewer_pattern,omitempty"`
	RecordedAt       string   `json:"recorded_at"`
	IssueReflectedAt string   `json:"issue_reflected_at,omitempty"`
}

type IssueOpsDevilsAdvocateReviewRequest struct {
	Verdict         string
	Findings        []string
	Waived          bool
	WaiverRationale string
}
```

Add the field to `IssueOpsRecord` (beside `CompatibilityReview *IssueOpsCompatibilityReview`):

```go
DevilsAdvocateReview *IssueOpsDevilsAdvocateReview `json:"devils_advocate_review,omitempty"`
```

- [ ] **Step 4: Write the recorder** `devils_advocate.go` (mirror `compatibilityreview/compatibility_review.go`):

```go
package devilsadvocate

import (
	"fmt"
	"strings"
	"time"

	"issueops/internal/core/issueops/model"
	"issueops/internal/core/policy"
)

type Store struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

var validVerdicts = map[string]bool{"pass": true, "revise": true, "stop": true}

func Record(store Store, stateRoot, id string, req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	review, err := Validate(req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.DevilsAdvocateReview = &review
	return store.TouchWrite(stateRoot, record)
}

func Validate(req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsDevilsAdvocateReview, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if !validVerdicts[verdict] {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("verdict must be pass, revise, or stop")
	}
	findings := cleanList(req.Findings)
	rationale := strings.TrimSpace(req.WaiverRationale)
	if req.Waived && rationale == "" {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("waived review requires waiver_rationale")
	}
	if (verdict == "stop" || verdict == "revise") && !req.Waived && len(findings) == 0 {
		return model.IssueOpsDevilsAdvocateReview{}, fmt.Errorf("%s verdict requires findings or an explicit waiver", verdict)
	}
	return model.IssueOpsDevilsAdvocateReview{
		Verdict:         verdict,
		Findings:        findings,
		Waived:          req.Waived,
		WaiverRationale: policy.RedactFreeform(rationale),
		ReviewerPattern: "devils-advocate-review",
		RecordedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		v = policy.RedactFreeform(v)
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
```

- [ ] **Step 5: Add the facade re-export** in `internal/core/issueops/issueops_facade.go` (mirror `RecordIssueOpsCompatibilityReview`): a `type IssueOpsDevilsAdvocateReviewRequest = ...` alias and `func RecordIssueOpsDevilsAdvocateReview(stateRoot, id string, req IssueOpsDevilsAdvocateReviewRequest) (IssueOpsRecord, error)` that builds a `devilsadvocate.Store{Read: ReadIssueOps, TouchWrite: Write…}` and calls `devilsadvocate.Record`. Find the compatibility-review facade func and mirror it exactly.

- [ ] **Step 6: Run tests + build** — Run: `go test ./internal/core/issueops/devilsadvocate/ ./internal/core/issueops/ -count=1` and `go build ./...` — Expected: PASS, build clean.

- [ ] **Step 7: Commit** — `git add` the four files; `git commit -m "feat(issueops): add devil's-advocate review record and recorder"`.

---

## Task 2: Implement-entry fail-closed gate

**Files:**
- Modify: `internal/core/issueops/issueops_readiness.go`
- Modify: `internal/core/issueops/issueops_readiness_test.go` (or the existing readiness test file)

**Interfaces:**
- Consumes: `record.DevilsAdvocateReview` (Task 1).
- Produces: missing key `devils_advocate_review` in `IssueOpsImplementationReadiness`.

- [ ] **Step 1: Write the failing test** (add to the readiness test file):

```go
func TestImplementationReadinessRequiresDevilsAdvocate(t *testing.T) {
	base := readyImplementRecordFixture(t) // a record otherwise implement-ready (reuse existing helper)
	if got := IssueOpsImplementationReadiness(base); containsString(got.Missing, "devils_advocate_review") == false {
		t.Fatalf("missing devil's-advocate review must block implement: %+v", got.Missing)
	}
	base.DevilsAdvocateReview = &IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "t"}
	if got := IssueOpsImplementationReadiness(base); containsString(got.Missing, "devils_advocate_review") {
		t.Fatalf("pass verdict should clear the gate: %+v", got.Missing)
	}
	base.DevilsAdvocateReview = &IssueOpsDevilsAdvocateReview{Verdict: "stop", Findings: []string{"x"}, RecordedAt: "t"}
	if got := IssueOpsImplementationReadiness(base); !containsString(got.Missing, "devils_advocate_review") {
		t.Fatalf("unwaived stop must block: %+v", got.Missing)
	}
	base.DevilsAdvocateReview.Waived = true
	base.DevilsAdvocateReview.WaiverRationale = "filed follow-up"
	if got := IssueOpsImplementationReadiness(base); containsString(got.Missing, "devils_advocate_review") {
		t.Fatalf("waived stop should clear the gate: %+v", got.Missing)
	}
}
```

> Reuse whatever fully-implement-ready fixture the existing readiness tests use; if none is exported, build the record inline the way `TestCompatibilityReview*` does. `containsString` = existing slice helper (or `slices.Contains`).

- [ ] **Step 2: Run test to verify it fails** — Run: `go test ./internal/core/issueops/ -run TestImplementationReadinessRequiresDevilsAdvocate` — Expected: FAIL (key not added yet).

- [ ] **Step 3: Add the gate** to `issueops_readiness.go`. In `IssueOpsImplementationReadiness`, after the compatibility-review append, add:

```go
	missing = append(missing, issueOpsDevilsAdvocateReviewMissing(record)...)
```

and define:

```go
func issueOpsDevilsAdvocateReviewMissing(record IssueOpsRecord) []string {
	review := record.DevilsAdvocateReview
	if review == nil || strings.TrimSpace(review.RecordedAt) == "" {
		return []string{"devils_advocate_review"}
	}
	if (review.Verdict == "stop" || review.Verdict == "revise") && !review.Waived {
		return []string{"devils_advocate_review"}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes** — Run: `go test ./internal/core/issueops/ -run TestImplementationReadinessRequiresDevilsAdvocate` — Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(issueops): gate implement entry on a devil's-advocate verdict"`.

---

## Task 3: CLI recorder — `issueops devils-advocate review`

**Files:**
- Create: `cmd/issueops/issueopscli/issueops_devilsadvocate_cli.go`
- Modify: `cmd/issueops/issueopscli/issueops.go` (register `"devils-advocate": runIssueOpsDevilsAdvocate`)
- Modify: the CLI test file that drives issueops subcommands (mirror the compatibility CLI test)

**Interfaces:**
- Consumes: `core.RecordIssueOpsDevilsAdvocateReview` (Task 1 facade).

- [ ] **Step 1: Write the failing test** — a CLI test that starts a cycle, calls `runIssueOpsDevilsAdvocate([]string{"review","--id",id,"--verdict","pass"})`, and asserts the record's `DevilsAdvocateReview.Verdict=="pass"`. Mirror the compatibility CLI test structure.

- [ ] **Step 2: Run to verify it fails** — Run: `go test ./cmd/issueops/issueopscli/ -run DevilsAdvocate` — Expected: FAIL (undefined func).

- [ ] **Step 3: Write the CLI** (mirror `issueops_compatibility_cli.go`):

```go
package issueopscli

import (
	"flag"
	"fmt"

	"issueops/internal/core"
)

func runIssueOpsDevilsAdvocate(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: issueops devils-advocate review --id ID --verdict pass|revise|stop [--finding TEXT]... [--waive --waiver-rationale TEXT] [--json]")
		return nil
	}
	if args[0] != "review" {
		return fmt.Errorf("unknown issueops devils-advocate subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops devils-advocate review", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	verdict := fs.String("verdict", "", "pass|revise|stop")
	waive := fs.Bool("waive", false, "explicitly waive a stop/revise verdict")
	rationale := fs.String("waiver-rationale", "", "rationale required when --waive is set")
	jsonOut := fs.Bool("json", false, "print JSON")
	var findings sliceFlag
	fs.Var(&findings, "finding", "surfaced problem (repeatable)")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.RecordIssueOpsDevilsAdvocateReview(core.IssueOpsStateRoot(), *id, core.IssueOpsDevilsAdvocateReviewRequest{
		Verdict:         *verdict,
		Findings:        findings,
		Waived:          *waive,
		WaiverRationale: *rationale,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}
```

Register in `issueops.go` subcommand map: `"devils-advocate": runIssueOpsDevilsAdvocate,`. Also add a `devils-advocate` entry to `suggestIssueOpsSubcommand` if it has an explicit list.

- [ ] **Step 4: Run tests + build** — Run: `go test ./cmd/issueops/issueopscli/ -run DevilsAdvocate` and `go build ./...` — Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(issueopscli): add devils-advocate review subcommand"`.

---

## Task 4: MCP recorder tool

**Files:**
- Modify: `cmd/issueops/mcpcli/mcp_tool_issueops.go` (registry) + `mcp_tool_issueops_handlers.go` (handler)
- Modify: `internal/adapter/mcp/issueops_catalog.go` (catalog entry)
- Modify: `cmd/issueops/testdata/mcp_tools.golden.json` (via -update)

**Interfaces:**
- Consumes: `core.RecordIssueOpsDevilsAdvocateReview`.
- Produces: MCP tool `issueops_record_devils_advocate_review`.

- [ ] **Step 1: Write the failing test** — in the `mcpcli/issueops` black-box test package, drive `issueops_record_devils_advocate_review` with `{id, verdict:"pass"}` and assert the returned record has the verdict. Mirror an existing `handleMCPIssueOpsRecordCompatibilityReview` test.

- [ ] **Step 2: Run to verify it fails** — Run: `go test ./cmd/issueops/mcpcli/issueops/ -run DevilsAdvocate` — Expected: FAIL (tool not registered).

- [ ] **Step 3: Add the handler** in `mcp_tool_issueops_handlers.go` (mirror `handleMCPIssueOpsRecordCompatibilityReview`):

```go
func handleMCPIssueOpsRecordDevilsAdvocateReview(args map[string]any) MCPToolOutcome {
	record, err := core.RecordIssueOpsDevilsAdvocateReview(core.IssueOpsStateRoot(), argmap.String(args, "id"), core.IssueOpsDevilsAdvocateReviewRequest{
		Verdict:         argmap.String(args, "verdict"),
		Findings:        argmap.StringSlice(args, "findings"),
		Waived:          argmap.Bool(args, "waived"),
		WaiverRationale: argmap.String(args, "waiver_rationale"),
	})
	return issueOpsMCPOutcome(record, err, "IssueOps record devils-advocate review failed")
}
```

Register in the `issueOpsMCPHandlers` map: `"issueops_record_devils_advocate_review": handleMCPIssueOpsRecordDevilsAdvocateReview,`. Add a catalog entry in `issueops_catalog.go` mirroring `issueops_record_compatibility_review` (Name, Description, input schema: `id`, `verdict`, `findings[]`, `waived`, `waiver_rationale`).

- [ ] **Step 4: Regenerate the MCP tools golden** — Run: `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`, then `git --no-pager diff cmd/issueops/testdata/mcp_tools.golden.json` and confirm the diff is ONLY the new tool entry. Run the golden test without `-update` to confirm PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(mcp): add issueops_record_devils_advocate_review tool"`.

---

## Task 5: Section-splice helper (pure, idempotent)

**Files:**
- Create: `internal/core/issueops/remoteartifact/issue_body_section.go` (or reuse an existing neutral pkg both adapters already import — `remoteparse` is imported by gitlab; github does not. Put it in `internal/port` companion or a new tiny `internal/core/issueops/issuebody` pkg imported by both adapters.)
- Create: its `_test.go`

**Interfaces:**
- Produces: `MergeIssueBodySection(body, sectionMarkdown string) string` — replaces the content between `<!-- issueops:devils-advocate:start -->` and `<!-- issueops:devils-advocate:end -->`, or appends the delimited block if absent. `RenderDevilsAdvocateSection(findings []string, ts string) string` — builds the delimited markdown.

- [ ] **Step 1: Write the failing test**:

```go
func TestMergeIssueBodySectionIdempotent(t *testing.T) {
	sec := RenderDevilsAdvocateSection([]string{"gold-plating", "schedule optimism"}, "2026-07-01T00:00:00Z")
	body := "original body\n"
	once := MergeIssueBodySection(body, sec)
	if !strings.Contains(once, "gold-plating") || !strings.HasPrefix(once, "original body") {
		t.Fatalf("append failed: %q", once)
	}
	sec2 := RenderDevilsAdvocateSection([]string{"new finding"}, "2026-07-02T00:00:00Z")
	twice := MergeIssueBodySection(once, sec2)
	if strings.Count(twice, "issueops:devils-advocate:start") != 1 {
		t.Fatalf("re-merge must not duplicate the block: %q", twice)
	}
	if strings.Contains(twice, "gold-plating") || !strings.Contains(twice, "new finding") {
		t.Fatalf("re-merge must replace the block content: %q", twice)
	}
}
```

- [ ] **Step 2: Run to verify it fails** — Run: `go test ./internal/core/issueops/issuebody/ -run Idempotent` — Expected: FAIL (undefined).

- [ ] **Step 3: Implement** (delimiter-scoped splice; never touch content outside markers):

```go
package issuebody

import (
	"fmt"
	"strings"
)

const (
	startMarker = "<!-- issueops:devils-advocate:start -->"
	endMarker   = "<!-- issueops:devils-advocate:end -->"
)

func RenderDevilsAdvocateSection(findings []string, ts string) string {
	var b strings.Builder
	b.WriteString(startMarker + "\n")
	fmt.Fprintf(&b, "## Devil's-advocate findings (%s)\n", ts)
	for _, f := range findings {
		fmt.Fprintf(&b, "- %s\n", f)
	}
	b.WriteString(endMarker)
	return b.String()
}

func MergeIssueBodySection(body, section string) string {
	s := strings.Index(body, startMarker)
	e := strings.Index(body, endMarker)
	if s >= 0 && e > s {
		return body[:s] + section + body[e+len(endMarker):]
	}
	trimmed := strings.TrimRight(body, "\n")
	if trimmed == "" {
		return section + "\n"
	}
	return trimmed + "\n\n" + section + "\n"
}
```

- [ ] **Step 4: Run to verify it passes** — Run: `go test ./internal/core/issueops/issuebody/ -count=1` — Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(issueops): add idempotent issue-body section splice helper"`.

---

## Task 6: `UpdateIssueBodySection` on the provider interface + both adapters

**Files:**
- Modify: `internal/port/provider.go` (interface + request/result types)
- Modify: `internal/adapter/provider/github/provider.go` + `_test.go`
- Modify: `internal/adapter/provider/gitlab/provider.go` + `_test.go`

**Interfaces:**
- Consumes: `issuebody.MergeIssueBodySection`, `issuebody.RenderDevilsAdvocateSection`.
- Produces: `port.IssueProvider.UpdateIssueBodySection(req IssueProviderUpdateIssueBodySectionRequest) (IssueProviderUpdateIssueBodySectionResult, error)`.

- [ ] **Step 1: Add the port types** to `provider.go`:

```go
type IssueProviderUpdateIssueBodySectionRequest struct {
	Repo     string   `json:"repo"`
	IssueURL string   `json:"issue_url"`
	Findings []string `json:"findings"`
	Confirm  bool     `json:"confirm"`
}

type IssueProviderUpdateIssueBodySectionResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url,omitempty"`
	Updated bool   `json:"updated"`
	Preview string `json:"preview,omitempty"`
}
```

Add to the `IssueProvider` interface: `UpdateIssueBodySection(req IssueProviderUpdateIssueBodySectionRequest) (IssueProviderUpdateIssueBodySectionResult, error)`.

- [ ] **Step 2: Run build to verify it fails** — Run: `go build ./...` — Expected: FAIL (github/gitlab don't satisfy the interface).

- [ ] **Step 3: Write the failing GitHub adapter test** (mirror `TestGitHubCreateChildConfirm…` fake-gh pattern): a fake `gh` that answers `issue view --json body` with a body, then `issue edit … --body …`; assert the edited body contains the delimited section and the findings. Add a dry-run sub-test asserting `Preview` non-empty and no `issue edit` call.

- [ ] **Step 4: Implement GitHub** in `github/provider.go`. `--confirm=false` returns a `Preview`; otherwise:

```go
func (Provider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	section := issuebody.RenderDevilsAdvocateSection(req.Findings, time.Now().UTC().Format(time.RFC3339))
	if !req.Confirm {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: true, Preview: "[dry-run] would gh issue edit " + req.IssueURL + " with devil's-advocate section"}, nil
	}
	view, err := runGhAPIJSON[struct{ Body string `json:"body"` }](req.Repo, []string{"issue", "view", req.IssueURL, "--json", "body"}, "issue body read")
	// NOTE: gh issue view is not `gh api`; use runGhJSON-style exec instead. See Step 5.
	_ = view
	// ... build merged body, run `gh issue edit <url> --body <merged>` ...
	return port.IssueProviderUpdateIssueBodySectionResult{OK: true, URL: req.IssueURL, Updated: true}, nil
}
```

> Implementation note (resolve in Step 4, do not leave as a comment): `gh issue view … --json body` is a `gh` subcommand, not `gh api`. Add a small `runGh(args, repo)` exec helper (mirror `runGhJSON` without the create-error wrapping) that returns raw stdout, unmarshal `{"body":...}`, `issuebody.MergeIssueBodySection`, then `exec.Command("gh","issue","edit",url,"--body",merged)`. Pass `--repo owner/repo` derived via `parseGitHubIssueURL`.

- [ ] **Step 5: Write + implement GitLab** (mirror `fetchGitLabIssueArtifact` for the GET, then PUT). Fake `glab` test: `glab api projects/:proj/issues/:iid` returns `{"description":"..."}`; then `glab api --method PUT … -f description=<merged>`. Assert merged description contains the section. GitLab PUT replaces the whole description, so round-trip the untouched remainder via `MergeIssueBodySection`.

- [ ] **Step 6: Run tests + build** — Run: `go test ./internal/adapter/provider/... -count=1` and `go build ./...` — Expected: PASS.

- [ ] **Step 7: Commit** — `git commit -m "feat(provider): add UpdateIssueBodySection for github and gitlab"`.

---

## Task 7: Reflect command + `IssueReflectedAt` stamp

**Files:**
- Modify: `internal/core/issueops/issueops_feedback.go` or a new `internal/core/issueops/devilsadvocate` reflect func; add facade `core.ReflectIssueOpsDevilsAdvocateFindings(stateRoot, id string, confirm bool)`.
- Modify: `cmd/issueops/issueopscli/remotecmd/remote.go` (add `reflect-devils-advocate` subcommand) + `cmd/issueops/mcpcli` handler + catalog.

**Interfaces:**
- Consumes: `provider.Resolve`, `UpdateIssueBodySection`, `record.DevilsAdvocateReview.Findings`, `record.IssueURL`.
- Produces: on confirmed success, stamps `record.DevilsAdvocateReview.IssueReflectedAt` and returns the record.

- [ ] **Step 1: Write the failing test** — with a fake gh, run reflect (confirm=false) → no stamp, preview present; reflect (confirm=true) → `IssueReflectedAt != ""` and the issue body got the section. Reflect with no `DevilsAdvocateReview` or empty findings → error.

- [ ] **Step 2: Run to verify it fails** — Run: `go test ./cmd/issueops/issueopscli/remotecmd/ -run Reflect` — Expected: FAIL.

- [ ] **Step 3: Implement the core reflect** (locked read-modify-write, mirror `MarkIssueOpsContractFeedbackIssueUpdated`): read record; require `DevilsAdvocateReview != nil` with findings; resolve provider from `record`; call `UpdateIssueBodySection{Repo, IssueURL, Findings, Confirm}`; on confirmed `Updated`, set `DevilsAdvocateReview.IssueReflectedAt = now` and `TouchWrite`.

- [ ] **Step 4: Wire CLI + MCP** — CLI `issueops remote reflect-devils-advocate --id ID [--confirm] [--json]` in `remotecmd`; MCP `issueops_remote_reflect_devils_advocate` handler + catalog entry.

- [ ] **Step 5: Regenerate goldens** — Run: `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`; confirm `mcp_tools.golden.json` diff is only the new tool. Golden test PASS without `-update`.

- [ ] **Step 6: Run tests + build; Commit** — `git commit -m "feat(issueops): reflect devil's-advocate findings into the remote issue"`.

---

## Task 8: Regress precondition + clear-on-regress

**Files:**
- Modify: `internal/core/issueops/issueops_regress.go`
- Modify: `internal/core/issueops/issueops_regress_test.go`

**Interfaces:**
- Consumes: `record.DevilsAdvocateReview`.

- [ ] **Step 1: Write the failing test**: regress rejected when `DevilsAdvocateReview` is nil, or verdict != stop, or `IssueReflectedAt == ""`; accepted when verdict=stop AND `IssueReflectedAt != ""`; after a successful regress, `record.DevilsAdvocateReview == nil` (cleared) and `record.Phase == grill`.

- [ ] **Step 2: Run to verify it fails** — Run: `go test ./internal/core/issueops/ -run Regress` — Expected: FAIL (new assertions).

- [ ] **Step 3: Add the precondition** in `regressIssueOpsForReplanLocked`, after the phase-rank check:

```go
	rev := record.DevilsAdvocateReview
	if rev == nil || rev.Verdict != "stop" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("regress requires a recorded devil's-advocate stop verdict")
	}
	if strings.TrimSpace(rev.IssueReflectedAt) == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("reflect the devil's-advocate findings to the issue before regressing (issueops remote reflect-devils-advocate --confirm)")
	}
```

And after setting `record.Phase = IssueOpsPhaseGrill`, clear the review so the gate re-fires: `record.DevilsAdvocateReview = nil`.

- [ ] **Step 4: Run to verify it passes** — Run: `go test ./internal/core/issueops/ -run Regress -count=1` — Expected: PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(issueops): fail-close regress on reflected devil's-advocate stop"`.

---

## Task 9: SKILL + ADR + docs-index golden

**Files:**
- Modify: `skills/issueops/SKILL.md` (plan + compatibility-review rows, design-review routing row, Concept→Command Map)
- Modify: `.issueops/ADR.md` (new dated ADR)
- Modify: `cmd/issueops/testdata/response_contracts.golden.json` (via -update, if any `.issueops/*.md` doc-index changed — ADR.md is indexed)

- [ ] **Step 1: Update SKILL.md** — in the plan/compatibility-review guidance, state that a recorded devil's-advocate verdict (pass, or waived stop/revise) is now a machine gate for `implement`, and that a `stop` requires `issueops remote reflect-devils-advocate --confirm` before `issueops regress`. Add the two new commands to the Concept→Command Map.

- [ ] **Step 2: Add an ADR** dated 2026-07-01 (mirror the 2026-06-29 phase-ledger ADR format): Decision = devil's-advocate is a first-class fail-closed gate whose stop findings reflect to the remote issue before regress; Consequences = new record/commands/tools are public contract with goldens.

- [ ] **Step 3: Regenerate response golden** (ADR.md is docs-indexed) — Run: `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -update -count=1`; confirm the diff is docs_index bytes/headings only (CAUTIONS §27). Golden PASS without `-update`.

- [ ] **Step 4: Commit** — `git commit -m "docs(issueops): document the enforced devil's-advocate loop"`.

---

## Task 10: End-to-end verification

- [ ] **Step 1:** `gofmt -l $(git ls-files '*.go')` empty; `go build ./...`; `go vet ./...` clean.
- [ ] **Step 2:** `go test ./... -count=1` — all packages green (goldens included).
- [ ] **Step 3: Real-binary E2E** (mirror the create-issue QA in this session's transcript): build `bin/issueops`; with a fake `gh` on PATH and a temp cycle brought to implement-readiness, assert: (a) `issueops phase --to implement` (or implement-readiness) is blocked with `devils_advocate_review`; (b) `issueops devils-advocate review --verdict stop --finding …`; (c) implement still blocked (unwaived stop); (d) `issueops remote reflect-devils-advocate --confirm` writes the issue body section (fake gh `issue edit` captured) and stamps reflected; (e) `issueops regress` moves to grill and clears the review; (f) a fresh `--verdict pass` unblocks implement.
- [ ] **Step 4:** Report evidence; do NOT push (push is a separate user-approval gate).

---

## Self-Review

**Spec coverage:** Gap A record (Task 1) · gate (Task 2) · CLI (Task 3) · MCP (Task 4). Gap B splice helper (Task 5) · provider method + adapters (Task 6) · reflect + stamp (Task 7) · regress precondition + clear (Task 8). Contract/SKILL/ADR/goldens (Tasks 4/7/9). Test matrix covered across Tasks 1-8,10. Every spec §3-§6 item maps to a task.

**Placeholder scan:** Task 6 Step 4 intentionally flags the `gh issue view` vs `gh api` distinction and instructs resolving it in the same step (add `runGh` helper) — not a deferred TODO. No other placeholders.

**Type consistency:** `IssueOpsDevilsAdvocateReview` fields, `devils_advocate_review` missing-key, `UpdateIssueBodySection` signature, `issuebody.MergeIssueBodySection`/`RenderDevilsAdvocateSection`, and `IssueReflectedAt` are used identically across Tasks 1-8.

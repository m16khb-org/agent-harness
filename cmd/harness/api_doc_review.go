package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core"
)

const (
	defaultAPIDocReviewModel     = "gpt-5.5"
	defaultAPIDocReviewReasoning = "medium"
	defaultAPIDocReviewTimeout   = 3 * time.Minute
)

type apiDocReviewFinding struct {
	File     string `json:"file"`
	Line     *int   `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type apiDocReviewResult struct {
	OK       bool                  `json:"ok"`
	Verdict  string                `json:"verdict"`
	Summary  string                `json:"summary"`
	Findings []apiDocReviewFinding `json:"findings"`
	Files    []string              `json:"files"`
	Skipped  bool                  `json:"skipped,omitempty"`
	Reason   string                `json:"reason,omitempty"`
	Model    string                `json:"model,omitempty"`
	Effort   string                `json:"reasoning_effort,omitempty"`
}

type apiDocReviewOptions struct {
	Repo       string
	Model      string
	Effort     string
	Timeout    time.Duration
	Files      []string
	DiffFile   string
	PromptFile string
	JSON       bool
}

func runAPIDoc(args []string) error {
	if len(args) == 0 {
		apiDocUsage()
		return fmt.Errorf("missing api-doc subcommand")
	}
	switch args[0] {
	case "review":
		return runAPIDocReview(args[1:])
	default:
		apiDocUsage()
		return fmt.Errorf("unknown api-doc subcommand %q", args[0])
	}
}

func apiDocUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  harness api-doc review [--repo PATH] [--model MODEL] [--reasoning EFFORT] [--timeout DURATION] [--diff-file FILE] [--prompt-file FILE] [--json] [--] [FILES...]
`)
}

func runAPIDocReview(args []string) error {
	fs := flag.NewFlagSet("api-doc review", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	model := fs.String("model", defaultAPIDocReviewModel, "Codex model")
	effort := fs.String("reasoning", defaultAPIDocReviewReasoning, "Codex model_reasoning_effort")
	timeoutText := fs.String("timeout", defaultAPIDocReviewTimeout.String(), "Codex timeout")
	diffFile := fs.String("diff-file", "", "read diff from file instead of git diff --cached")
	promptFile := fs.String("prompt-file", "", "append project-specific review instructions from file")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	timeout, err := time.ParseDuration(*timeoutText)
	if err != nil {
		return err
	}
	root := resolveTarget(*repo)
	options := apiDocReviewOptions{Repo: root, Model: *model, Effort: *effort, Timeout: timeout, Files: fs.Args(), DiffFile: *diffFile, PromptFile: *promptFile, JSON: *jsonOut}
	result, err := runAPIDocReviewWithOptions(options)
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocReview(result)
	return err
}

func runAPIDocReviewWithOptions(options apiDocReviewOptions) (apiDocReviewResult, error) {
	files := normalizeAPIDocFiles(options.Repo, options.Files)
	if len(files) == 0 && options.DiffFile == "" {
		files = stagedAPIDocFiles(options.Repo)
	}
	if len(files) == 0 && options.DiffFile == "" {
		return apiDocReviewResult{OK: true, Verdict: "pass", Summary: "No staged API documentation candidate files.", Findings: []apiDocReviewFinding{}, Files: []string{}, Skipped: true, Reason: "no_api_doc_candidate_files"}, nil
	}
	diff, err := apiDocDiff(options.Repo, files, options.DiffFile)
	if err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	if strings.TrimSpace(diff) == "" {
		return apiDocReviewResult{OK: true, Verdict: "pass", Summary: "No staged API documentation diff.", Findings: []apiDocReviewFinding{}, Files: files, Skipped: true, Reason: "empty_diff"}, nil
	}
	extraPrompt := ""
	if options.PromptFile != "" {
		b, err := os.ReadFile(options.PromptFile)
		if err != nil {
			return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files}, err
		}
		extraPrompt = string(b)
	}
	review, err := runCodexAPIDocReview(options, files, diff, extraPrompt)
	if err != nil {
		return review, err
	}
	return review, nil
}

func runCodexAPIDocReview(options apiDocReviewOptions, files []string, diff, extraPrompt string) (apiDocReviewResult, error) {
	tmpDir, err := os.MkdirTemp("", "agent-harness-api-doc-review-")
	if err != nil {
		return apiDocReviewResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	schemaPath := filepath.Join(tmpDir, "schema.json")
	outputPath := filepath.Join(tmpDir, "review.json")
	if err := os.WriteFile(schemaPath, mustJSON(apiDocReviewSchema()), 0o600); err != nil {
		return apiDocReviewResult{}, err
	}
	cmd := exec.Command("codex", "--ask-for-approval", "never", "exec", "--model", options.Model, "--config", fmt.Sprintf("model_reasoning_effort=\"%s\"", options.Effort), "--cd", options.Repo, "--sandbox", "read-only", "--output-schema", schemaPath, "--output-last-message", outputPath, "-")
	cmd.Stdin = strings.NewReader(buildAPIDocReviewPrompt(files, diff, extraPrompt))
	cmd.Dir = options.Repo
	cmd.Env = os.Environ()
	if options.Timeout <= 0 {
		options.Timeout = defaultAPIDocReviewTimeout
	}
	if err := runWithTimeout(cmd, options.Timeout); err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	b, err := os.ReadFile(outputPath)
	if err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	var result apiDocReviewResult
	if err := json.Unmarshal(b, &result); err != nil {
		return apiDocReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, Model: options.Model, Effort: options.Effort}, err
	}
	result.Files = files
	result.Model = options.Model
	result.Effort = options.Effort
	result.OK = result.Verdict == "pass"
	if result.Verdict == "fail" {
		return result, fmt.Errorf("api documentation AI review failed")
	}
	return result, nil
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("codex failed: %w: %s", err, stderr.String())
		}
		return nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return fmt.Errorf("codex timed out after %s", timeout)
	}
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt string) string {
	return fmt.Sprintf(`You are a strict, framework-agnostic pre-commit reviewer for API documentation contract drift.

Review only the staged diff for the files listed below. Do not fail unrelated legacy debt outside this diff.

Goal:
- New or changed API endpoints, request/response schemas, DTOs, handlers, route methods, or OpenAPI specs must keep machine-readable API documentation complete enough for clients.
- Apply the documentation style used by the target project and framework. Do not force NestJS decorators onto Go, Python, Java, OpenAPI YAML, or other stacks.

Examples of framework-specific evidence to look for:
- NestJS/OpenAPI: @ApiOperation with description, @ApiParam for path params, @ApiHeader for headers, @ApiResponse/@ApiBadRequestResponse/@ApiUnauthorizedResponse, @ApiProperty/@ApiPropertyOptional, @IsOptional when validation expects optional fields.
- Go Swagger/OpenAPI tools such as swaggo: @Summary/@Description, @Param for path/query/header/body, @Success/@Failure responses, @Security for protected endpoints, documented request/response structs.
- OpenAPI/Swagger specs: paths, parameters, requestBody, responses including validation/auth failures, schemas, required vs optional properties.
- Spring/FastAPI/other API frameworks: equivalent operation summaries/descriptions, parameters, request/response models, validation/auth/error responses.

Blocking omissions include:
- A changed endpoint/handler/route lacks operation-level documentation expected by the project.
- A changed path/query/header/body parameter is not represented in docs.
- A changed request or response shape is not represented in docs/schema.
- Required vs optional fields are misdocumented or validation optionality is undocumented where the stack has an explicit doc/validation convention.
- A changed endpoint can clearly return validation/auth errors but the API docs omit those responses.
- A protected endpoint clearly lacks security/auth documentation.

Decision rules:
- verdict "fail" only for blocking API documentation omissions introduced or exposed by the staged diff.
- verdict "pass" if there are no blocking omissions.
- Warnings are allowed, but any blocking finding must make verdict "fail".
- Be conservative where static inference is impossible. Do not require documenting every deep service-layer business exception unless the staged diff makes the public endpoint contract clearly incomplete.
- Respond only with JSON matching the schema. No Markdown.

Additional project-specific instructions, if any:
%s

Staged files:
%s

Staged diff:
%s
`, strings.TrimSpace(extraPrompt), bulletLines(files), diff)
}

func apiDocReviewSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"verdict", "summary", "findings"}, "properties": map[string]any{
		"verdict": map[string]any{"type": "string", "enum": []string{"pass", "fail"}},
		"summary": map[string]any{"type": "string"},
		"findings": map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"file", "line", "severity", "message"}, "properties": map[string]any{
			"file":     map[string]any{"type": "string"},
			"line":     map[string]any{"type": []string{"number", "null"}},
			"severity": map[string]any{"type": "string", "enum": []string{"blocking", "warning"}},
			"message":  map[string]any{"type": "string"},
		}}},
	}}
}

func apiDocDiff(repo string, files []string, diffFile string) (string, error) {
	if diffFile != "" {
		b, err := os.ReadFile(diffFile)
		return string(b), err
	}
	args := append([]string{"diff", "--cached", "--"}, files...)
	code, out, stderr := core.GitCmd(repo, args...)
	if code != 0 {
		return "", fmt.Errorf("git diff failed: %s", stderr)
	}
	return out, nil
}

func stagedAPIDocFiles(repo string) []string {
	code, out, _ := core.GitCmd(repo, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--")
	if code != 0 {
		return nil
	}
	return normalizeAPIDocFiles(repo, splitLines(out))
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" || !isAPIDocCandidate(file) {
			continue
		}
		if filepath.IsAbs(file) {
			if rel, err := filepath.Rel(repo, file); err == nil {
				file = rel
			}
		}
		file = filepath.ToSlash(filepath.Clean(file))
		if file == "." || strings.HasPrefix(file, "../") || seen[file] {
			continue
		}
		seen[file] = true
		out = append(out, file)
	}
	sort.Strings(out)
	return out
}

var apiDocCandidateRe = regexp.MustCompile(`(?i)(controller|dto|route|router|handler|endpoint|openapi|swagger|api|schema|proto)`)

func isAPIDocCandidate(file string) bool {
	base := filepath.Base(file)
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".md" || ext == ".txt" {
		return false
	}
	if base == "package.json" || strings.HasSuffix(base, "lock") {
		return false
	}
	return apiDocCandidateRe.MatchString(file)
}

func bulletLines(files []string) string {
	if len(files) == 0 {
		return "- <none>"
	}
	var b strings.Builder
	for _, file := range files {
		b.WriteString("- ")
		b.WriteString(file)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func mustJSON(value any) []byte {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

func printAPIDocReview(result apiDocReviewResult) {
	fmt.Printf("API doc review verdict: %s\n", result.Verdict)
	if result.Summary != "" {
		fmt.Println(result.Summary)
	}
	for _, finding := range result.Findings {
		location := finding.File
		if finding.Line != nil {
			location = fmt.Sprintf("%s:%d", finding.File, *finding.Line)
		}
		fmt.Printf("- [%s] %s %s\n", finding.Severity, location, finding.Message)
	}
}

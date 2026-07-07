package harnessapp

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"agent-harness/cmd/harness/apidoc"
	"agent-harness/cmd/harness/contractcli"
	"agent-harness/cmd/harness/pathutil"
	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/core"
)

type CompatibilityContract = contractcli.CompatibilityContract

type (
	apiDocCheckResult     = apidoc.CheckResult
	apiDocReviewFinding   = apidoc.ReviewFinding
	apiDocReviewOptions   = apidoc.ReviewOptions
	apiDocReviewResult    = apidoc.ReviewResult
	apiDocStaticOptions   = apidoc.StaticOptions
	apiDocStaticResult    = apidoc.StaticResult
	apiDocStaticViolation = apidoc.StaticViolation
)

var (
	errAPIDocReviewGateFailed     = apidoc.ErrReviewGateFailed
	errAPIDocStaticGateFailed     = apidoc.ErrStaticGateFailed
	errSelfVerificationGateFailed = selfworkflow.ErrSelfVerificationGateFailed
)

func configureContractCLI() {
	contractcli.MCPTools = mcpTools
}

func runContract(args []string) error {
	configureContractCLI()
	return contractcli.Run(args)
}

func compatibilityContract() CompatibilityContract {
	configureContractCLI()
	return contractcli.BuildCompatibilityContract()
}

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, errAPIDocReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, errAPIDocStaticGateFailed)
}

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, errSelfVerificationGateFailed)
}

func printJSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func readHarnessFile(parts ...string) (string, error) {
	return pathutil.ReadHarnessFile(harnessRoot(), parts...)
}

func harnessRoot() string {
	return pathutil.HarnessRoot(filepath.Join("skills", skillName, "SKILL.md"))
}

func findUp(start, marker string) (string, bool) {
	return pathutil.FindUp(start, marker)
}

func resolveTarget(arg string) string {
	return pathutil.ResolveTarget(arg)
}

func exists(path string) bool {
	return pathutil.Exists(path)
}

func splitLines(s string) []string {
	return pathutil.SplitLines(s)
}

func splitCSV(s string) []string {
	return pathutil.SplitCSV(s)
}

func containsString(items []string, want string) bool {
	return pathutil.ContainsString(items, want)
}

func stateDoctorHasIssueCode(issues []core.StateDoctorIssue, want string) bool {
	return pathutil.StateDoctorHasIssueCode(issues, want)
}

func runAPIDoc(args []string) error {
	return apidoc.Run(args)
}

func runAPIDocCheck(args []string) error {
	return apidoc.RunCheck(args)
}

func runAPIDocCheckWithOptions(staticOptions apiDocStaticOptions, reviewOptions apiDocReviewOptions) (apiDocCheckResult, error) {
	return apidoc.RunCheckWithOptions(staticOptions, reviewOptions)
}

func runAPIDocReview(args []string) error {
	return apidoc.RunReview(args)
}

func runAPIDocReviewWithOptions(options apiDocReviewOptions) (apiDocReviewResult, error) {
	return apidoc.RunReviewWithOptions(options)
}

func runAPIDocStaticCheck(args []string) error {
	return apidoc.RunStaticCheck(args)
}

func runAPIDocStaticCheckWithOptions(options apiDocStaticOptions) (apiDocStaticResult, error) {
	return apidoc.RunStaticCheckWithOptions(options)
}

func printAPIDocReview(result apiDocReviewResult) {
	apidoc.PrintReview(result)
}

func printAPIDocStaticCheck(result apiDocStaticResult) {
	apidoc.PrintStaticCheck(result)
}

func mustJSON(value any) []byte {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

func checkNestControllerStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestControllerStatic(file, text)
}

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	return apidoc.CheckNestDTOStatic(file, text)
}

func buildAPIDocReviewPrompt(files []string, diff, extraPrompt string) string {
	return apidoc.BuildReviewPrompt(files, diff, extraPrompt)
}

func apiDocReviewSchema() map[string]any {
	return apidoc.ReviewSchema()
}

func apiDocReviewExtraPrompt(options apiDocReviewOptions) (string, error) {
	return apidoc.ReviewExtraPrompt(options.Repo, options.PromptFile)
}

func apiDocDiff(repo string, files []string, diffFile string) (string, error) {
	return apidoc.Diff(repo, files, diffFile)
}

func apiDocInput(repo string, files []string, diffFile string, all bool) (string, error) {
	return apidoc.Input(repo, files, diffFile, all)
}

func apiDocFullContent(repo string, files []string) (string, error) {
	return apidoc.FullContent(repo, files)
}

func stagedAPIDocFiles(repo string) []string {
	return apidoc.StagedFiles(repo)
}

func trackedAPIDocFiles(repo string) []string {
	return apidoc.TrackedFiles(repo)
}

func normalizeAPIDocFiles(repo string, files []string) []string {
	return apidoc.NormalizeFiles(repo, files)
}

func isAPIDocCandidate(file string) bool {
	return apidoc.IsCandidate(file)
}

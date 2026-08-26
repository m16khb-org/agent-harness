package apidoc

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type apiDocStaticResult struct {
	OK         bool                    `json:"ok"`
	Summary    string                  `json:"summary"`
	Files      []string                `json:"files"`
	Violations []apiDocStaticViolation `json:"violations"`
	Skipped    bool                    `json:"skipped,omitempty"`
	Reason     string                  `json:"reason,omitempty"`
}

type apiDocStaticOptions struct {
	Repo  string
	Files []string
	All   bool
	JSON  bool
}

func runAPIDocStaticCheck(args []string) error {
	fs := flag.NewFlagSet("api-doc static-check", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "check all tracked API documentation candidate files instead of staged changes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := runAPIDocStaticCheckWithOptions(apiDocStaticOptions{Repo: ResolveTarget(*repo), Files: fs.Args(), All: *all, JSON: *jsonOut})
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocStaticCheck(result)
	return err
}

func runAPIDocStaticCheckWithOptions(options apiDocStaticOptions) (apiDocStaticResult, error) {
	files := normalizeAPIDocFiles(options.Repo, options.Files)
	if len(files) == 0 && options.All {
		files = trackedAPIDocFiles(options.Repo)
	}
	if len(files) == 0 && !options.All {
		files = stagedAPIDocFiles(options.Repo)
	}
	if len(files) == 0 {
		return apiDocStaticResult{OK: true, Summary: "No API documentation candidate files.", Files: []string{}, Violations: []apiDocStaticViolation{}, Skipped: true, Reason: "no_api_doc_candidate_files"}, nil
	}
	if apiDocMode(options.Repo) == "contract-tests" {
		return apiDocStaticResult{
			OK:         true,
			Summary:    "Swagger decorator checks skipped; repository uses contract-tests API documentation.",
			Files:      files,
			Violations: []apiDocStaticViolation{},
			Skipped:    true,
			Reason:     "contract_tests_mode",
		}, nil
	}
	var violations []apiDocStaticViolation
	for _, file := range files {
		if !strings.HasSuffix(file, ".ts") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(options.Repo, filepath.Clean(file)))
		if err != nil {
			return apiDocStaticResult{OK: false, Summary: err.Error(), Files: files}, err
		}
		text := string(b)
		lower := strings.ToLower(file)
		if strings.Contains(lower, "controller") || strings.Contains(lower, "handler") || strings.Contains(lower, "route") || strings.Contains(lower, "router") {
			violations = append(violations, checkNestControllerStatic(file, text)...)
		}
		if strings.Contains(lower, "dto") || strings.Contains(lower, "schema") {
			violations = append(violations, checkNestDTOStatic(file, text)...)
		}
	}
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		if violations[i].Line != violations[j].Line {
			return violations[i].Line < violations[j].Line
		}
		return violations[i].Code < violations[j].Code
	})
	if violations == nil {
		violations = []apiDocStaticViolation{}
	}
	result := apiDocStaticResult{OK: len(violations) == 0, Files: files, Violations: violations}
	if result.OK {
		result.Summary = "API documentation static check passed."
		return result, nil
	}
	result.Summary = fmt.Sprintf("API documentation static check found %d violation(s).", len(violations))
	return result, ErrStaticGateFailed
}

func apiDocMode(repo string) string {
	content, err := os.ReadFile(filepath.Join(repo, ".agent-harness", "OPEN_API_SPEC.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		key, value, found := strings.Cut(trimmed, ":")
		if found && strings.TrimSpace(key) == "api_doc_mode" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type apiDocStaticViolation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

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
	result, err := runAPIDocStaticCheckWithOptions(apiDocStaticOptions{Repo: resolveTarget(*repo), Files: fs.Args(), All: *all, JSON: *jsonOut})
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
	return result, fmt.Errorf("api documentation static check failed")
}

var nestRouteRe = regexp.MustCompile(`@(Get|Post|Put|Patch|Delete)\s*\(\s*(?:["'\x60]([^"'\x60]*)["'\x60])?`)
var nestPathParamRe = regexp.MustCompile(`:([A-Za-z0-9_]+)`)
var apiResponseStatusRe = regexp.MustCompile(`@ApiResponse\s*\(\s*\{[^}]*status\s*:\s*([0-9]+)`)
var methodNameRe = regexp.MustCompile(`^\s*(?:async\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\(`)
var queryNamedRe = regexp.MustCompile(`@Query\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)
var bodyNamedRe = regexp.MustCompile(`@Body\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)

func checkNestControllerStatic(file, text string) []apiDocStaticViolation {
	lines := strings.Split(text, "\n")
	var violations []apiDocStaticViolation
	for i := 0; i < len(lines); i++ {
		if !methodNameRe.MatchString(lines[i]) {
			continue
		}
		start := i
		for start > 0 && (strings.TrimSpace(lines[start-1]) == "" || strings.HasPrefix(strings.TrimSpace(lines[start-1]), "@") || strings.HasPrefix(strings.TrimSpace(lines[start-1]), ".") || strings.Contains(lines[start-1], "})") || strings.Contains(lines[start-1], "})")) {
			start--
			if start == 0 || strings.Contains(lines[start], "@Get") || strings.Contains(lines[start], "@Post") || strings.Contains(lines[start], "@Put") || strings.Contains(lines[start], "@Patch") || strings.Contains(lines[start], "@Delete") {
				break
			}
		}
		end := i
		for end < len(lines) && !strings.Contains(lines[end], "{") {
			end++
		}
		block := strings.Join(lines[start:min(end+1, len(lines))], "\n")
		route := nestRouteRe.FindStringSubmatch(block)
		if route == nil {
			continue
		}
		line := i + 1
		if !strings.Contains(block, "@ApiOperation") {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_api_operation", Message: "route method is missing @ApiOperation"})
		} else if !strings.Contains(block, "description") {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_api_operation_description", Message: "@ApiOperation is missing description"})
		} else if !strings.Contains(block, "### ") {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "invalid_api_operation_description_format", Message: "@ApiOperation.description must use the project sectioned Markdown format"})
		}
		for _, m := range nestPathParamRe.FindAllStringSubmatch(route[2], -1) {
			if !regexp.MustCompile(`@ApiParam\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(m[1]) + `["'\x60]`).MatchString(block) {
				violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_api_param", Message: "path parameter :" + m[1] + " is missing @ApiParam documentation"})
			}
		}
		if strings.Contains(block, "@Headers") && !strings.Contains(block, "@ApiHeader") {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_api_header", Message: "@Headers usage is missing @ApiHeader documentation"})
		}
		for _, m := range queryNamedRe.FindAllStringSubmatch(block, -1) {
			if !regexp.MustCompile(`@ApiQuery\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(m[1]) + `["'\x60]`).MatchString(block) {
				violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_api_query", Message: "named query parameter " + m[1] + " is missing @ApiQuery documentation"})
			}
		}
		if bodyNamedRe.MatchString(block) && !strings.Contains(block, "@ApiBody") {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_api_body", Message: "named @Body usage is missing @ApiBody documentation"})
		}
		if (strings.Contains(block, "@Body") || strings.Contains(block, "@Query") || strings.Contains(block, "@Headers")) && !hasNestResponseStatus(block, 400) {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_400_response", Message: "body/query/header validation surface is missing a 400 Swagger response"})
		}
		if isNestPrivateRoute(block, text) && !hasNestResponseStatus(block, 401) {
			violations = append(violations, apiDocStaticViolation{File: file, Line: line, Code: "missing_401_response", Message: "private/auth route is missing a 401 Swagger response"})
		}
	}
	return violations
}

func hasNestResponseStatus(block string, status int) bool {
	if status == 400 && strings.Contains(block, "@ApiBadRequestResponse") {
		return true
	}
	if status == 401 && strings.Contains(block, "@ApiUnauthorizedResponse") {
		return true
	}
	for _, m := range apiResponseStatusRe.FindAllStringSubmatch(block, -1) {
		if m[1] == fmt.Sprint(status) {
			return true
		}
	}
	return false
}

func isNestPrivateRoute(block, fileText string) bool {
	if strings.Contains(block, "@Public") {
		return false
	}
	return strings.Contains(block, "@ApiBearerAuth") || strings.Contains(fileText, "@ApiBearerAuth") || strings.Contains(block, "UseGuards") || strings.Contains(block, "RequireTier")
}

var dtoPropertyRe = regexp.MustCompile(`^\s*(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\??\s*[!:?]?\s*:`)

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	lines := strings.Split(text, "\n")
	var violations []apiDocStaticViolation
	var decorators []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@") {
			decorators = append(decorators, trimmed)
			continue
		}
		m := dtoPropertyRe.FindStringSubmatch(line)
		if m == nil || strings.Contains(trimmed, "(") || strings.HasPrefix(trimmed, "private ") || strings.HasPrefix(trimmed, "static ") {
			if trimmed != "" && !strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, "}") {
				decorators = nil
			}
			continue
		}
		deco := strings.Join(decorators, "\n")
		optional := strings.Contains(line, "?:") || strings.Contains(deco, "@IsOptional")
		if optional {
			if !strings.Contains(deco, "@ApiPropertyOptional") {
				violations = append(violations, apiDocStaticViolation{File: file, Line: i + 1, Code: "missing_api_property_optional", Message: "optional DTO property " + m[1] + " is missing @ApiPropertyOptional"})
			}
			if !strings.Contains(deco, "@IsOptional") {
				violations = append(violations, apiDocStaticViolation{File: file, Line: i + 1, Code: "missing_is_optional", Message: "optional DTO property " + m[1] + " is missing @IsOptional"})
			}
		} else if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
			violations = append(violations, apiDocStaticViolation{File: file, Line: i + 1, Code: "missing_api_property", Message: "required DTO property " + m[1] + " is missing @ApiProperty"})
		}
		decorators = nil
	}
	return violations
}

func printAPIDocStaticCheck(result apiDocStaticResult) {
	if result.OK {
		fmt.Println(result.Summary)
		return
	}
	fmt.Println(result.Summary)
	for _, v := range result.Violations {
		fmt.Printf("- %s:%d %s: %s\n", v.File, v.Line, v.Code, v.Message)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

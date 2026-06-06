package apidoc

import (
	"fmt"
	"regexp"
	"strings"
)

var nestRouteRe = regexp.MustCompile(`@(Get|Post|Put|Patch|Delete)\s*\(\s*(?:["'\x60]([^"'\x60]*)["'\x60])?`)
var nestPathParamRe = regexp.MustCompile(`:([A-Za-z0-9_]+)`)
var apiResponseStatusRe = regexp.MustCompile(`@ApiResponse\s*\(\s*\{[^}]*status\s*:\s*([0-9]+)`)
var methodNameRe = regexp.MustCompile(`^\s*(?:async\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\(`)
var queryNamedRe = regexp.MustCompile(`@Query\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)
var bodyNamedRe = regexp.MustCompile(`@Body\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)

func CheckNestControllerStatic(file, text string) []StaticViolation {
	lines := strings.Split(text, "\n")
	var violations []StaticViolation
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
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_api_operation", Message: "route method is missing @ApiOperation"})
		} else if !strings.Contains(block, "description") {
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_api_operation_description", Message: "@ApiOperation is missing description"})
		} else if !strings.Contains(block, "### ") {
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "invalid_api_operation_description_format", Message: "@ApiOperation.description must use the project sectioned Markdown format"})
		}
		for _, m := range nestPathParamRe.FindAllStringSubmatch(route[2], -1) {
			if !regexp.MustCompile(`@ApiParam\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(m[1]) + `["'\x60]`).MatchString(block) {
				violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_api_param", Message: "path parameter :" + m[1] + " is missing @ApiParam documentation"})
			}
		}
		if strings.Contains(block, "@Headers") && !strings.Contains(block, "@ApiHeader") {
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_api_header", Message: "@Headers usage is missing @ApiHeader documentation"})
		}
		for _, m := range queryNamedRe.FindAllStringSubmatch(block, -1) {
			if !regexp.MustCompile(`@ApiQuery\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(m[1]) + `["'\x60]`).MatchString(block) {
				violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_api_query", Message: "named query parameter " + m[1] + " is missing @ApiQuery documentation"})
			}
		}
		if bodyNamedRe.MatchString(block) && !strings.Contains(block, "@ApiBody") {
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_api_body", Message: "named @Body usage is missing @ApiBody documentation"})
		}
		if (strings.Contains(block, "@Body") || strings.Contains(block, "@Query") || strings.Contains(block, "@Headers")) && !hasNestResponseStatus(block, 400) {
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_400_response", Message: "body/query/header validation surface is missing a 400 Swagger response"})
		}
		if isNestPrivateRoute(block, text) && !hasNestResponseStatus(block, 401) {
			violations = append(violations, StaticViolation{File: file, Line: line, Code: "missing_401_response", Message: "private/auth route is missing a 401 Swagger response"})
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

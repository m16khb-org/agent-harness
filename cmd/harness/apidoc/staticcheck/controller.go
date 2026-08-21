package staticcheck

import (
	"regexp"
	"strconv"
	"strings"
)

var nestRouteRe = regexp.MustCompile(`@(Get|Post|Put|Patch|Delete|All|Head|Options)\s*\(\s*(?:["'\x60]([^"'\x60]*)["'\x60])?`)
var nestPathParamRe = regexp.MustCompile(`:([A-Za-z0-9_]+)`)
var apiResponseStatusRe = regexp.MustCompile(`@ApiResponses?\s*\(`)
var apiResponseStatusValueRe = regexp.MustCompile(`status\s*:\s*(?:HttpStatus\.)?([0-9]+|[A-Z_]+)`)

var nestHttpStatusNames = map[string]int{
	"OK":                    200,
	"CREATED":               201,
	"ACCEPTED":              202,
	"NO_CONTENT":            204,
	"BAD_REQUEST":           400,
	"UNAUTHORIZED":          401,
	"PAYMENT_REQUIRED":      402,
	"FORBIDDEN":             403,
	"NOT_FOUND":             404,
	"CONFLICT":              409,
	"GONE":                  410,
	"PRECONDITION_FAILED":   412,
	"UNPROCESSABLE_ENTITY":  422,
	"TOO_MANY_REQUESTS":     429,
	"INTERNAL_SERVER_ERROR": 500,
	"NOT_IMPLEMENTED":       501,
	"BAD_GATEWAY":           502,
	"SERVICE_UNAVAILABLE":   503,
	"GATEWAY_TIMEOUT":       504,
}
var methodNameRe = regexp.MustCompile(`^\s*(?:async\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\(`)
var queryNamedRe = regexp.MustCompile(`@Query\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)
var bodyNamedRe = regexp.MustCompile(`@Body\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`)

func CheckNestController(file, text string) []Violation {
	lines := strings.Split(text, "\n")
	var violations []Violation
	for i := 0; i < len(lines); i++ {
		if !methodNameRe.MatchString(lines[i]) {
			continue
		}
		start := controllerDecoratorStart(lines, i)
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
			violations = append(violations, Violation{File: file, Line: line, Code: "missing_api_operation", Message: "route method is missing @ApiOperation"})
		} else if !strings.Contains(block, "description") {
			violations = append(violations, Violation{File: file, Line: line, Code: "missing_api_operation_description", Message: "@ApiOperation is missing description"})
		} else if !strings.Contains(block, "### ") {
			violations = append(violations, Violation{File: file, Line: line, Code: "invalid_api_operation_description_format", Message: "@ApiOperation.description must use the project sectioned Markdown format"})
		}
		for _, m := range nestPathParamRe.FindAllStringSubmatch(route[2], -1) {
			if !regexp.MustCompile(`@ApiParam\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(m[1]) + `["'\x60]`).MatchString(block) {
				violations = append(violations, Violation{File: file, Line: line, Code: "missing_api_param", Message: "path parameter :" + m[1] + " is missing @ApiParam documentation"})
			}
		}
		if strings.Contains(block, "@Headers") && !strings.Contains(block, "@ApiHeader") {
			violations = append(violations, Violation{File: file, Line: line, Code: "missing_api_header", Message: "@Headers usage is missing @ApiHeader documentation"})
		}
		for _, m := range queryNamedRe.FindAllStringSubmatch(block, -1) {
			if !regexp.MustCompile(`@ApiQuery\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(m[1]) + `["'\x60]`).MatchString(block) {
				violations = append(violations, Violation{File: file, Line: line, Code: "missing_api_query", Message: "named query parameter " + m[1] + " is missing @ApiQuery documentation"})
			}
		}
		if bodyNamedRe.MatchString(block) && !strings.Contains(block, "@ApiBody") {
			violations = append(violations, Violation{File: file, Line: line, Code: "missing_api_body", Message: "named @Body usage is missing @ApiBody documentation"})
		}
		if (strings.Contains(block, "@Body") || strings.Contains(block, "@Query") || strings.Contains(block, "@Headers")) && !hasNestResponseStatus(block, 400) {
			violations = append(violations, Violation{File: file, Line: line, Code: "missing_400_response", Message: "body/query/header validation surface is missing a 400 Swagger response"})
		}
		if isNestPrivateRoute(block, text) && !hasNestResponseStatus(block, 401) {
			violations = append(violations, Violation{File: file, Line: line, Code: "missing_401_response", Message: "private/auth route is missing a 401 Swagger response"})
		}
	}
	return violations
}

// controllerDecoratorStart returns the first line of the decorator block for
// the member whose signature sits at lines[member]. It walks upward across
// decorators and their multi-line object interiors (summary:/description:
// properties inside @ApiOperation({...})) using paren/brace balance, and stops
// at member or class boundaries. A naive line-prefix walk stops inside
// multi-line decorator objects and silently skips the whole route method.
func controllerDecoratorStart(lines []string, member int) int {
	start := member
	balance := 0
	for start > 0 {
		line := lines[start-1]
		trimmed := strings.TrimSpace(line)
		delta := braceDepthDelta(line) + parenDepthDelta(line)
		if strings.Contains(trimmed, "class ") && strings.HasSuffix(trimmed, "{") {
			break
		}
		if trimmed == "}" || strings.HasSuffix(trimmed, ";") {
			break
		}
		if balance != 0 || delta < 0 || strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, ".") || trimmed == "" {
			balance += delta
			start--
			continue
		}
		break
	}
	return start
}

func parenDepthDelta(line string) int {
	delta := 0
	for _, ch := range line {
		switch ch {
		case '(':
			delta++
		case ')':
			delta--
		}
	}
	return delta
}

func hasNestResponseStatus(block string, status int) bool {
	if status == 400 && strings.Contains(block, "@ApiBadRequestResponse") {
		return true
	}
	if status == 401 && strings.Contains(block, "@ApiUnauthorizedResponse") {
		return true
	}
	for _, span := range apiResponseDecoratorSpans(block) {
		for _, m := range apiResponseStatusValueRe.FindAllStringSubmatch(span, -1) {
			code, err := strconv.Atoi(m[1])
			if err != nil {
				code = nestHttpStatusNames[m[1]]
			}
			if code == status {
				return true
			}
		}
	}
	return false
}

// apiResponseDecoratorSpans returns the balanced argument text of every
// @ApiResponse / @ApiResponses occurrence, so both object form
// ({ status: 404 }) and array form ([{ status: 404 }, ...]) are scanned.
func apiResponseDecoratorSpans(block string) []string {
	var spans []string
	for _, loc := range apiResponseStatusRe.FindAllStringIndex(block, -1) {
		depth := 0
		started := false
		for i := loc[1] - 1; i < len(block); i++ {
			switch block[i] {
			case '(':
				depth++
				started = true
			case ')':
				depth--
			}
			if started && depth <= 0 {
				spans = append(spans, block[loc[1]:i])
				break
			}
		}
	}
	return spans
}

func isNestPrivateRoute(block, fileText string) bool {
	if strings.Contains(block, "@Public") {
		return false
	}
	return strings.Contains(block, "@ApiBearerAuth") || strings.Contains(fileText, "@ApiBearerAuth") || strings.Contains(block, "UseGuards") || strings.Contains(block, "RequireTier") || hasClassLevelGuard(fileText)
}

// hasClassLevelGuard reports whether the file declares a controller-level auth
// guard (@UseGuards/@ApiBearerAuth/RequireTier above the class declaration),
// which makes every member route private unless it opts out with @Public.
func hasClassLevelGuard(fileText string) bool {
	classIdx := strings.Index(fileText, "class ")
	if classIdx < 0 {
		return false
	}
	header := fileText[:classIdx]
	return strings.Contains(header, "@UseGuards") || strings.Contains(header, "@ApiBearerAuth") || strings.Contains(header, "RequireTier")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

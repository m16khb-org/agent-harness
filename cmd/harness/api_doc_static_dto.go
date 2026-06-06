package main

import (
	"regexp"
	"strings"
)

var dtoPropertyRe = regexp.MustCompile(`^\s*(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\??\s*[!:?]?\s*:`)
var dtoClassDeclarationRe = regexp.MustCompile(`\b(?:export\s+)?(?:abstract\s+)?class\s+[A-Za-z_][A-Za-z0-9_]*\b`)

func checkNestDTOStatic(file, text string) []apiDocStaticViolation {
	lines := strings.Split(text, "\n")
	var violations []apiDocStaticViolation
	var decorators []string
	inClass := false
	pendingClassBody := false
	classDepth := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inClass {
			if dtoClassDeclarationRe.MatchString(trimmed) {
				pendingClassBody = true
			}
			if pendingClassBody && strings.Contains(trimmed, "{") {
				inClass = true
				pendingClassBody = false
				classDepth = braceDepthDelta(line)
			}
			decorators = nil
			continue
		}
		if classDepth != 1 {
			classDepth += braceDepthDelta(line)
			if classDepth <= 0 {
				inClass = false
				decorators = nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			decorators = append(decorators, trimmed)
			classDepth += braceDepthDelta(line)
			continue
		}
		m := dtoPropertyRe.FindStringSubmatch(line)
		if m == nil || strings.Contains(trimmed, "(") || strings.HasPrefix(trimmed, "private ") || strings.HasPrefix(trimmed, "static ") {
			if trimmed != "" && !strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, "}") {
				decorators = nil
			}
			classDepth += braceDepthDelta(line)
			if classDepth <= 0 {
				inClass = false
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
		classDepth += braceDepthDelta(line)
		if classDepth <= 0 {
			inClass = false
			decorators = nil
		}
	}
	return violations
}

func braceDepthDelta(line string) int {
	delta := 0
	for _, ch := range line {
		switch ch {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

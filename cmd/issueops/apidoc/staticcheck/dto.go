package staticcheck

import (
	"regexp"
	"strings"
)

var dtoPropertyRe = regexp.MustCompile(`^\s*(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\??\s*[!:?]?\s*:`)
var dtoClassDeclarationRe = regexp.MustCompile(`\b(?:export\s+)?(?:abstract\s+)?class\s+[A-Za-z_][A-Za-z0-9_]*\b`)

func CheckNestDTO(file, text string) []Violation {
	lines := strings.Split(text, "\n")
	var violations []Violation
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
		tsOptional := strings.Contains(line, "?:") || strings.Contains(deco, "@IsOptional")
		swaggerOptional := strings.Contains(deco, "@ApiPropertyOptional") || apiPropertyObjectFlag(deco, "required", "false")
		swaggerRequiredExplicit := strings.Contains(deco, "@ApiProperty") && !strings.Contains(deco, "@ApiPropertyOptional") && (apiPropertyObjectFlag(deco, "required", "true") || !apiPropertyHasRequiredKey(deco))
		if tsOptional && swaggerRequiredExplicit {
			violations = append(violations, Violation{File: file, Line: i + 1, Code: "required_optional_mismatch", Message: "optional DTO property " + m[1] + " is documented as required (@ApiProperty required: true/default) but is optional in TypeScript/validation"})
		} else if !tsOptional && swaggerOptional {
			violations = append(violations, Violation{File: file, Line: i + 1, Code: "required_optional_mismatch", Message: "required DTO property " + m[1] + " is documented as optional in Swagger but is required in TypeScript/validation"})
		}
		if tsOptional {
			if !swaggerOptional && !swaggerRequiredExplicit {
				violations = append(violations, Violation{File: file, Line: i + 1, Code: "missing_api_property_optional", Message: "optional DTO property " + m[1] + " is missing @ApiPropertyOptional"})
			}
			if !strings.Contains(deco, "@IsOptional") {
				violations = append(violations, Violation{File: file, Line: i + 1, Code: "missing_is_optional", Message: "optional DTO property " + m[1] + " is missing @IsOptional"})
			}
		} else if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
			violations = append(violations, Violation{File: file, Line: i + 1, Code: "missing_api_property", Message: "required DTO property " + m[1] + " is missing @ApiProperty"})
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

var apiPropertyObjectValueRe = regexp.MustCompile(`required\s*:\s*(true|false)`)

// apiPropertyObjectFlag reports whether a @ApiProperty(...) decorator object
// sets the required key to the given boolean. Plain string search stays
// decorator-safe because other decorators on the same property do not carry a
// required key.
func apiPropertyObjectFlag(deco, key, want string) bool {
	if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
		return false
	}
	for _, m := range apiPropertyObjectValueRe.FindAllStringSubmatch(deco, -1) {
		if m[1] == want {
			return true
		}
	}
	return false
}

func apiPropertyHasRequiredKey(deco string) bool {
	if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
		return false
	}
	return apiPropertyObjectValueRe.MatchString(deco)
}

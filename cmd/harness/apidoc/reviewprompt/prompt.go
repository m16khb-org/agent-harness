package reviewprompt

import (
	"strings"

	"agent-harness/internal/core"
)

func Build(files []string, diff, extraPrompt string) string {
	return core.BuildStructuredPrompt(core.StructuredPromptSpec{
		Identity:  "You are a strict, framework-agnostic pre-commit reviewer for API documentation contract drift.",
		Objective: "Review the provided diff/content for the listed files, then inspect the directly related endpoint/controller/handler, DTO/schema, service/usecase, and error-mapping code needed to understand the public API contract. Do not fail unrelated legacy debt outside the changed endpoint/DTO/API surface.",
		Phases: []string{
			"Scan the changed API surface and directly related public contract code.",
			"Compare the documentation against the target project's existing framework and style.",
			"Classify only introduced or exposed documentation omissions as blocking.",
			"Return the final verdict using the required JSON schema.",
		},
		Inputs: []string{
			"Additional project-specific instructions, if any.",
			"Files under review.",
			"Diff/content under review.",
		},
		Rules: []string{
			"New or changed API endpoints, request/response schemas, DTOs, handlers, route methods, or OpenAPI specs must keep machine-readable API documentation complete enough for clients.",
			"Apply the documentation style used by the target project and framework. Do not force NestJS decorators onto Go, Python, Java, OpenAPI YAML, or other stacks.",
			"business logic errors that are part of the changed endpoint contract must appear in the OpenAPI/Swagger docs, including NotFound/404, Conflict/409, Forbidden/403, validation/400, auth/401, or equivalent domain errors when directly visible from the changed contract.",
			"Prefer clean Swagger output: concise operation summary, sectioned/consistent description, complete params, explicit request/response examples or schemas where the project convention supports them, and no misleading success-only documentation.",
			"Framework evidence includes NestJS @ApiOperation/@ApiParam/@ApiHeader/@ApiProperty/@ApiPropertyOptional/@IsOptional, Go swaggo @Summary/@Description/@Param/@Success/@Failure/@Security, OpenAPI/Swagger specs with paths/parameters/requestBody/responses/schemas, and Spring/FastAPI equivalents.",
			"Blocking omissions include missing operation docs, undocumented params, undocumented request/response shapes, required/optional drift, missing validation/auth/domain error responses, missing auth docs, or vague client-facing descriptions introduced by the change.",
			"Be conservative where static inference is impossible. Do not require documenting every deep service-layer exception unless the staged diff makes the public endpoint contract clearly incomplete.",
		},
		OutputContract: []string{
			"Respond only with JSON matching the schema. No Markdown.",
			`verdict is "fail" only for blocking API documentation omissions introduced or exposed by the provided diff/content.`,
			`verdict is "pass" if there are no blocking omissions.`,
			"Warnings are allowed, but any blocking finding must make verdict fail.",
		},
		VerificationChecklist: []string{
			"Every blocking finding cites a file and line when available.",
			"The verdict ignores unrelated legacy documentation debt.",
			"Business-logic public error contracts visible from the change were considered.",
			"The output is strict JSON with no prose or Markdown wrapper.",
		},
		Data: []core.PromptDataSection{
			{Title: "Additional Project-Specific Instructions", Content: strings.TrimSpace(extraPrompt)},
			{Title: "Files Under Review", Content: bulletLines(files)},
			{Title: "Diff Content Under Review", Content: diff},
		},
	})
}

func Schema() map[string]any {
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

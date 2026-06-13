package staticcheck

import "testing"

func TestCheckNestControllerReportsMissingDocumentation(t *testing.T) {
	text := `
@ApiBearerAuth()
export class UsersController {
  @Get(':id')
  findOne(@Param('id') id: string, @Query('expand') expand: string, @Headers('x-client') client: string) {
    return {}
  }
}`
	violations := CheckNestController("users.controller.ts", text)
	for _, code := range []string{
		"missing_api_operation",
		"missing_api_param",
		"missing_api_header",
		"missing_api_query",
		"missing_400_response",
		"missing_401_response",
	} {
		if !hasViolation(violations, code) {
			t.Fatalf("expected violation %s in %#v", code, violations)
		}
	}
}

func TestCheckNestControllerAcceptsDocumentedRoute(t *testing.T) {
	text := `
export class UsersController {
  @Get(':id')
  @Public()
  @ApiOperation({ summary: 'Get user', description: '### Summary\nGet one user' })
  @ApiParam({ name: 'id' })
  @ApiQuery({ name: 'expand' })
  @ApiBadRequestResponse()
  findOne(@Param('id') id: string, @Query('expand') expand: string) {
    return {}
  }
}`
	if violations := CheckNestController("users.controller.ts", text); len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
	}
	if !hasNestResponseStatus("@ApiResponse({ status: 400 })", 400) {
		t.Fatal("expected generic ApiResponse status detection")
	}
	if isNestPrivateRoute("@Public()\n@Get()", "@ApiBearerAuth()") {
		t.Fatal("public route should not be private")
	}
	if min(2, 3) != 2 || min(3, 2) != 2 {
		t.Fatal("unexpected min result")
	}
}

func TestCheckNestDTOReportsRequiredAndOptionalDocumentation(t *testing.T) {
	text := `
export class CreateUserDto {
  name: string
  @ApiPropertyOptional()
  nickname?: string
  @IsOptional()
  displayName?: string
  private secret: string
  static kind: string
  method(): void {}
}`
	violations := CheckNestDTO("create-user.dto.ts", text)
	for _, code := range []string{
		"missing_api_property",
		"missing_is_optional",
		"missing_api_property_optional",
	} {
		if !hasViolation(violations, code) {
			t.Fatalf("expected violation %s in %#v", code, violations)
		}
	}
}

func TestCheckNestDTOAcceptsDocumentedProperties(t *testing.T) {
	text := `
export abstract class CreateUserDto {
  @ApiProperty()
  readonly name!: string
  @ApiPropertyOptional()
  @IsOptional()
  nickname?: string
}`
	if violations := CheckNestDTO("create-user.dto.ts", text); len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
	}
	if braceDepthDelta("{a:{}}") != 0 || braceDepthDelta("{") != 1 || braceDepthDelta("}") != -1 {
		t.Fatal("unexpected brace depth deltas")
	}
}

func hasViolation(violations []Violation, code string) bool {
	for _, violation := range violations {
		if violation.Code == code {
			return true
		}
	}
	return false
}

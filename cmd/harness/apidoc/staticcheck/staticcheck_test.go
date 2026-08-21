package staticcheck

import (
	"strings"
	"testing"
)

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

func TestHasNestResponseStatusCoversEnumArrayAndNamedForms(t *testing.T) {
	cases := []struct {
		block  string
		status int
		want   bool
	}{
		{"@ApiResponse({ status: 404 })", 404, true},
		{"@ApiResponse({ status: HttpStatus.NOT_FOUND })", 404, true},
		{"@ApiResponse({ status: HttpStatus.BAD_REQUEST })", 400, true},
		{"@ApiResponses([\n  { status: 400, description: 'bad' },\n  { status: HttpStatus.CONFLICT },\n])", 400, true},
		{"@ApiResponses([\n  { status: 400, description: 'bad' },\n  { status: HttpStatus.CONFLICT },\n])", 409, true},
		{"@ApiResponses([\n  { status: 400 },\n])", 404, false},
		{"@ApiQuery({ name: 'status' })", 400, false},
	}
	for _, tc := range cases {
		if got := hasNestResponseStatus(tc.block, tc.status); got != tc.want {
			t.Fatalf("hasNestResponseStatus(%q, %d) = %v, want %v", tc.block, tc.status, got, tc.want)
		}
	}
}

func TestCheckNestControllerCoversAllAndHeadRoutes(t *testing.T) {
	text := `
export class UsersController {
  @All(':id')
  findOne(@Param('id') id: string) {
    return {};
  }
}
`
	if !hasViolation(CheckNestController("users.controller.ts", text), "missing_api_operation") {
		t.Fatal("expected @All route to be checked")
	}
}

func TestCheckNestControllerRequires401ForClassLevelGuards(t *testing.T) {
	text := `@UseGuards(JwtAuthGuard)
@Controller('users')
export class UsersController {
  @Get(':id')
  @ApiOperation({ summary: 'x', description: '### 목적\n- x' })
  @ApiParam({ name: 'id' })
  findOne(@Param('id') id: string) { return {}; }
}
`
	if !hasViolation(CheckNestController("users.controller.ts", text), "missing_401_response") {
		t.Fatal("expected class-level guard route to require a 401 response")
	}
	public := strings.Replace(text, "  @Get(':id')", "  @Get(':id')\n  @Public()", 1)
	if hasViolation(CheckNestController("users.controller.ts", public), "missing_401_response") {
		t.Fatal("@Public route must not require a 401 response")
	}
}

func TestCheckNestDTOAcceptsCommonObjectForms(t *testing.T) {
	text := `
export class FilterDto {
  @ApiPropertyOptional({ example: 'a', description: 'x' })
  @IsOptional()
  keyword?: string;

  @ApiProperty({ enum: ['a', 'b'], description: '정렬' })
  sort: string;

  @ApiProperty({ required: false, nullable: true })
  @IsOptional()
  memo?: string;

  @ApiProperty({ type: String, example: 'u1' })
  userId: string;

  @ApiPropertyOptional({ type: Number, minimum: 0 })
  @IsOptional()
  page?: number;
}
`
	if violations := CheckNestDTO("filter.dto.ts", text); len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
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

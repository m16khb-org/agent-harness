package apidoc

import "testing"

func TestCheckNestDTOStaticIgnoresConstObjectKeys(t *testing.T) {
	text := `
export const SEARCHABLE_FIELDS = {
  nickname: true,
  displayName: true,
}

export class SearchRequestDto {
  @ApiProperty()
  keyword!: string
}
`
	got := checkNestDTOStatic("search.dto.ts", text)
	if len(got) != 0 {
		t.Fatalf("const object keys must not be treated as DTO properties: %+v", got)
	}
}

func TestCheckNestDTOStaticStillChecksClassProperties(t *testing.T) {
	text := `
export class SearchRequestDto {
  keyword!: string
}
`
	got := checkNestDTOStatic("search.dto.ts", text)
	if len(got) != 1 {
		t.Fatalf("expected missing ApiProperty violation for class property, got %+v", got)
	}
	if got[0].Code != "missing_api_property" || got[0].Line != 3 {
		t.Fatalf("unexpected violation: %+v", got[0])
	}
}

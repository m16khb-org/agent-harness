package draftmeta

import (
	"reflect"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "empty content",
			content: "",
			want:    map[string]string{},
		},
		{
			name:    "no frontmatter delimiter",
			content: "just some text\nmore text\n",
			want:    map[string]string{},
		},
		{
			name:    "valid frontmatter",
			content: "---\ntitle: My Draft\nsource: claude-mem\ntarget_wiki: dev-fundamentals\nsummary: A test draft\n---\n# Body text\n",
			want:    map[string]string{"title": "My Draft", "source": "claude-mem", "target_wiki": "dev-fundamentals", "summary": "A test draft"},
		},
		{
			name:    "quoted values",
			content: "---\ntitle: \"My Draft\"\nsource: \"claude-mem\"\n---\n",
			want:    map[string]string{"title": "My Draft", "source": "claude-mem"},
		},
		{
			name:    "with target_type",
			content: "---\ntitle: Test\ntarget_type: notes\n---\n",
			want:    map[string]string{"title": "Test", "target_type": "notes"},
		},
		{
			name:    "unknown keys ignored",
			content: "---\nunknown_key: value\ntitle: Test\n---\n",
			want:    map[string]string{"title": "Test"},
		},
		{
			name:    "only delimiter no content",
			content: "---\n---\n",
			want:    map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseFrontmatter(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseFrontmatter(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

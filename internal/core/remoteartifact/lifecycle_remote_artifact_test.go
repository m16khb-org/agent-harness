package remoteartifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanAbsPath(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"", ""},
		{"  ", ""},
		{"/tmp/foo", "/tmp/foo"},
		{"/tmp/foo/../bar", "/tmp/bar"},
	}
	for _, tt := range tests {
		got := cleanAbsPath(tt.input)
		if got != tt.expected {
			t.Errorf("cleanAbsPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseGHRemoteArtifactCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		repo     string
		wantOK   bool
		wantProv string
		wantKind string
		wantAct  string
		wantTtl  string
	}{
		{
			name:     "gh issue create",
			command:  `gh issue create --title "버그 수정" --body "내용"`,
			repo:     "/tmp/repo",
			wantOK:   true,
			wantProv: "github",
			wantKind: "issue",
			wantAct:  "create",
			wantTtl:  "버그 수정",
		},
		{
			name:     "gh pr create",
			command:  `gh pr create --title "feat: 기능 추가" --body "PR 내용" --label enhancement`,
			repo:     "/tmp/repo",
			wantOK:   true,
			wantProv: "github",
			wantKind: "pr",
			wantAct:  "create",
			wantTtl:  "feat: 기능 추가",
		},
		{
			name:     "glab mr create",
			command:  `glab mr create --title "fix: 버그 수정" --description "MR 설명"`,
			repo:     "/tmp/repo",
			wantOK:   true,
			wantProv: "gitlab",
			wantKind: "mr",
			wantAct:  "create",
			wantTtl:  "fix: 버그 수정",
		},
		{
			name:    "no gh/glab",
			command: `echo hello`,
			repo:    "/tmp/repo",
			wantOK:  false,
		},
		{
			name:    "gh help",
			command: `gh issue create --help`,
			repo:    "/tmp/repo",
			wantOK:  false,
		},
		{
			name:     "gh issue create with labels via --labels=",
			command:  `gh issue create --title="버그" --body="내용" --labels="bug,enhancement"`,
			repo:     "/tmp/repo",
			wantOK:   true,
			wantProv: "github",
			wantKind: "issue",
			wantAct:  "create",
			wantTtl:  "버그",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact, ok := parseGHRemoteArtifactCommand(tt.command, tt.repo)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if artifact.provider != tt.wantProv {
				t.Errorf("provider = %q, want %q", artifact.provider, tt.wantProv)
			}
			if artifact.kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", artifact.kind, tt.wantKind)
			}
			if artifact.action != tt.wantAct {
				t.Errorf("action = %q, want %q", artifact.action, tt.wantAct)
			}
			if tt.wantTtl != "" && artifact.title != tt.wantTtl {
				t.Errorf("title = %q, want %q", artifact.title, tt.wantTtl)
			}
		})
	}
}

func TestParseGHRemoteArtifactCommand_LabelsAndAssignees(t *testing.T) {
	cmd := `gh issue create --title "제목" --body "본문" --label bug --label enhancement --assignee user1 --assignees user2,user3`
	artifact, ok := parseGHRemoteArtifactCommand(cmd, "/tmp/repo")
	if !ok {
		t.Fatal("expected ok")
	}
	if len(artifact.labels) != 2 {
		t.Errorf("expected 2 labels, got %d: %v", len(artifact.labels), artifact.labels)
	}
	if len(artifact.assignees) != 3 {
		t.Errorf("expected 3 assignees, got %d: %v", len(artifact.assignees), artifact.assignees)
	}
}

func TestScoreKoreanRemoteArtifactLanguage(t *testing.T) {
	tests := []struct {
		name                string
		text                string
		wantHangulMin       int
		wantEnglishWordsMax int
	}{
		{"korean only", "안녕하세요 버그 수정 완료했습니다 확인 부탁드립니다", 20, 5},
		{"english only", "This is a bug fix for the login feature", 0, 5},
		{"mixed", "안녕하세요 여러분 버그 수정했습니다 This is a bug fix 확인 부탁드립니다", 15, 10},
		{"code fenced", "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n안녕하세요 버그 수정 완료했습니다 확인 부탁드립니다 감사합니다", 20, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hangul, english := scoreKoreanRemoteArtifactLanguage(tt.text)
			if hangul < tt.wantHangulMin {
				t.Errorf("hangul=%d, want at least %d", hangul, tt.wantHangulMin)
			}
			if hangul < tt.wantHangulMin && english > tt.wantEnglishWordsMax {
				t.Errorf("english=%d, want at most %d (hangul=%d)", english, tt.wantEnglishWordsMax, hangul)
			}
		})
	}
}

func TestKoreanBlockReason(t *testing.T) {
	t.Run("not enough hangul", func(t *testing.T) {
		reason := KoreanBlockReason("bash", `gh issue create --title "fix bug" --body "some english text"`, "/tmp/repo")
		if reason == "" {
			t.Error("expected block reason for not enough hangul")
		}
	})
	t.Run("enough hangul", func(t *testing.T) {
		reason := KoreanBlockReason("bash", `gh issue create --title "한글 제목입니다 충분한 글자 수를 확보하기 위해 더 많은 한글 텍스트를 작성합니다" --body "한글 본문입니다 충분한 글자 수를 확보하기 위해 더 많은 한글 텍스트를 작성합니다 추가로 더 많은 한글 내용을 채워서 게이트를 통과할 수 있도록 합니다"`, "/tmp/repo")
		if reason != "" {
			t.Errorf("expected no block, got %q", reason)
		}
	})
	t.Run("not gh/glab", func(t *testing.T) {
		reason := KoreanBlockReason("bash", "echo hello", "/tmp/repo")
		if reason != "" {
			t.Errorf("expected no block for non-gh command, got %q", reason)
		}
	})
	t.Run("empty title and body", func(t *testing.T) {
		reason := KoreanBlockReason("bash", "gh issue create", "/tmp/repo")
		if reason == "" {
			t.Error("expected block reason for empty title/body")
		}
	})
	t.Run("non-bash tool", func(t *testing.T) {
		reason := KoreanBlockReason("mcp__github__issues__create", `gh issue create --title "fix bug" --body "text"`, "/tmp/repo")
		if reason == "" {
			t.Error("expected block reason for mcp github tool")
		}
	})
}

func TestGHLabRemoteArtifactGateApplies(t *testing.T) {
	tests := []struct {
		tool     string
		expected bool
	}{
		{"bash", true},
		{"sh", true},
		{"mcp__github__issues__create", true},
		{"mcp__gitlab__merge_requests__create", true},
		{"mcp__glab__mr__create", true},
		{"read_file", false},
		{"mcp__someserver__issues", false},
		{"", false},
	}
	for _, tt := range tests {
		got := remoteArtifactGateAppliesToTool(tt.tool)
		if got != tt.expected {
			t.Errorf("remoteArtifactGateAppliesToTool(%q) = %v, want %v", tt.tool, got, tt.expected)
		}
	}
}

func TestVCSIssueLinkingBlockReason_LabelsAndAssignees(t *testing.T) {
	t.Run("no labels blocks create", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `gh issue create --title "제목" --body "한글 본문입니다 충분한 글자 수를 확보하기 위해 더 많은 한글 텍스트를 작성합니다"`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "labels") {
			t.Errorf("expected labels block, got %q", reason)
		}
	})
	t.Run("no assignees blocks create", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `gh issue create --title "제목" --body "한글 본문입니다 충분한 글자 수를 확보하기 위해 더 많은 한글 텍스트를 작성합니다" --label bug`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "assignee") {
			t.Errorf("expected assignee block, got %q", reason)
		}
	})
	t.Run("placeholder assignee blocked", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `gh issue create --title "제목" --body "한글 본문입니다 충분한 글자 수를 확보하기 위해 더 많은 한글 텍스트를 작성합니다" --label bug --assignee @me`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "placeholder") {
			t.Errorf("expected placeholder block, got %q", reason)
		}
	})
}

func TestVCSIssueLinkingBlockReason_PlanLinkHeading(t *testing.T) {
	t.Run("plan link in body blocked", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `gh issue create --title "제목" --body "## Plan Link\nhttp://plan\n\n한글 본문입니다 충분한 글자 수를 확보하기 위해 더 많은 한글 텍스트를 작성합니다" --label bug --assignee user1`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "Plan Link") {
			t.Errorf("expected plan link block, got %q", reason)
		}
	})
}

func TestVCSIssueLinkingBlockReason_ChildHierarchyLinkedIssue(t *testing.T) {
	t.Run("issueops child title blocks link-related", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `agent-harness issueops link-related --id abc --type implements --related-url https://gitlab.example/group/project/-/issues/2 --title "하위 Task: 캐시 검증" --json`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "child-task") {
			t.Errorf("expected child-task block, got %q", reason)
		}
	})
	t.Run("gitlab child link api blocks", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `glab api projects/1/issues/2/links -X POST -f target_issue_iid=3 -f link_type=relates_to # child task`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "linked items and child items are different") || !strings.Contains(reason, "create-child") {
			t.Errorf("expected create-child block, got %q", reason)
		}
	})
	t.Run("gitlab child items wording blocks issue links api", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `glab api projects/1/issues/2/links -X POST -f target_issue_iid=3 -f link_type=relates_to -f note="child items under umbrella"`, "/tmp/repo")
		if reason == "" || !strings.Contains(reason, "IssueOps child-task breakdown") {
			t.Errorf("expected child item block, got %q", reason)
		}
	})
	t.Run("ordinary related link remains allowed", func(t *testing.T) {
		reason := VCSIssueLinkingBlockReason("bash", `agent-harness issueops link-related --id abc --type depends-on --related-url https://github.com/example/repo/issues/42 --title "upstream dependency" --json`, "/tmp/repo")
		if reason != "" {
			t.Errorf("expected no block, got %q", reason)
		}
	})
}

func TestRemoteArtifactBodyFileTargetAliases(t *testing.T) {
	tests := []struct {
		input  string
		expect int
	}{
		{"", 0},
		{"-", 0},
		{"  ", 0},
		{"body.md", 1},
		{"$BODY", 3},  // $BODY, ${BODY}, BODY (original $BODY deduped)
		{"${VAR}", 3}, // ${VAR}, $VAR, VAR
	}
	for _, tt := range tests {
		got := remoteArtifactBodyFileTargetAliases(tt.input)
		if tt.expect == 0 {
			if got != nil {
				t.Errorf("remoteArtifactBodyFileTargetAliases(%q) = %v, want nil", tt.input, got)
			}
		} else if len(got) != tt.expect {
			t.Errorf("remoteArtifactBodyFileTargetAliases(%q) = %v (len=%d), want len=%d", tt.input, got, len(got), tt.expect)
		}
	}
}

func TestHereDocMarkerFromLine(t *testing.T) {
	tests := []struct {
		line   string
		marker string
	}{
		{"cat <<EOF", "EOF"},
		{"cat <<-EOF", "EOF"},
		{"cat > file << 'MARKER'", "MARKER"},
		{"no heredoc", ""},
		{"cat <<", ""},
	}
	for _, tt := range tests {
		got := hereDocMarkerFromLine(tt.line)
		if got != tt.marker {
			t.Errorf("hereDocMarkerFromLine(%q) = %q, want %q", tt.line, got, tt.marker)
		}
	}
}

func TestExtractInlineHereDocBodyForTarget(t *testing.T) {
	t.Run("simple heredoc", func(t *testing.T) {
		cmd := "cat > body.md <<EOF\nline1\nline2\nEOF"
		got := extractInlineHereDocBodyForTarget(cmd, "body.md")
		if got != "line1\nline2" {
			t.Errorf("got %q", got)
		}
	})
	t.Run("no match", func(t *testing.T) {
		got := extractInlineHereDocBodyForTarget("gh issue create --title test", "body.md")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestReadRemoteArtifactBodyFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "body.md"), "test body content\n")

	got := readRemoteArtifactBodyFile(dir, "body.md")
	if got != "test body content\n" {
		t.Errorf("got %q, want %q", got, "test body content\n")
	}

	got = readRemoteArtifactBodyFile(dir, "")
	if got != "" {
		t.Errorf("expected empty for empty path, got %q", got)
	}

	got = readRemoteArtifactBodyFile(dir, "-")
	if got != "" {
		t.Errorf("expected empty for '-' path, got %q", got)
	}
}

func TestRemoteArtifactCLIName(t *testing.T) {
	tests := []struct {
		prov string
		want string
	}{
		{"github", "gh"},
		{"gitlab", "glab"},
		{"unknown", "remote"},
	}
	for _, tt := range tests {
		got := remoteArtifactCLIName(remoteArtifactCommand{provider: tt.prov})
		if got != tt.want {
			t.Errorf("remoteArtifactCLIName(%q) = %q, want %q", tt.prov, got, tt.want)
		}
	}
}

func TestAllDigits(t *testing.T) {
	if allDigits("") {
		t.Error("empty should not be all digits")
	}
	if !allDigits("12345") {
		t.Error("12345 should be all digits")
	}
	if allDigits("12a45") {
		t.Error("12a45 should not be all digits")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

package commandparse

import "testing"

// ParseExactIssueOpsArgs는 hook이 전체 명령 문자열 대신 argv를 갖고 있을 때의
// 진입점이다. 문자열 진입점(ParseExactIssueOpsCommand)과 같은 exact 규칙을
// argv 형태로 지켜야 한다 — 특히 2-단어 하위명령 뒤에 플래그가 오면
// fail-closed로 거부한다.
func TestParseExactIssueOpsArgsMatchesCommandForms(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantOK   bool
		wantPath string
		wantArgv []string
	}{
		{
			name:     "single word with flags",
			args:     []string{"status", "--id", "io-1", "--json"},
			wantOK:   true,
			wantPath: "status",
			wantArgv: []string{"status", "--id", "io-1", "--json"},
		},
		{
			name:     "two word subcommand",
			args:     []string{"execution", "claim", "--id", "io-1"},
			wantOK:   true,
			wantPath: "execution claim",
			wantArgv: []string{"execution", "claim", "--id", "io-1"},
		},
		{
			name:   "two word group followed by flag must fail closed",
			args:   []string{"execution", "--id", "io-1"},
			wantOK: false,
		},
		{
			name:   "empty args must fail closed",
			args:   nil,
			wantOK: false,
		},
		{
			// ParseExactIssueOpsArgs는 최상위 `issueops` 이후 argv를 받는
			// 진입점이다. 상위 명령 검증은 호출자 책임이라 daemon/status도
			// path로 파싱된다(문서화된 계약).
			name:     "non-issueops path parses as its own path",
			args:     []string{"daemon", "status"},
			wantOK:   true,
			wantPath: "daemon",
			wantArgv: []string{"daemon", "status"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseExactIssueOpsArgs(tc.args)
			if ok != tc.wantOK {
				t.Fatalf("ParseExactIssueOpsArgs(%q) ok=%v want=%v", tc.args, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Path != tc.wantPath {
				t.Fatalf("path=%q want=%q", got.Path, tc.wantPath)
			}
			if len(got.Tokens) != len(tc.wantArgv)+1 {
				t.Fatalf("tokens=%q want argv prefix issueops + %q", got.Tokens, tc.wantArgv)
			}
			if got.Tokens[0] != "issueops" {
				t.Fatalf("tokens must start with the issueops envelope: %q", got.Tokens)
			}
			for i, want := range tc.wantArgv {
				if got.Tokens[i+1] != want {
					t.Fatalf("tokens=%q want %q at %d", got.Tokens, want, i+1)
				}
			}
		})
	}
}

// IssueOpsLifecycleIDFlag은 하위 세션 명령이 부모 lifecycle을, 나머지가
// 자신의 lifecycle을 가리키게 하는 경로 규칙이다. 위임 경로만 --parent다.
func TestIssueOpsLifecycleIDFlagRoutesDelegationToParent(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"status", "--id"},
		{"phase", "--id"},
		{"execution claim", "--id"},
		{"child start", "--parent"},
		{"child status", "--parent"},
		{"child accept", "--parent"},
		{"child reject", "--parent"},
		{"child drop", "--parent"},
	}
	for _, tc := range cases {
		if got := IssueOpsLifecycleIDFlag(tc.path); got != tc.want {
			t.Fatalf("IssueOpsLifecycleIDFlag(%q)=%q want %q", tc.path, got, tc.want)
		}
	}
}

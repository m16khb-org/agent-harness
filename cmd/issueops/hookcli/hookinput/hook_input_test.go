package hookinput

import "testing"

func TestRepoFromHookInputUsesTopLevelAndNestedValues(t *testing.T) {
	cases := map[string]string{
		`{"cwd":" /repo "}`:                                    "/repo",
		`{"repo":"/explicit","cwd":"/ignored"}`:                "/explicit",
		`{"hook_input":{"project_dir":"/nested"}}`:             "/nested",
		`{"cwd":"","hook_input":{"workspace_root":"/nested"}}`: "/nested",
		`{}`:       "",
		``:         "",
		`not json`: "",
	}
	for input, want := range cases {
		if got := RepoFromHookInput([]byte(input)); got != want {
			t.Fatalf("RepoFromHookInput(%s) = %q, want %q", input, got, want)
		}
	}
}

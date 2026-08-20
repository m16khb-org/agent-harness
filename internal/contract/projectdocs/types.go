// Package projectdocs는 projectdocs capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package projectdocs

type ProjectDocsReviseRequest struct {
	RepoRoot       string   `json:"repo_root"`
	RelPath        string   `json:"rel_path"`
	Content        string   `json:"content"`
	ExpectedSHA256 string   `json:"expected_sha256,omitempty"`
	Summary        string   `json:"summary"`
	Evidence       []string `json:"evidence,omitempty"`
	Confirm        bool     `json:"confirm"`
}

type ProjectDocsReviseResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	Action        string   `json:"action"`
	Confirmed     bool     `json:"confirmed"`
	DryRun        bool     `json:"dry_run"`
	GeneratedAt   string   `json:"generated_at"`
	CurrentSHA256 string   `json:"current_sha256,omitempty"`
	NextSHA256    string   `json:"next_sha256"`
	Bytes         int      `json:"bytes"`
	Summary       string   `json:"summary"`
	Evidence      []string `json:"evidence,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type ProjectDocsAppendRequest struct {
	RepoRoot     string   `json:"repo_root"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Context      string   `json:"context,omitempty"`
	Resolution   string   `json:"resolution,omitempty"`
	Decision     string   `json:"decision,omitempty"`
	Evidence     []string `json:"evidence,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Consequences string   `json:"consequences,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type ProjectDocsAppendResult struct {
	OK            bool     `json:"ok"`
	Kind          string   `json:"kind"`
	RecordKind    string   `json:"record_kind"`
	RepoRoot      string   `json:"repo_root"`
	RelPath       string   `json:"rel_path"`
	Path          string   `json:"path"`
	GeneratedAt   string   `json:"generated_at"`
	BytesAppended int      `json:"bytes_appended"`
	SHA256        string   `json:"sha256"`
	Warnings      []string `json:"warnings,omitempty"`
}

type ProjectDocsReadResult struct {
	OK          bool     `json:"ok"`
	Kind        string   `json:"kind"`
	RepoRoot    string   `json:"repo_root"`
	RelPath     string   `json:"rel_path"`
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Content     string   `json:"content,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	GeneratedAt string   `json:"generated_at"`
	Warnings    []string `json:"warnings,omitempty"`
}

type ProjectDocsRouteResult struct {
	OK          bool                   `json:"ok"`
	Kind        string                 `json:"kind"`
	RepoRoot    string                 `json:"repo_root"`
	Task        string                 `json:"task"`
	GeneratedAt string                 `json:"generated_at"`
	Docs        []ProjectDocRouteEntry `json:"docs"`
	Warnings    []string               `json:"warnings,omitempty"`
}

type ProjectDocRouteEntry struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	Exists  bool   `json:"exists"`
}

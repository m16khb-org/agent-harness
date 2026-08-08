// Package projectdoc는 프로젝트 프로필 DTO를 소유한다.
package projectdoc

type ProjectProfile struct {
	VCS             ProjectVCSProfile `json:"vcs"`
	Languages       []string          `json:"languages"`
	PackageManagers []string          `json:"package_managers,omitempty"`
	ProjectTypes    []string          `json:"project_types,omitempty"`
	Frameworks      []string          `json:"frameworks,omitempty"`
	Monorepo        bool              `json:"monorepo"`
	Evidence        []string          `json:"evidence,omitempty"`
}
type ProjectVCSProfile struct {
	Provider   string `json:"provider"`
	Hosting    string `json:"hosting"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemoteName string `json:"remote_name,omitempty"`
}

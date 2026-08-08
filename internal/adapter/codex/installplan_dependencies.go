package codex

import (
	"os"

	"agent-harness/internal/port"
)

// InstallPlan은 이 adapter가 실제로 쓰는 설치 계획 조작만 선언한다. 계획을 어떻게
// 누적하고 마감하는지는 composition root가 고른 구현의 몫이다.
type InstallPlan interface {
	Err(err error)
	Errs(errs []error)
	File(file port.InstallFile, err error)
	Files(files []port.InstallFile)
	Link(link port.InstallLink, err error)
	Links(links []port.InstallLink)
	Message(msg string)
	Messages(msgs []string)
	Finish() (port.HostInstallResult, error)
}

// 설치 계획 생성과 파일 쓰기는 composition root가 설치한다.
var (
	NewInstallPlan func(host string, dryRun bool) InstallPlan
	WriteJSONPlan  func(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	WriteTextPlan  func(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	TOMLString     func(value string) string
)

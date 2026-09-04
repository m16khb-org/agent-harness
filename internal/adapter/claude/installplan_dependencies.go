package claude

import (
	"os"

	"issueops/internal/port"
)

type InstallPlan = port.InstallPlan

// 설치 계획 생성과 파일 쓰기는 composition root가 설치한다.
var (
	NewInstallPlan func(host string, dryRun bool) InstallPlan
	WriteJSONPlan  func(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	WriteTextPlan  func(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	TOMLString     func(value string) string
)

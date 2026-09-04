package agy

import (
	"os"

	"issueops/internal/port"
)

type InstallPlan = port.InstallPlan

var (
	NewInstallPlan func(host string, dryRun bool) InstallPlan
	WriteJSONPlan  func(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error)
	WriteTextPlan  func(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error)

	CaptureNativeActivationEvidence func(host, surface, path, semanticSHA256 string) (port.NativeActivationEvidence, error)
	EnsureSymlinkPlan               func(target, path string, dryRun bool) (port.InstallLink, error)
	PlanHostSkillLinks              func(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error)
	SemanticSHA256                  func(value any) (string, error)
	MCPCatalogSHA256                func() (string, error)
)

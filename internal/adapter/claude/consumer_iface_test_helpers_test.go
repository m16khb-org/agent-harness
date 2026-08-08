package claude

import (
	"agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	NewInstallPlan = func(host string, dryRun bool) InstallPlan { return installutil.NewPlan(host, dryRun) }
	TOMLString = installutil.TOMLString
	WriteJSONPlan = installutil.WriteJSONPlan
	WriteTextPlan = installutil.WriteTextPlan
}

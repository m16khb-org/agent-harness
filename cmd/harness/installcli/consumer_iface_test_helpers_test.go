package installcli

import (
	claudehpd "agent-harness/internal/adapter/claude"
	codexhpd "agent-harness/internal/adapter/codex"
	"agent-harness/internal/adapter/installutil"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	claudehpd.NewInstallPlan = func(host string, dryRun bool) claudehpd.InstallPlan { return installutil.NewPlan(host, dryRun) }
	claudehpd.TOMLString = installutil.TOMLString
	claudehpd.WriteJSONPlan = installutil.WriteJSONPlan
	claudehpd.WriteTextPlan = installutil.WriteTextPlan
	codexhpd.NewInstallPlan = func(host string, dryRun bool) codexhpd.InstallPlan { return installutil.NewPlan(host, dryRun) }
	codexhpd.TOMLString = installutil.TOMLString
	codexhpd.WriteJSONPlan = installutil.WriteJSONPlan
	codexhpd.WriteTextPlan = installutil.WriteTextPlan
}

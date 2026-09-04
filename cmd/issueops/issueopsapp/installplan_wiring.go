package issueopsapp

import (
	agyadapter "issueops/internal/adapter/agy"
	claudeadapter "issueops/internal/adapter/claude"
	codexadapter "issueops/internal/adapter/codex"
	"issueops/internal/adapter/installutil"
	omoadapter "issueops/internal/adapter/omo"
)

// configureInstallPlans는 host adapter에 설치 계획 구현을 조립한다.
//
// host들은 계획을 쌓는 방식이 같고 담는 내용만 다르다. 그 구현을 고르는 것은
// composition root의 결정이다.
func configureInstallPlans() {
	claudeadapter.NewInstallPlan = func(host string, dryRun bool) claudeadapter.InstallPlan {
		return installutil.NewPlan(host, dryRun)
	}
	claudeadapter.WriteJSONPlan = installutil.WriteJSONPlan
	claudeadapter.WriteTextPlan = installutil.WriteTextPlan
	claudeadapter.TOMLString = installutil.TOMLString

	codexadapter.NewInstallPlan = func(host string, dryRun bool) codexadapter.InstallPlan {
		return installutil.NewPlan(host, dryRun)
	}
	codexadapter.WriteJSONPlan = installutil.WriteJSONPlan
	codexadapter.WriteTextPlan = installutil.WriteTextPlan
	codexadapter.TOMLString = installutil.TOMLString

	omoadapter.NewInstallPlan = func(host string, dryRun bool) omoadapter.InstallPlan {
		return installutil.NewPlan(host, dryRun)
	}
	omoadapter.WriteJSONPlan = installutil.WriteJSONPlan
	omoadapter.WriteTextPlan = installutil.WriteTextPlan

	agyadapter.NewInstallPlan = func(host string, dryRun bool) agyadapter.InstallPlan {
		return installutil.NewPlan(host, dryRun)
	}
	agyadapter.WriteJSONPlan = installutil.WriteJSONPlan
	agyadapter.WriteTextPlan = installutil.WriteTextPlan
}

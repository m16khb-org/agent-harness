package harnessapp

import (
	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/adapter/installutil"
)

// configureInstallPlans는 host adapter에 설치 계획 구현을 조립한다.
//
// 두 host는 계획을 쌓는 방식이 같고 담는 내용만 다르다. 그 구현을 고르는 것은
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
}

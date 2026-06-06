package core

import corepreflight "agent-harness/internal/core/preflight"

type PreflightResult = corepreflight.PreflightResult
type RemoteInfo = corepreflight.RemoteInfo
type CommitInfo = corepreflight.CommitInfo

func GitPreflight(target, harnessRoot string) PreflightResult {
	return corepreflight.GitPreflight(target, harnessRoot)
}

func GitCmd(dir string, args ...string) (int, string, string) {
	return corepreflight.GitCmd(dir, args...)
}

func GitOut(dir string, args ...string) string {
	return corepreflight.GitOut(dir, args...)
}

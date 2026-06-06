package core

import "agent-harness/internal/core/remoteartifact"

func koreanRemoteArtifactBlockReason(req HookToolUseLifecycleRequest) string {
	return remoteartifact.KoreanBlockReason(req.Tool, req.Command, req.Repo)
}

func vcsIssueLinkingBlockReason(req HookToolUseLifecycleRequest) string {
	return remoteartifact.VCSIssueLinkingBlockReason(req.Tool, req.Command, req.Repo)
}

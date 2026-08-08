package hookcli

import (
	hookpromptcontract "agent-harness/internal/contract/hookprompt"
)

// prompt hint 구성은 저장소를 읽는다. 구현은 composition root가 설치한다.
var (
	BuildUserPromptMCPHints     func(req hookpromptcontract.HookUserPromptRequest) hookpromptcontract.HookUserPromptResult
	StopOrchestrationRelayFacts func(repo string) string
)

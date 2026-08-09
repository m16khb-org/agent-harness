package commandparse

import "testing"

func TestExactSelfVerifyVerificationAcceptsQuickAndGovernedFullForms(t *testing.T) {
	for _, command := range []string{
		"./bin/agent-harness self-verify --seed=100 --target-score=95 --json",
		"agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
	} {
		if !ExactSelfVerifyVerification(command) {
			t.Fatalf("governed self-verify form was rejected: %q", command)
		}
	}
}

func TestExactSelfVerifyVerificationRejectsUnsafeFullVariants(t *testing.T) {
	for _, command := range []string{
		"./bin/agent-harness self-verify --iterations=10 --json",
		"./bin/agent-harness self-verify --full --iterations=0 --json",
		"./bin/agent-harness self-verify --full --iterations=invalid --json",
		"./bin/agent-harness self-verify --full --full --json",
		"./bin/agent-harness self-verify --full --iterations=10 --iterations=11 --json",
		"./bin/agent-harness self-verify --full --progress=none --json",
		"./bin/agent-harness self-verify --full --write --json",
		"./bin/agent-harness self-verify --full --json > result.json",
		"./bin/agent-harness self-verify --full --json && touch result",
	} {
		if ExactSelfVerifyVerification(command) {
			t.Fatalf("unsafe self-verify form was accepted: %q", command)
		}
	}
}

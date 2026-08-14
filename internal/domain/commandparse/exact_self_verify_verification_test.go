package commandparse

import "testing"

func TestExactSelfVerifyVerificationAdmitsOnlyCurrentDeterministicForms(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		want    bool
	}{
		{
			name:    "quick verification",
			command: "./bin/agent-harness self-verify --seed=100 --target-score=95 --json",
			want:    true,
		},
		{
			name:    "collect all verification",
			command: "./bin/agent-harness self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    true,
		},
		{
			name:    "collect all accepts PATH executable",
			command: "agent-harness self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    true,
		},
		{
			name:    "collect all accepts non-relative bin executable",
			command: "bin/agent-harness self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    true,
		},
		{
			name:    "retired full flag",
			command: "./bin/agent-harness self-verify --full --seed=100 --target-score=95 --llm-eval=false --json",
			want:    false,
		},
		{
			name:    "retired iterations flag",
			command: "./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --llm-eval=false --json",
			want:    false,
		},
		{
			name:    "retired goal flag",
			command: "./bin/agent-harness self-verify --goal=quality --seed=100 --target-score=95 --json",
			want:    false,
		},
		{
			name:    "state write flag",
			command: "./bin/agent-harness self-verify --seed=100 --target-score=95 --save-state --json",
			want:    false,
		},
		{
			name:    "redirect",
			command: "./bin/agent-harness self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json > result.json",
			want:    false,
		},
		{
			name:    "control operator",
			command: "./bin/agent-harness self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json && git status",
			want:    false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExactSelfVerifyVerification(tc.command); got != tc.want {
				t.Fatalf("ExactSelfVerifyVerification(%q) = %t, want %t", tc.command, got, tc.want)
			}
		})
	}
}

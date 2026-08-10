package commandparse

import "testing"

func TestExactSelfVerifyVerificationAdmitsOnlyKnownQuickOrFullVerificationForms(t *testing.T) {
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
			name:    "exact full verification",
			command: "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    true,
		},
		{
			name:    "full verification rejects PATH executable",
			command: "agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    false,
		},
		{
			name:    "full verification rejects non-relative bin executable",
			command: "bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    false,
		},
		{
			name:    "full verification missing full flag",
			command: "./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    false,
		},
		{
			name:    "duplicate full flag",
			command: "./bin/agent-harness self-verify --full --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    false,
		},
		{
			name:    "incorrect iteration count",
			command: "./bin/agent-harness self-verify --full --iterations=9 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json",
			want:    false,
		},
		{
			name:    "unknown write flag",
			command: "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --save-state --json",
			want:    false,
		},
		{
			name:    "redirect",
			command: "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json > result.json",
			want:    false,
		},
		{
			name:    "control operator",
			command: "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json && git status",
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

package rerun

import (
	"fmt"
	"strconv"
)

func SelfVerifyRerunCommands(failedStep string, iterations int, baseSeed int64, targetScore float64) []string {
	commands := []string{}
	if command, ok := SelfVerifyStepRerunCommand(failedStep); ok {
		commands = append(commands, command)
	}
	if iterations < 10 {
		iterations = 10
	}
	commands = append(commands, fmt.Sprintf("./bin/agent-harness self-verify --iterations=%d --seed=%d --target-score=%s --progress=jsonl --json", iterations, baseSeed, FormatScore(targetScore)))
	return commands
}

func SelfVerifyStepRerunCommand(label string) (string, bool) {
	switch label {
	case "go test":
		return "go test ./... -count=1", true
	case "contract golden tests":
		return "go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1", true
	case "risk QA tier":
		return "go vet ./... && go test -race ./... -count=1", true
	case "go build":
		return "go build -o bin/agent-harness ./cmd/harness", true
	case "inspect smoke":
		return "./bin/agent-harness inspect --json", true
	case "docs index smoke":
		return "./bin/agent-harness docs --json", true
	case "candidate export":
		return "tmp_state=\"$(mktemp -d)\" && HARNESS_STATE_DIR=\"$tmp_state\" ./bin/agent-harness self-verify candidates --save-state --state-key self-verify-candidates-test --json && HARNESS_STATE_DIR=\"$tmp_state\" ./bin/agent-harness state read --key self-verify-candidates-test --json; rm -rf \"$tmp_state\"", true
	case "step budget baseline":
		return "tmp_state=\"$(mktemp -d)\" && HARNESS_STATE_DIR=\"$tmp_state\" ./bin/agent-harness self-verify --seed=100 --target-score=95 --save-state --state-key self-verify-budget-baseline --json && HARNESS_STATE_DIR=\"$tmp_state\" ./bin/agent-harness self-verify compare --baseline-key self-verify-budget-baseline --candidate-key self-verify-budget-baseline --json; rm -rf \"$tmp_state\"", true
	case "install dry-run smoke":
		return "tmp_home=\"$(mktemp -d)\" tmp_root=\"$(mktemp -d)\" && mkdir -p \"$tmp_root/skills/atomic-commit-push\" && printf -- '---\\nname: atomic-commit-push\\ndescription: smoke\\n---\\n' > \"$tmp_root/skills/atomic-commit-push/SKILL.md\" && HOME=\"$tmp_home\" CODEX_HOME=\"$tmp_home/.codex\" HARNESS_ROOT=\"$tmp_root\" ./bin/agent-harness install-native --dry-run --project-local --json; rm -rf \"$tmp_home\" \"$tmp_root\"", true
	case "command policy smoke":
		return "./bin/agent-harness policy check --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short", true
	case "command audit smoke":
		return "tmp_audit=$(mktemp) && HARNESS_AUDIT_LOG=\"$tmp_audit\" ./bin/agent-harness policy audit --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short", true
	case "contract check":
		return "./bin/agent-harness contract check --json", true
	case "worker lifecycle smoke":
		return "tmp_worker=$(mktemp -d) && HARNESS_WORKER_DIR=\"$tmp_worker\" ./bin/agent-harness worker enqueue --kind smoke --json", true
	case "MCP smoke":
		return "./bin/agent-harness mcp", true
	case "state roundtrip":
		return "tmp_state=\"$(mktemp -d)\" && HARNESS_STATE_DIR=\"$tmp_state\" ./bin/agent-harness state migrate --json; rm -rf \"$tmp_state\"", true
	case "parallel isolation":
		return "./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json", true
	case "daemon resilience":
		return "tmp_daemon=\"$(mktemp -d)\" && HARNESS_DAEMON_DIR=\"$tmp_daemon\" ./bin/agent-harness daemon start --json && HARNESS_DAEMON_DIR=\"$tmp_daemon\" ./bin/agent-harness daemon stop --json; rm -rf \"$tmp_daemon\"", true
	case "preflight fuzz":
		return "./bin/agent-harness preflight --json \"$PWD\"", true
	case "native integration":
		return "./scripts/install-native.sh && ./bin/agent-harness install-native --dry-run --json", true
	case "redaction audit", "QA gate", "harness invariants":
		return "go test ./cmd/harness -run Test -count=1", true
	default:
		return "", false
	}
}

func FormatScore(score float64) string {
	if score == float64(int64(score)) {
		return strconv.FormatInt(int64(score), 10)
	}
	return strconv.FormatFloat(score, 'f', -1, 64)
}

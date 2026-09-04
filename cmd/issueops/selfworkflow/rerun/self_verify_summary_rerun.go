package rerun

import (
	"fmt"
	"strconv"
)

func SelfVerifyRerunCommands(failedStep string, baseSeed int64, targetScore float64) []string {
	commands := []string{}
	if command, ok := SelfVerifyStepRerunCommand(failedStep); ok {
		commands = append(commands, command)
	}
	commands = append(commands, fmt.Sprintf("./bin/issueops self-verify --collect-all-steps --seed=%d --target-score=%s --llm-eval=false --progress=jsonl --json", baseSeed, FormatScore(targetScore)))
	return commands
}

func SelfVerifyStepRerunCommand(label string) (string, bool) {
	switch label {
	case "go test":
		return "go test ./... -count=1", true
	case "contract golden tests":
		return "go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1", true
	case "risk QA tier":
		return "go vet ./... && go test -race ./... -count=1", true
	case "go build":
		return "go build -o bin/issueops ./cmd/issueops", true
	case "gofmt":
		return "gofmt -l $(git ls-files '*.go')", true
	case "inspect smoke":
		return "./bin/issueops inspect --json", true
	case "docs index smoke":
		return "./bin/issueops docs --json", true
	case "candidate export":
		return "tmp_state=\"$(mktemp -d)\" && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops self-verify candidates --save-state --state-key self-verify-candidates-test --json && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops state read --key self-verify-candidates-test --json; rm -rf \"$tmp_state\"", true
	case "step budget baseline":
		return "tmp_state=\"$(mktemp -d)\" && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --save-state --state-key self-verify-budget-baseline --json && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops self-verify compare --baseline-key self-verify-budget-baseline --candidate-key self-verify-budget-baseline --json; rm -rf \"$tmp_state\"", true
	case "install dry-run smoke":
		return "tmp_home=\"$(mktemp -d)\" tmp_root=\"$(mktemp -d)\" && mkdir -p \"$tmp_root/skills/atomic-commit-push\" && printf -- '---\\nname: atomic-commit-push\\ndescription: smoke\\n---\\n' > \"$tmp_root/skills/atomic-commit-push/SKILL.md\" && HOME=\"$tmp_home\" CODEX_HOME=\"$tmp_home/.codex\" ISSUEOPS_ROOT=\"$tmp_root\" ./bin/issueops install --dry-run --project-local --json; rm -rf \"$tmp_home\" \"$tmp_root\"", true
	case "command policy smoke":
		return "./bin/issueops policy check --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short", true
	case "command audit smoke":
		return "tmp_audit=$(mktemp) && ISSUEOPS_AUDIT_LOG=\"$tmp_audit\" ./bin/issueops policy audit --workspace-root \"$PWD\" --cwd \"$PWD\" --json -- git status --short", true
	case "contract check":
		return "./bin/issueops contract check --json", true
	case "worker lifecycle smoke":
		return "tmp_worker=$(mktemp -d) && ISSUEOPS_WORKER_DIR=\"$tmp_worker\" ./bin/issueops worker enqueue --kind smoke --json", true
	case "MCP smoke":
		return "./bin/issueops mcp", true
	case "state roundtrip":
		return "tmp_state=\"$(mktemp -d)\" && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops state write --key smoke --value smoke --json && ISSUEOPS_STATE_DIR=\"$tmp_state\" ./bin/issueops state read --key smoke --json; rm -rf \"$tmp_state\"", true
	case "parallel isolation":
		return "./bin/issueops self-verify --collect-all-steps --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json", true
	case "daemon resilience":
		return "tmp_daemon=\"$(mktemp -d)\" && ISSUEOPS_DAEMON_DIR=\"$tmp_daemon\" ./bin/issueops daemon start --json && ISSUEOPS_DAEMON_DIR=\"$tmp_daemon\" ./bin/issueops daemon stop --json; rm -rf \"$tmp_daemon\"", true
	case "preflight fuzz":
		return "./bin/issueops preflight --json \"$PWD\"", true
	case "native integration":
		return "./scripts/install-native.sh && ./bin/issueops install --dry-run --json", true
	case "redaction audit", "QA gate", "harness invariants":
		return "go test ./cmd/issueops -run Test -count=1", true
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

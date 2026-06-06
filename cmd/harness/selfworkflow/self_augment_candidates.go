package selfworkflow

const (
	selfAugmentCandidateStatusOpen      = "open"
	selfAugmentCandidateStatusSatisfied = "already_satisfied"
)

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	base := []SelfAugmentCandidate{
		{
			ID: "loop-taxonomy-score-gates", Title: "Separate self-verification and self-augmentation loops and enforce >95 exit gates", Category: "quality",
			Impact: 99, Feasibility: 98, Novelty: 93, Risk: 8,
			WhyNow:       []string{"현재 self-augment가 실제로는 검증 루프 역할을 한다", "사용자가 테스트와 QA 포함 및 95점 게이트를 요구했다"},
			ExpectedGain: []string{"루프 이름/책임 혼동 제거", "검증 없는 종료 방지", "CLI/MCP/native skill 계약 일치"},
			VerifyWith:   []string{"go test ./...", "MCP/CLI golden", "harness self-verify --full --iterations=10 --target-score=95"},
		},
		{
			ID: "agent-skill-executor", Title: "Provide the self-augmentation loop as a native skill executor that creates real improvement diffs", Category: "feature",
			Impact: 97, Feasibility: 96, Novelty: 92, Risk: 12,
			WhyNow:       []string{"Go CLI는 LLM 판단/코드 편집 주체가 아니므로 skill 표면이 필요하다", "Codex/Claude 공용 하네스 목적과 맞다"},
			ExpectedGain: []string{"후보화→선택→구현→검증을 agent가 실제 수행", "GENIUS_THINK.md와 연구 앵커를 반복적으로 활용"},
			VerifyWith:   []string{"skill quick_validate", "install-native", "self-verify QA gate"},
		},
		{
			ID: "durable-augmentation-memory", Title: "Accumulate self-augmentation candidates, decisions, and failure lessons in state", Category: "memory",
			Impact: 93, Feasibility: 92, Novelty: 86, Risk: 20,
			WhyNow:       []string{"Reflexion식 언어 피드백을 다음 실행에 재사용하려면 durable memory가 필요하다"},
			ExpectedGain: []string{"동일 실패 반복 감소", "레포별 개선 히스토리 누적"},
			VerifyWith:   []string{"state roundtrip", "history/compare contract"},
		},
		{
			ID: "qa-dashboard-summary", Title: "Expand self-verification summaries into QA dashboard scorecards", Category: "observability",
			Impact: 91, Feasibility: 95, Novelty: 72, Risk: 10,
			WhyNow:       []string{"느린 step과 실패 위치만으로는 목표별 종료 판단이 부족하다"},
			ExpectedGain: []string{"목표별 점수와 증거 label을 바로 확인", "CI/agent가 gate를 기계적으로 판정"},
			VerifyWith:   []string{"response_contract golden", "self-verify --json schema inspection"},
		},
		{
			ID: "reflexion-state-memory", Title: "Store self-augmentation failure lessons in state", Category: "memory",
			Impact: 89, Feasibility: 91, Novelty: 84, Risk: 16,
			WhyNow:       []string{"반복 실패를 다음 cycle에서 활용하려면 언어 피드백 저장소가 필요하다"},
			ExpectedGain: []string{"실패 원인 재발 감소", "레포별 개선 이력 검색 가능"},
			VerifyWith:   []string{"state_write/read"},
		},
		{
			ID: "qa-race-tier", Title: "Conditionally attach risk-based race/static QA tier to the self-verification loop", Category: "qa",
			Impact: 88, Feasibility: 88, Novelty: 79, Risk: 22,
			WhyNow:       []string{"모든 반복에서 race를 돌리면 과하지만 concurrency/policy 변경에는 필요하다"},
			ExpectedGain: []string{"검증 비용과 신뢰도 균형", "고위험 변경에 대한 추가 방어"},
			VerifyWith:   []string{"go test -race ./... -count=1", "targeted package tests"},
		},
		{
			ID: "adapter-contract-matrix", Title: "Codex/Claude adapter 계약을 matrix fixture로 고정", Category: "test",
			Impact: 87, Feasibility: 90, Novelty: 75, Risk: 14,
			WhyNow:       []string{"설치 방식과 host adapter가 늘면서 core/adapter 계약 회귀 가능성이 커졌다"},
			ExpectedGain: []string{"SOLID 구조의 포트 계약 보존", "host별 출력 drift 조기 탐지"},
			VerifyWith:   []string{"internal/core install tests", "adapter golden fixtures"},
		},
		{
			ID: "install-dry-run-mode", Title: "install-native에 dry-run planning mode 추가", Category: "safety",
			Impact: 85, Feasibility: 86, Novelty: 70, Risk: 18,
			WhyNow:       []string{"user/global 설치는 안전하지만 쓰기 전 preview가 있으면 신뢰도가 오른다"},
			ExpectedGain: []string{"destructive concern 감소", "다른 레포 설치 전 diff 예측"},
			VerifyWith:   []string{"install tests", "temporary HOME smoke"},
		},
		{
			ID: "genius-mermaid-lint", Title: "GENIUS_THINK Mermaid 규칙을 docs QA lint로 승격", Category: "docs",
			Impact: 78, Feasibility: 94, Novelty: 67, Risk: 8,
			WhyNow:       []string{"문서 생성 시 Mermaid 파싱 오류를 반복 방지할 수 있다"},
			ExpectedGain: []string{"문서 품질 안정화", "한글 Mermaid 노드 규칙 자동 확인"},
			VerifyWith:   []string{"docs QA gate", "markdown fixture lint"},
		},
		{
			ID: "performance-baseline", Title: "self-verify slowest_steps를 성능 baseline regression으로 승격", Category: "performance",
			Impact: 83, Feasibility: 87, Novelty: 74, Risk: 16,
			WhyNow:       []string{"현재 elapsed regression은 summary 단위라 병목 위치 추적이 약하다"},
			ExpectedGain: []string{"느린 step regression 탐지", "성능 개선 후보 자동 생성"},
			VerifyWith:   []string{"self-verify compare fixtures", "slowest_steps golden"},
		},
		{
			ID: "repo-local-augmentation-sandbox", Title: "Harden workspace-boundary sandboxing for self-augmenting other repositories", Category: "safety",
			Impact: 90, Feasibility: 82, Novelty: 83, Risk: 24,
			WhyNow:       []string{"하네스가 여러 레포에서 쓰이면 repo별 권한·상태 경계가 핵심이다"},
			ExpectedGain: []string{"레포별 독립 self-augment 실행", "root 밖 접근 회귀 방지"},
			VerifyWith:   []string{"command_policy_check", "temp repo integration tests"},
		},
		{
			ID: "cli-mcp-adapter-split", Title: "Split CLI usage and MCP schema catalog into adapter-owned packages", Category: "architecture",
			Impact: 94, Feasibility: 86, Novelty: 76, Risk: 24,
			WhyNow:       []string{"cmd/harness가 routing, usage, MCP schema를 모두 소유하면 기능 추가 때 drift 위험이 커진다"},
			ExpectedGain: []string{"CLI/MCP 표면을 독립적으로 테스트", "후속 worker와 contract 기능 추가 비용 감소"},
			VerifyWith:   []string{"internal/adapter/cli tests", "internal/adapter/mcp tests", "usage golden"},
		},
		{
			ID: "dto-compatibility-contract", Title: "Expose a CLI/MCP DTO compatibility contract and schema check", Category: "contract",
			Impact: 92, Feasibility: 88, Novelty: 78, Risk: 18,
			WhyNow:       []string{"worker와 audit 표면이 늘면 JSON response field drift를 기계적으로 확인해야 한다"},
			ExpectedGain: []string{"response DTO 변경 시 명시적 hash와 required field 확인", "MCP tool schema와 CLI usage를 같은 계약으로 검증"},
			VerifyWith:   []string{"harness contract check --json", "response contract golden"},
		},
		{
			ID: "candidate-refill-curriculum", Title: "Refill self-augmentation candidates after the original curriculum is satisfied", Category: "curriculum",
			Impact: 90, Feasibility: 91, Novelty: 82, Risk: 12,
			WhyNow:       []string{"기존 후보가 모두 satisfied가 되면 자가증강 루프가 다음 필요 기능을 선택하지 못한다"},
			ExpectedGain: []string{"완료된 후보는 audit history로 유지", "release/worker 등 후속 open 후보를 계속 노출"},
			VerifyWith:   []string{"self-augment --json open_candidate_ids", "self_augment_summary_test"},
		},
		{
			ID: "policy-audit-redaction", Title: "Add append-only redacted command-policy audit records", Category: "safety",
			Impact: 91, Feasibility: 87, Novelty: 77, Risk: 20,
			WhyNow:       []string{"실제 runner나 worker 확장 전 audit log와 secret redaction 기반이 필요하다"},
			ExpectedGain: []string{"명령 허용/거부 판단을 재현 가능하게 저장", "secret 원문 없는 JSONL 증거 확보"},
			VerifyWith:   []string{"policy audit CLI smoke", "redaction audit unit test"},
		},
		{
			ID: "worker-mvp-no-shell", Title: "Implement a no-shell local worker MVP for job lifecycle records", Category: "worker",
			Impact: 89, Feasibility: 84, Novelty: 80, Risk: 26,
			WhyNow:       []string{"Phase 4 worker는 필요하지만 첫 단계는 shell 실행 없는 queue/status/cancel로 제한해야 안전하다"},
			ExpectedGain: []string{"worker lifecycle DTO와 storage 경계 검증", "future process runner 도입 전 안전한 API 확보"},
			VerifyWith:   []string{"worker enqueue/status/list/cancel tests", "MCP worker tool smoke"},
		},
		{
			ID: "release-repro-pack", Title: "Create a clean-machine release and install reproducibility pack", Category: "release",
			Impact: 86, Feasibility: 78, Novelty: 68, Risk: 22,
			WhyNow:       []string{"adapter, contract, worker 기반이 들어오면 Phase 7 clean install 검증으로 넘어갈 수 있다"},
			ExpectedGain: []string{"새 환경 설치 절차 재현성", "tarball/Homebrew 여부 결정 근거"},
			VerifyWith:   []string{"release checklist", "temp HOME install smoke"},
		},
	}
	for i := range base {
		base[i].Status = selfAugmentCandidateStatusOpen
		base[i].Score = selfAugmentCandidateScore(base[i])
		markSatisfiedSelfAugmentCandidate(&base[i], signals)
	}
	return base
}

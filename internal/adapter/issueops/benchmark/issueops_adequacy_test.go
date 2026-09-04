package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"sort"
	"strings"
	"testing"
)

// A4 — scorer 검증 충분성(차원별 mutation suite)이다.
//
// live benchmark gate(issueops benchmark run)는 fixture의 기대 필드로 올바른
// artifact를 만드는 FromFixture의 PASSING 결과만 채점하므로, 구조상
// avg=min=100을 보고한다. 이는 SYNTHESIZER의 정상 동작만 증명할 뿐 SCORER가
// 결함을 잡는지는 증명하지 못한다. 검사 결과가 조용히 항상 ok가 된 차원(dead
// check)은 gate를 통과해, 단순 artifact 필드만 신호로 쓰는 19개 차원 중 약 7개
// (intent_understanding, plan_quality, tdd_quality, implementation_readiness,
// phase_control_quality, branch_worktree_gate_quality,
// worktree_cleanup_quality)의 기존 테스트를 모두 빠져나갈 수 있다.
//
// 이 suite는 각 차원에서 완전히 통과한 artifact를 변형해 판별 신호를 제거한 뒤,
// 해당 차원의 점수 행 자체가 실제 0으로 떨어짐을 증명한다. 항상 ok인 dead check는
// 행을 100으로 남겨 assertion을 실패시킨다. 함께 고장 난 sibling이 있더라도
// aggregate min/avg가 아닌 차원별 행(dimScore)을 읽으므로 이를 가릴 수 없다.
//
// 설계 메모(S2 review):
//   - 여러 차원이 artifact 필드를 공유하므로 "D를 변형해도 나머지 18개는 100"인
//     전역 격리는 구조적으로 불가능하다. 대신 각 행에 반드시 함께 떨어질 다른
//     차원(coupled)을 선언하고, 하락 차원 집합이 정확히 {D} ∪ coupled인지
//     검증한다. 전체 artifact를 비우는 과도한 변형도 이로써 잡는다.
//   - FULL fixture에는 critical-failure 규칙이 없으므로 각 mutation은
//     deterministic 차원 channel만 검증하며, critical이 발생하지 않아야 한다.
//     critical-failure 검출은 TestScoreIssueOpsBenchmarkArtifact*CriticalFailures가
//     별도로 담당한다.
//   - pioneer_skill_contribution과 skill_routing_fidelity는 metadata 조건부다.
//     mutation은 fixture metadata를 보존하고 artifact 신호
//     (PioneerSkillEvidence/RoutingTrace)만 비운다. 따라서 제외된 N/A가 아닌
//     실제 deterministic 0만이 검사가 실행됐음을 증명하며, !NotApplicable
//     assertion이 0으로 위장한 N/A를 거부한다.
//   - completeness guard는 테이블과 issueOpsBenchmarkDimensions를 양방향으로
//     묶어, 충분성 mutator가 없는 새 차원이 추가되지 못하게 한다(A5
//     dimension-count-regression 교훈).

func adequacyFixtureForTest() issueopscontract.IssueOpsBenchmarkFixture {
	return issueopscontract.IssueOpsBenchmarkFixture{
		ID:                 "adequacy-full",
		Title:              "Adequacy full fixture",
		UserPrompt:         "exercise every scoring dimension",
		PioneerSkillTarget: "database-design",
		ExpectedRouting:    []issueopscontract.SkillRouting{{Phase: "plan", Skill: "database-design"}},
		// 이 suite는 deterministic 차원 channel만 검증하므로 의도적으로
		// CriticalFailures를 두지 않는다.
	}
}

func adequacyArtifactForTest() issueopscontract.IssueOpsBenchmarkArtifact {
	a := completeBenchmarkArtifactForTest()
	a.PioneerSkillEvidence = coddKeywordEvidence
	a.RoutingTrace = []issueopscontract.SkillRouting{{Phase: "plan", Skill: "database-design"}}
	return a
}

func adequacyRow(score IssueOpsBenchmarkScore, dimension string) IssueOpsDimensionScore {
	for _, d := range score.DimensionScores {
		if d.Dimension == dimension {
			return d
		}
	}
	// Sentinel: 누락 차원은 live-100과 live-0 검증을 모두 실패시킨다.
	return IssueOpsDimensionScore{Dimension: dimension, Score: -1}
}

func adequacyDroppedDims(score IssueOpsBenchmarkScore) []string {
	var dropped []string
	for _, dim := range issueOpsBenchmarkDimensions {
		row := adequacyRow(score, dim)
		if row.Score == 0 && !row.NotApplicable {
			dropped = append(dropped, dim)
		}
	}
	sort.Strings(dropped)
	return dropped
}

func TestScoreIssueOpsBenchmarkArtifactEveryDimensionDiscriminates(t *testing.T) {
	fixture := adequacyFixtureForTest()

	// 기준선: 완전 통과 artifact는 모든 차원에서 실패 없이 실제 100점이어야 한다.
	// 따라서 아래 mutation은 정상 상태에서 시작하며, 신호를 제거하지 못한 no-op
	// mutator도 공허하게 통과하지 않고 mutated-row==0 assertion에 잡힌다.
	baseScore := ScoreIssueOpsBenchmarkArtifact(fixture, adequacyArtifactForTest())
	if !baseScore.Passed || len(baseScore.DeterministicFailures) != 0 || len(baseScore.CriticalFailures) != 0 {
		t.Fatalf("adequacy baseline must pass cleanly: %+v", baseScore)
	}
	for _, dim := range issueOpsBenchmarkDimensions {
		row := adequacyRow(baseScore, dim)
		if row.NotApplicable || row.Score != issueOpsBenchmarkMaxScore {
			t.Fatalf("baseline dimension %q must be live at 100, got %+v", dim, row)
		}
	}

	type adequacyCase struct {
		// mutate는 대상 차원의 판별 신호만 제거한다. 공유 필드 차원은 coupled에
		// 선언한 범위만 함께 제거한다.
		mutate func(issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact
		// coupled는 같은 artifact 필드를 읽어 반드시 함께 하락하는 다른 차원이다.
		// 비어 있으면 단일 축 mutation이다.
		coupled []string
	}

	cases := map[string]adequacyCase{
		"intent_understanding": {
			// ok = ProblemSummary!="" || IssueDraft!=""이므로 둘 다 비워야 한다.
			// IssueDraft를 비우면 공유 필드인 issue_quality도 함께 하락한다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.ProblemSummary = ""
				a.IssueDraft = ""
				return a
			},
			coupled: []string{"issue_quality"},
		},
		"issue_quality": {
			// issue 전용 label-decision 절만 깨뜨린다. ProblemSummary와 IssueDraft는
			// 비어 있지 않아 intent_understanding은 100점을 유지한다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.IssueDraft = strings.ReplaceAll(a.IssueDraft, "선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n", "")
				return a
			},
		},
		"domain_contract_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.DomainContractEvidence = ""
				return a
			},
		},
		"plan_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.Plan = "Implement the change."
				return a
			},
		},
		"api_doc_gate_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.APIDocGateEvidence = ""
				return a
			},
		},
		"live_evidence_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.LiveEvidenceMatrix = ""
				return a
			},
		},
		"task_decomposition": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.TaskBreakdown = ""
				return a
			},
		},
		"tdd_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.TDDPlan = ""
				return a
			},
		},
		"subagent_orchestration": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.SubagentPrompts = ""
				return a
			},
		},
		"review_feedback_accountability": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.ReviewFeedbackEvidence = ""
				return a
			},
		},
		"implementation_readiness": {
			// ok = BranchName!="" && WorktreePath!=""이다. BranchName을 비우면
			// feature/ prefix를 보는 branch_worktree_gate_quality도 함께 하락한다.
			// 두 차원은 필드를 공유하며 implementation_readiness에는 분리 가능한
			// 전용 신호가 없다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.BranchName = ""
				return a
			},
			coupled: []string{"branch_worktree_gate_quality"},
		},
		"pr_mr_quality": {
			// PRDraft에서 issue-link 절만 제거한다. GuidelineRef는 보존하므로
			// issue_quality가 공유하는 guideline 절에는 영향이 없다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.PRDraft = strings.ReplaceAll(a.PRDraft, "Issue: https://example.com/acme/issueops/issues/1\n", "")
				return a
			},
		},
		"phase_control_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.PhaseChoices = ""
				return a
			},
		},
		"branch_worktree_gate_quality": {
			// BranchName은 비어 있지 않게 유지해 implementation_readiness는 100점을
			// 유지하고, feature/ prefix만 제거해 gate 차원만 실패시킨다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.BranchName = "main"
				return a
			},
		},
		"isolation_compliance": {
			// worktree 밖에서 구현한 것으로 만든다. WorktreePath/BranchName은 남겨
			// implementation_readiness와 gate 차원은 100점을 유지한다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.ImplementationLocation = "/elsewhere/outside-worktree"
				return a
			},
		},
		"completion_hygiene_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.CompletionHygiene = ""
				return a
			},
		},
		"worktree_cleanup_quality": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.WorktreeCleanup = ""
				return a
			},
		},
		"pioneer_skill_contribution": {
			// fixture의 PioneerSkillTarget을 보존해 차원이 계속 적용되게 하고,
			// artifact 신호만 제거한다.
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.PioneerSkillEvidence = ""
				return a
			},
		},
		"skill_routing_fidelity": {
			mutate: func(a issueopscontract.IssueOpsBenchmarkArtifact) issueopscontract.IssueOpsBenchmarkArtifact {
				a.RoutingTrace = nil
				return a
			},
		},
	}

	// 양방향 completeness guard: 모든 채점 차원에는 adequacy mutator가 있어야
	// 하며 테이블에는 알 수 없는 차원이 없어야 한다. 판별 능력을 증명하지 못한
	// 새 차원은 추가할 수 없다.
	if len(cases) != len(issueOpsBenchmarkDimensions) {
		t.Fatalf("adequacy table has %d cases but there are %d dimensions", len(cases), len(issueOpsBenchmarkDimensions))
	}
	for _, dim := range issueOpsBenchmarkDimensions {
		if _, ok := cases[dim]; !ok {
			t.Fatalf("dimension %q has no adequacy mutator (completeness guard)", dim)
		}
	}
	for dim := range cases {
		if !containsString(issueOpsBenchmarkDimensions, dim) {
			t.Fatalf("adequacy case %q is not a known dimension (completeness guard)", dim)
		}
	}

	for dim, tc := range cases {
		mutated := ScoreIssueOpsBenchmarkArtifact(fixture, tc.mutate(adequacyArtifactForTest()))

		// (1) 대상 차원의 행 자체가 실제 deterministic 0으로 하락해야 한다.
		// min/avg가 아닌 행을 읽으므로 coupled sibling도 함께 하락했을 때
		// dead always-ok check를 검출할 수 있다.
		row := adequacyRow(mutated, dim)
		if row.NotApplicable || row.Score != 0 {
			t.Fatalf("mutating %q must drop its own row to a live 0, got %+v", dim, row)
		}

		// (2) 영향 범위 제한: 정확히 {dim} ∪ coupled만 하락해야 한다. 관계없는
		// 신호까지 비우는 과도한 mutation은 더 많은 차원을 떨어뜨려 여기서 실패한다.
		want := []string{dim}
		want = append(want, tc.coupled...)
		sort.Strings(want)
		got := adequacyDroppedDims(mutated)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("mutating %q dropped %v, want exactly %v (over/under-mutation)", dim, got, want)
		}

		// (3) deterministic mutation이 critical channel로 새면 안 된다.
		if len(mutated.CriticalFailures) != 0 {
			t.Fatalf("dimension mutation %q must not trip criticals, got %v", dim, mutated.CriticalFailures)
		}
	}
}

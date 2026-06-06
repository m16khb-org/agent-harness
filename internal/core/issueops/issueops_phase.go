package issueops

import (
	"fmt"
	"strings"
	"time"
)

type IssueOpsPhase string

const (
	IssueOpsPhaseProblem     IssueOpsPhase = "problem"
	IssueOpsPhaseGrill       IssueOpsPhase = "grill"
	IssueOpsPhasePlan        IssueOpsPhase = "plan"
	IssueOpsPhaseImplement   IssueOpsPhase = "implement"
	IssueOpsPhaseAISlopClean IssueOpsPhase = "ai-slop-clean"
	IssueOpsPhaseFeedback    IssueOpsPhase = "feedback"
	IssueOpsPhasePR          IssueOpsPhase = "pr"
	IssueOpsPhaseDone        IssueOpsPhase = "done"
)

var IssueOpsPhases = []IssueOpsPhase{
	IssueOpsPhaseProblem,
	IssueOpsPhaseGrill,
	IssueOpsPhasePlan,
	IssueOpsPhaseImplement,
	IssueOpsPhaseAISlopClean,
	IssueOpsPhaseFeedback,
	IssueOpsPhasePR,
	IssueOpsPhaseDone,
}

func knownIssueOpsPhase(phase IssueOpsPhase) bool {
	for _, known := range IssueOpsPhases {
		if known == phase {
			return true
		}
	}
	return false
}

func issueOpsPhaseRank(phase IssueOpsPhase) int {
	for i, known := range IssueOpsPhases {
		if known == phase {
			return i + 1
		}
	}
	return 0
}

func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	switch phase {
	case IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhaseFeedback, IssueOpsPhasePR:
		return true
	default:
		return false
	}
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (IssueOpsRecord, error) {
	phase := IssueOpsPhase(strings.TrimSpace(to))
	if !knownIssueOpsPhase(phase) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == phase {
		if phase == IssueOpsPhaseAISlopClean {
			return refreshIssueOpsAISlopClean(stateRoot, record)
		}
		return record, nil
	}
	if record.Phase == IssueOpsPhaseDone {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot leave done phase")
	}
	if shouldRefreshIssueOpsAISlopClean(record, phase) {
		return refreshIssueOpsAISlopClean(stateRoot, record)
	}
	if issueOpsPhaseRank(phase) < issueOpsPhaseRank(record.Phase) {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot move issueops phase backward from %s to %s", record.Phase, phase)
	}
	if phase == IssueOpsPhasePlan {
		if ready := IssueOpsPlanReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter plan phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseImplement {
		if ready := IssueOpsImplementationReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter implement phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseAISlopClean {
		if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseFeedback && strings.TrimSpace(record.AISlopCleanAt) == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
	}
	if phase == IssueOpsPhasePR {
		if ready := IssueOpsStrictPRReadiness(record); !ready.Ready {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseDone && record.Phase != IssueOpsPhasePR {
		return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter done phase before pr phase")
	}
	if phase == IssueOpsPhaseDone {
		if missing := issueOpsRemoteArtifactMissing(record); len(missing) > 0 {
			return IssueOpsRecord{OK: false}, fmt.Errorf("cannot enter done phase before remote artifact verification: missing %s", strings.Join(missing, ", "))
		}
	}
	record.Phase = phase
	if phase == IssueOpsPhaseAISlopClean && strings.TrimSpace(record.AISlopCleanAt) == "" {
		record.AISlopCleanAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if phase == IssueOpsPhaseAISlopClean {
		record.AISlopCleanHead = issueOpsCurrentHead(record)
		record.AISlopCleanFingerprint = issueOpsChangeFingerprint(record)
	}
	return touchAndWriteIssueOps(stateRoot, record)
}

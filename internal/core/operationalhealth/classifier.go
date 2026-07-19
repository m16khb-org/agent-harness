package operationalhealth

import "strings"

func EvaluateCycleAuthority(cycle Cycle, opts Options) CycleAuthority {
	if strings.EqualFold(strings.TrimSpace(cycle.Phase), "done") {
		return AuthorityDead
	}
	if strings.EqualFold(strings.TrimSpace(cycle.HandoffState), "claimed") {
		if cycle.LastHeartbeatAt.IsZero() || opts.Now.IsZero() {
			return AuthorityDead
		}
		age := opts.Now.Sub(cycle.LastHeartbeatAt)
		if age < 0 || age > HeartbeatTTL {
			return AuthorityDead
		}
		return AuthorityLive
	}
	for _, id := range opts.PreserveCycleIDs {
		if strings.TrimSpace(id) == cycle.ID {
			return AuthorityPreserved
		}
	}
	return AuthorityDead
}

func Classify(snapshot Snapshot, opts Options) Result {
	result := Result{Healthy: true, Findings: []Finding{}}
	for _, cycle := range snapshot.Cycles {
		if strings.EqualFold(strings.TrimSpace(cycle.Phase), "done") {
			continue
		}
		if EvaluateCycleAuthority(cycle, opts) != AuthorityDead {
			continue
		}
		result.Findings = append(result.Findings, Finding{
			Code:         FindingDeadOwner,
			ResourceKind: "cycle",
			ResourceID:   cycle.ID,
			Summary:      "cycle owner has no fresh fenced heartbeat or invocation preservation",
		})
	}
	result.Healthy = len(result.Findings) == 0
	return result
}

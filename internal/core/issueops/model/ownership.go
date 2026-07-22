package model

// CurrentOwnershipAttempt resolves live mutation authority exclusively through
// the durable active-attempt pointer. Slice position never grants authority.
func CurrentOwnershipAttempt(record IssueOpsRecord) *IssueOpsOwnershipAttempt {
	if record.Ownership == nil || record.Ownership.ActiveAttempt <= 0 {
		return nil
	}
	for index := range record.Ownership.Attempts {
		if record.Ownership.Attempts[index].Number == record.Ownership.ActiveAttempt {
			return &record.Ownership.Attempts[index]
		}
	}
	return nil
}

// LastOwnershipAttempt returns audit history only. It must not be used as live
// authority when CurrentOwnershipAttempt returns nil or a different attempt.
func LastOwnershipAttempt(record IssueOpsRecord) *IssueOpsOwnershipAttempt {
	if record.Ownership == nil || len(record.Ownership.Attempts) == 0 {
		return nil
	}
	return &record.Ownership.Attempts[len(record.Ownership.Attempts)-1]
}

func CurrentExecutionWorkspace(record IssueOpsRecord) *IssueOpsExecutionWorkspace {
	attempt := CurrentOwnershipAttempt(record)
	if attempt == nil {
		return nil
	}
	return attempt.Workspace
}

func CurrentExecutionHandoff(record IssueOpsRecord) *IssueOpsExecutionHandoff {
	attempt := CurrentOwnershipAttempt(record)
	if attempt == nil {
		return nil
	}
	return attempt.Handoff
}

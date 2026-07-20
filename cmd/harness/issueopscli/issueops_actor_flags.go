package issueopscli

import (
	"flag"

	"agent-harness/internal/core"
)

type issueOpsActorFlags struct {
	host      *string
	sessionID *string
	agentID   *string
	cwd       *string
}

func addIssueOpsActorFlags(fs *flag.FlagSet) issueOpsActorFlags {
	return issueOpsActorFlags{
		host: fs.String("host", "", "native actor host"), sessionID: fs.String("session-id", "", "native actor session id"),
		agentID: fs.String("agent-id", "", "native actor agent id"), cwd: fs.String("cwd", "", "canonical actor cwd"),
	}
}

func (flags issueOpsActorFlags) actor() core.IssueOpsActor {
	return core.IssueOpsActor{Host: *flags.host, SessionID: *flags.sessionID, AgentID: *flags.agentID, CWD: *flags.cwd}
}

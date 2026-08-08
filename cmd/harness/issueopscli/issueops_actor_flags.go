package issueopscli

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"flag"
	"os"
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

func (flags issueOpsActorFlags) actor() issueopscontract.IssueOpsActor {
	ancestry, _ := issueOpsCLIDeps.ObserveNativeProcessAncestry(os.Getpid())
	return issueopscontract.IssueOpsActor{
		Host: *flags.host, SessionID: *flags.sessionID, AgentID: *flags.agentID, CWD: *flags.cwd,
		NativeProcessAncestry: ancestry,
	}
}

package issueopsbasesync

import "context"

type Request struct {
	Worktree   string
	BaseBranch string
}

type Receipt struct {
	BaseOID      string
	WorkOID      string
	SyncRequired bool
}

type Inspector interface {
	Observe(context.Context, Request) (Receipt, error)
}

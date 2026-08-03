package nativeactivation

import "context"

type Evidence struct {
	Host           string
	Surface        string
	Path           string
	SemanticSHA256 string
	SHA256         string
	Mode           uint32
	Size           int64
	Device         uint64
	Inode          uint64
}

type Readback struct {
	CatalogSHA256 string
	Evidence      []Evidence
}

type BeginRequest struct {
	StateRoot    string
	HarnessRoot  string
	TargetBinary string
}

type SealRequest struct {
	StateRoot     string
	HarnessRoot   string
	TargetBinary  string
	TransitionID  string
	CatalogSHA256 string
	Evidence      []Evidence
}

type AbortRequest struct {
	StateRoot    string
	HarnessRoot  string
	TargetBinary string
	TransitionID string
}

type Result struct {
	StateRoot    string
	HarnessRoot  string
	TargetBinary string
	BinarySHA256 string
	TransitionID string
	Pending      bool
	Sealed       bool
	Aborted      bool
	UpdatedAt    string
}

type Backend interface {
	Begin(context.Context, BeginRequest) (Result, error)
	Seal(context.Context, SealRequest) (Result, error)
	Abort(context.Context, AbortRequest) (Result, error)
}

type ReadbackVerifier interface {
	Verify(context.Context, string, string) (Readback, error)
}

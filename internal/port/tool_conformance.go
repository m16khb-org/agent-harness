package port

import "context"

// HostProbeRequest is the host-neutral input for one isolated conformance episode.
type HostProbeRequest struct {
	HarnessBinary         string
	FixtureID             string
	ProbeTool             string
	SourceTool            string
	SchemaSHA256          string
	ExpectedArgumentsJSON string
	Prompt                string
	Model                 string
	Profile               string
	Attempt               int
	RunToken              string
}

// HostProbePreflight records whether a host can run without starting a model episode.
type HostProbePreflight struct {
	Ready          bool
	Host           string
	Version        string
	RequestedModel string
	ObservedModel  string
	Cause          string
	Code           string
	EvidenceSource string
}

// HostProbeResult contains only bounded, redacted evidence from one episode.
type HostProbeResult struct {
	Completed              bool
	Host                   string
	HostVersion            string
	RequestedModel         string
	ObservedModel          string
	FixtureID              string
	SchemaSHA256           string
	Profile                string
	Attempt                int
	DurationMS             int64
	SessionStartObserved   bool
	PreToolUseObserved     bool
	AmbientToolCount       int
	CallCount              int
	ResponseSHA256         string
	ExitCode               int
	RawArgumentsSHA256     string
	CanonicalArgumentsJSON string
	EvidenceID             string
	Classification         string
	AdvertisedValid        bool
	CanonicalValid         bool
	DiagnosticsJSON        string
	Cause                  string
	Code                   string
	EvidenceSource         string
}

// HostProbeRunner isolates one host while preserving a shared core benchmark contract.
type HostProbeRunner interface {
	Name() string
	Preflight(context.Context, HostProbeRequest) HostProbePreflight
	Run(context.Context, HostProbeRequest) HostProbeResult
}

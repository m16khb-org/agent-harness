package toolconformance

import toolconformancecontract "agent-harness/internal/contract/toolconformance"

// 계약 타입은 contract 계층이 소유한다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	Fixture           = toolconformancecontract.Fixture
	RegressionFixture = toolconformancecontract.RegressionFixture
	ReplayResult      = toolconformancecontract.ReplayResult
	ToolDescriptor    = toolconformancecontract.ToolDescriptor
	Diagnostic        = toolconformancecontract.Diagnostic
	CallObservation   = toolconformancecontract.CallObservation
	Classification    = toolconformancecontract.Classification
	CaseResult        = toolconformancecontract.CaseResult
	BaselineCase      = toolconformancecontract.BaselineCase
	GateDecision      = toolconformancecontract.GateDecision
	EpisodeStatus     = toolconformancecontract.EpisodeStatus
	EpisodeReport     = toolconformancecontract.EpisodeReport
	HostReport        = toolconformancecontract.HostReport
	BenchmarkCounts   = toolconformancecontract.BenchmarkCounts
	GateReport        = toolconformancecontract.GateReport
	BenchmarkEvidence = toolconformancecontract.BenchmarkEvidence
	BenchmarkReport   = toolconformancecontract.BenchmarkReport
)

const (
	FixtureManifestVersion        = toolconformancecontract.FixtureManifestVersion
	ReportSchemaVersion           = toolconformancecontract.ReportSchemaVersion
	ExactValid                    = toolconformancecontract.ExactValid
	UnknownKey                    = toolconformancecontract.UnknownKey
	CoercibleTypeDrift            = toolconformancecontract.CoercibleTypeDrift
	NoncoercibleTypeDrift         = toolconformancecontract.NoncoercibleTypeDrift
	ValidButSemanticallyDifferent = toolconformancecontract.ValidButSemanticallyDifferent
	InvalidJSON                   = toolconformancecontract.InvalidJSON
	NoCall                        = toolconformancecontract.NoCall
	MultipleCalls                 = toolconformancecontract.MultipleCalls
	MissingRequired               = toolconformancecontract.MissingRequired
	EnumMismatch                  = toolconformancecontract.EnumMismatch
	EpisodeCompleted              = toolconformancecontract.EpisodeCompleted
	EpisodeIncomplete             = toolconformancecontract.EpisodeIncomplete
	GateBaselinePassed            = toolconformancecontract.GateBaselinePassed
	GateInconclusive              = toolconformancecontract.GateInconclusive
	GateDeferHardening            = toolconformancecontract.GateDeferHardening
	GateNeedsReproduction         = toolconformancecontract.GateNeedsReproduction
	GateAuthorizeHardening        = toolconformancecontract.GateAuthorizeHardening
	GateUnreproducedObservation   = toolconformancecontract.GateUnreproducedObservation
)

var (
	ParseEpisodeStatus  = toolconformancecontract.ParseEpisodeStatus
	ValidGateDecision   = toolconformancecontract.ValidGateDecision
	ParseClassification = toolconformancecontract.ParseClassification
	ParseGateDecision   = toolconformancecontract.ParseGateDecision
)

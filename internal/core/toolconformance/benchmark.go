package toolconformance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/failurecause"
	"agent-harness/internal/port"
)

const contextPressureBytes = 32 << 10

type LiveBenchmarkRequest struct {
	Hosts              []string
	Models             map[string]string
	GJCAuthEnv         []string
	Profile            string
	Only               string
	TargetCompleted    int
	MaxAttemptsPerCase int
	HarnessBinary      string
	RunID              string
	Previous           *BenchmarkReport
}

type LiveBenchmarkDependencies struct {
	Runners map[string]port.HostProbeRunner
	Now     func() time.Time
	Token   func() string
}

func RunLiveBenchmark(ctx context.Context, request LiveBenchmarkRequest, descriptors []ToolDescriptor, deps LiveBenchmarkDependencies) (BenchmarkReport, error) {
	if err := validateLiveRequest(request); err != nil {
		return BenchmarkReport{}, err
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Token == nil {
		deps.Token = randomToken
	}
	fixtures, _, err := LoadManifest(descriptors)
	if err != nil {
		return BenchmarkReport{}, err
	}
	selected, err := selectFixturePairs(request.Hosts, fixtures, request.Only)
	if err != nil {
		return BenchmarkReport{}, err
	}
	report := BenchmarkReport{
		OK:            true,
		SchemaVersion: ReportSchemaVersion,
		RunID:         request.RunID,
		Profile:       request.Profile,
		CaseCount:     len(selected),
		Hosts:         []HostReport{},
		Warnings:      []string{},
	}
	_, report.ProfileSHA256 = BuildEpisodePrompt(selected[0].Fixture, request.Profile)
	if report.RunID == "" {
		report.RunID = deps.Now().UTC().Format("20060102T150405.000000000Z")
	}
	previous := previousEpisodes(request.Previous, request.Profile)
	models := request.Models
	seenEvidenceIDs := map[string]bool{}
	for _, host := range request.Hosts {
		runner := deps.Runners[host]
		if runner == nil {
			return BenchmarkReport{}, fmt.Errorf("unsupported_host:%s", host)
		}
		hostReport := HostReport{Host: host, RequestedModel: modelForHost(models, host), Cases: []EpisodeReport{}}
		for _, episode := range previous[host] {
			fixture, selectedPair := selectedFixtureForPair(selected, host, episode.FixtureID)
			if !selectedPair || episode.SchemaSHA256 != fixture.SchemaSHA256 || episode.RequestedModel != hostReport.RequestedModel {
				continue
			}
			if !validResumedEpisode(episode) {
				return BenchmarkReport{}, fmt.Errorf("invalid_previous_episode_evidence")
			}
			if episode.Status == EpisodeCompleted {
				if seenEvidenceIDs[episode.EvidenceID] {
					return BenchmarkReport{}, fmt.Errorf("duplicate_previous_episode_evidence")
				}
				seenEvidenceIDs[episode.EvidenceID] = true
			}
			hostReport.Cases = append(hostReport.Cases, episode)
		}
		preflightRequest := port.HostProbeRequest{HarnessBinary: request.HarnessBinary, Model: hostReport.RequestedModel, GJCAuthEnv: append([]string(nil), request.GJCAuthEnv...)}
		preflight := runner.Preflight(ctx, preflightRequest)
		hostReport.Version = preflight.Version
		if preflight.ObservedModel != "" {
			hostReport.ObservedModel = preflight.ObservedModel
		}
		for _, pair := range selected {
			if pair.Host != host {
				continue
			}
			fixture := pair.Fixture
			completed := completedForFixture(hostReport.Cases, fixture.ID)
			if !preflight.Ready {
				episode := incompleteEpisode(host, preflight.Version, fixture, request.Profile, hostReport.RequestedModel, 0, preflight.Cause, preflight.Code, preflight.EvidenceSource)
				hostReport.Cases = append(hostReport.Cases, episode)
				continue
			}
			newAttemptLimit := (request.TargetCompleted - completed) * request.MaxAttemptsPerCase
			for newAttempts := 0; completed < request.TargetCompleted && newAttempts < newAttemptLimit; newAttempts++ {
				attempt := attemptsForFixture(hostReport.Cases, fixture.ID) + 1
				prompt, _ := BuildEpisodePrompt(fixture, request.Profile)
				runResult := runner.Run(ctx, port.HostProbeRequest{
					HarnessBinary:         request.HarnessBinary,
					FixtureID:             fixture.ID,
					ProbeTool:             fixture.ProbeTool,
					SourceTool:            fixture.SourceTool,
					SchemaSHA256:          fixture.SchemaSHA256,
					ExpectedArgumentsJSON: mustJSON(fixture.ExpectedArguments),
					Prompt:                prompt,
					Model:                 hostReport.RequestedModel,
					Profile:               request.Profile,
					Attempt:               attempt,
					RunToken:              deps.Token(),
					GJCAuthEnv:            append([]string(nil), request.GJCAuthEnv...),
				})
				episode := classifyHostResult(runResult, fixture)
				if episode.HostVersion == "" {
					episode.HostVersion = preflight.Version
				}
				if episode.ObservedModel == "" {
					episode.ObservedModel = hostReport.ObservedModel
				}
				if episode.Status == EpisodeCompleted {
					if seenEvidenceIDs[episode.EvidenceID] {
						return BenchmarkReport{}, fmt.Errorf("duplicate_episode_evidence")
					}
					seenEvidenceIDs[episode.EvidenceID] = true
				}
				if episode.ObservedModel != "" {
					hostReport.ObservedModel = episode.ObservedModel
				}
				hostReport.Cases = append(hostReport.Cases, episode)
				if episode.Status == "completed" {
					completed++
				}
			}
		}
		sort.SliceStable(hostReport.Cases, func(i, j int) bool {
			if hostReport.Cases[i].FixtureID != hostReport.Cases[j].FixtureID {
				return hostReport.Cases[i].FixtureID < hostReport.Cases[j].FixtureID
			}
			return hostReport.Cases[i].Attempt < hostReport.Cases[j].Attempt
		})
		hostReport.AttemptCount = len(hostReport.Cases)
		hostReport.CompletedEpisodes = countCompleted(hostReport.Cases)
		report.Hosts = append(report.Hosts, hostReport)
	}
	report.Counts = countReport(report)
	report.Gate = decideGate(report, selected, request.TargetCompleted)
	if report.Gate.Decision == GateInconclusive {
		report.OK = false
	}
	if report.Counts.Attempts > 0 && float64(report.Counts.EnvironmentFailures+report.Counts.TransportFailures)/float64(report.Counts.Attempts) > 0.05 {
		report.Warnings = append(report.Warnings, "environment_transport_failure_rate_above_5_percent")
	}
	return report, nil
}

type fixturePair struct {
	Host    string
	Fixture Fixture
}

func selectFixturePairs(hosts []string, fixtures []Fixture, only string) ([]fixturePair, error) {
	pairs := []fixturePair{}
	onlyHost, onlyFixture := "", ""
	if only != "" {
		parts := strings.Split(only, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid_only:%s", only)
		}
		onlyHost, onlyFixture = parts[0], parts[1]
	}
	for _, host := range hosts {
		for _, fixture := range fixtures {
			if only != "" && (host != onlyHost || fixture.ID != onlyFixture) {
				continue
			}
			pairs = append(pairs, fixturePair{Host: host, Fixture: fixture})
		}
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no_selected_cases")
	}
	return pairs, nil
}

func validateLiveRequest(request LiveBenchmarkRequest) error {
	if len(request.Hosts) == 0 {
		return fmt.Errorf("hosts_required")
	}
	if request.Profile != "clean" && request.Profile != "context-pressure" {
		return fmt.Errorf("invalid_profile:%s", request.Profile)
	}
	if request.TargetCompleted != 1 && request.TargetCompleted != 10 && request.TargetCompleted != 20 {
		return fmt.Errorf("invalid_target_completed:%d", request.TargetCompleted)
	}
	if request.MaxAttemptsPerCase < 1 || request.MaxAttemptsPerCase > 3 {
		return fmt.Errorf("invalid_max_attempts_per_case:%d", request.MaxAttemptsPerCase)
	}
	if request.TargetCompleted > 1 && request.Only == "" {
		return fmt.Errorf("only_required_for_reproduction")
	}
	if request.HarnessBinary == "" {
		return fmt.Errorf("harness_binary_required")
	}
	return nil
}

func BuildEpisodePrompt(fixture Fixture, profile string) (string, string) {
	instruction := fmt.Sprintf("Call the only allowed MCP tool %s exactly once with these exact JSON arguments. Do not add, remove, rename, or coerce any field. After the tool result, output only captured.\nArguments:\n%s", fixture.ProbeTool, mustJSON(fixture.ExpectedArguments))
	if profile != "context-pressure" {
		return instruction, ""
	}
	unit := "CONFORMANCE_CONTEXT_PRESSURE_FIXED_BLOCK\n"
	appendix := strings.Repeat(unit, contextPressureBytes/len(unit)+1)[:contextPressureBytes]
	sum := sha256.Sum256([]byte(appendix))
	return appendix + "\n" + instruction, hex.EncodeToString(sum[:])
}

func classifyHostResult(result port.HostProbeResult, fixture Fixture) EpisodeReport {
	if !result.Completed {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, result.Cause, result.Code, result.EvidenceSource)
	}
	classification, err := ParseClassification(result.Classification)
	if err != nil || result.EvidenceID == "" || !validEvidenceID(result.EvidenceID) {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if result.CallCount == 0 || classification == Classification(NoCall) {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "unknown", "no_call", result.Host+"_runner")
	}
	if (result.CallCount > 1) != (classification == Classification(MultipleCalls)) {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	var arguments any
	if err := json.Unmarshal([]byte(result.CanonicalArgumentsJSON), &arguments); err != nil {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	diagnostics := []Diagnostic{}
	if err := json.Unmarshal([]byte(result.DiagnosticsJSON), &diagnostics); err != nil {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	sortDiagnostics(diagnostics)
	if (classification == Classification(ExactValid) || classification == Classification(ValidButSemanticallyDifferent)) && (!result.CanonicalValid || len(diagnostics) != 0) {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if schemaDriftClassification(classification) && result.CanonicalValid {
		return incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	evidence := []failurecause.Evidence{}
	failed := classification != Classification(ExactValid)
	if failed {
		cause := failurecause.Model
		if result.AdvertisedValid && !result.CanonicalValid {
			cause = failurecause.ContractInput
		}
		evidence = append(evidence, failurecause.Evidence{Cause: cause, Code: string(classification), Source: "tool_conformance"})
	}
	causeResult := failurecause.Classify(failed, evidence)
	return EpisodeReport{
		Status:               EpisodeCompleted,
		Host:                 result.Host,
		HostVersion:          result.HostVersion,
		RequestedModel:       result.RequestedModel,
		ObservedModel:        result.ObservedModel,
		FixtureID:            fixture.ID,
		SchemaSHA256:         result.SchemaSHA256,
		Profile:              result.Profile,
		Attempt:              result.Attempt,
		DurationMS:           result.DurationMS,
		AmbientToolCount:     result.AmbientToolCount,
		CallCount:            result.CallCount,
		RawArgumentsSHA256:   result.RawArgumentsSHA256,
		EvidenceID:           result.EvidenceID,
		CanonicalArguments:   arguments,
		Classification:       classification,
		AdvertisedValid:      result.AdvertisedValid,
		CanonicalValid:       result.CanonicalValid,
		Diagnostics:          diagnostics,
		DiagnosticSignature:  DiagnosticSignature(classification, diagnostics),
		FailureCause:         causeResult.Cause,
		FailureCauseReason:   causeResult.Reason,
		FailureCauseEvidence: causeResult.Evidence,
	}
}

func validEvidenceID(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validResumedEpisode(episode EpisodeReport) bool {
	if episode.Status == EpisodeIncomplete {
		return true
	}
	if episode.Status != EpisodeCompleted || !validEvidenceID(episode.EvidenceID) || len(episode.RawArgumentsSHA256) != sha256.Size*2 {
		return false
	}
	if _, err := hex.DecodeString(episode.RawArgumentsSHA256); err != nil {
		return false
	}
	if episode.DiagnosticSignature != DiagnosticSignature(episode.Classification, episode.Diagnostics) {
		return false
	}
	if (episode.CallCount > 1) != (episode.Classification == Classification(MultipleCalls)) || episode.CallCount < 1 {
		return false
	}
	if (episode.Classification == Classification(ExactValid) || episode.Classification == Classification(ValidButSemanticallyDifferent)) && (!episode.CanonicalValid || len(episode.Diagnostics) != 0) {
		return false
	}
	return !schemaDriftClassification(episode.Classification) || !episode.CanonicalValid
}
func incompleteEpisode(host, version string, fixture Fixture, profile, model string, attempt int, cause, code, source string) EpisodeReport {
	parsedCause := failurecause.Cause(cause)
	if parsedCause != failurecause.HarnessEnvironment && parsedCause != failurecause.Transport && parsedCause != failurecause.ContractInput && parsedCause != failurecause.Model {
		parsedCause = failurecause.Unknown
	}
	if source == "" {
		source = "tool_conformance"
	}
	result := failurecause.Classify(true, []failurecause.Evidence{{Cause: parsedCause, Code: code, Source: source}})
	return EpisodeReport{
		Status:               EpisodeIncomplete,
		Host:                 host,
		HostVersion:          version,
		RequestedModel:       model,
		ObservedModel:        "",
		FixtureID:            fixture.ID,
		SchemaSHA256:         fixture.SchemaSHA256,
		Profile:              profile,
		Attempt:              attempt,
		Diagnostics:          []Diagnostic{},
		FailureCause:         result.Cause,
		FailureCauseReason:   result.Reason,
		FailureCauseEvidence: result.Evidence,
	}
}

func DiagnosticSignature(classification Classification, diagnostics []Diagnostic) string {
	value := struct {
		Classification Classification `json:"classification"`
		Diagnostics    []Diagnostic   `json:"diagnostics"`
	}{classification, append([]Diagnostic(nil), diagnostics...)}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func decideGate(report BenchmarkReport, selected []fixturePair, target int) GateReport {
	for _, pair := range selected {
		if completedForPair(report, pair.Host, pair.Fixture.ID) < target {
			return GateReport{Decision: GateInconclusive}
		}
	}
	type signatureCount struct {
		Host      string
		FixtureID string
		Signature string
		Count     int
	}
	counts := map[string]*signatureCount{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			if episode.Status != "completed" || !schemaDriftClassification(episode.Classification) {
				continue
			}
			key := host.Host + "\x00" + episode.FixtureID + "\x00" + episode.DiagnosticSignature
			if counts[key] == nil {
				counts[key] = &signatureCount{Host: host.Host, FixtureID: episode.FixtureID, Signature: episode.DiagnosticSignature}
			}
			counts[key].Count++
		}
	}
	ordered := make([]*signatureCount, 0, len(counts))
	for _, count := range counts {
		ordered = append(ordered, count)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Host != ordered[j].Host {
			return ordered[i].Host < ordered[j].Host
		}
		if ordered[i].FixtureID != ordered[j].FixtureID {
			return ordered[i].FixtureID < ordered[j].FixtureID
		}
		return ordered[i].Signature < ordered[j].Signature
	})
	for _, count := range ordered {
		if count.Count >= 2 {
			return GateReport{Decision: GateAuthorizeHardening, ConfirmedSignature: count.Signature, ConfirmedCount: count.Count}
		}
	}
	if len(ordered) == 0 {
		return GateReport{Decision: GateDeferHardening}
	}
	if target >= 20 {
		return GateReport{Decision: GateUnreproducedObservation}
	}
	return GateReport{Decision: GateNeedsReproduction, NextReproductionTarget: ordered[0].Host + ":" + ordered[0].FixtureID}
}

func schemaDriftClassification(classification Classification) bool {
	switch string(classification) {
	case UnknownKey, CoercibleTypeDrift, NoncoercibleTypeDrift, InvalidJSON, MissingRequired, EnumMismatch:
		return true
	default:
		return false
	}
}

func countReport(report BenchmarkReport) BenchmarkCounts {
	counts := BenchmarkCounts{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			counts.Attempts++
			if episode.Status != "completed" {
				noCall := false
				for _, evidence := range episode.FailureCauseEvidence {
					if evidence.Code == "no_call" {
						noCall = true
						break
					}
				}
				if noCall {
					counts.NoCalls++
				} else {
					switch episode.FailureCause {
					case failurecause.HarnessEnvironment:
						counts.EnvironmentFailures++
					case failurecause.Transport:
						counts.TransportFailures++
					}
				}
				continue
			}
			counts.Completed++
			counts.ModelDenominator++
			if episode.Classification == Classification(ValidButSemanticallyDifferent) {
				counts.ValidSemanticDifferences++
			}
			if schemaDriftClassification(episode.Classification) {
				counts.SchemaDriftObservations++
			}
		}
	}
	return counts
}

func previousEpisodes(previous *BenchmarkReport, profile string) map[string][]EpisodeReport {
	out := map[string][]EpisodeReport{}
	if previous == nil || previous.Profile != profile {
		return out
	}
	for _, host := range previous.Hosts {
		out[host.Host] = append([]EpisodeReport(nil), host.Cases...)
	}
	return out
}

func selectedFixtureForPair(pairs []fixturePair, host, fixture string) (Fixture, bool) {
	for _, pair := range pairs {
		if pair.Host == host && pair.Fixture.ID == fixture {
			return pair.Fixture, true
		}
	}
	return Fixture{}, false
}

func pairSelected(pairs []fixturePair, host, fixture string) bool {
	_, selected := selectedFixtureForPair(pairs, host, fixture)
	return selected
}

func completedForPair(report BenchmarkReport, host, fixture string) int {
	for _, hostReport := range report.Hosts {
		if hostReport.Host == host {
			return completedForFixture(hostReport.Cases, fixture)
		}
	}
	return 0
}

func completedForFixture(episodes []EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture && episode.Status == "completed" {
			count++
		}
	}
	return count
}

func attemptsForFixture(episodes []EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture {
			count++
		}
	}
	return count
}

func countCompleted(episodes []EpisodeReport) int {
	count := 0
	for _, episode := range episodes {
		if episode.Status == "completed" {
			count++
		}
	}
	return count
}

func modelForHost(models map[string]string, host string) string {
	if model := strings.TrimSpace(models[host]); model != "" {
		return model
	}
	if host == "gjc" {
		return ""
	}
	return "default"
}

func randomToken() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		sum := sha256.Sum256([]byte(time.Now().UTC().String()))
		return hex.EncodeToString(sum[:16])
	}
	return hex.EncodeToString(value)
}

func mustJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

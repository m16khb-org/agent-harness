// Package issueopsbodysync decides how a proposed body replaces the live body
// of a remote IssueOps artifact.
//
// Every decision here is pure: the caller supplies the live body it read and
// the body the agent authored, and this package answers what the merged body
// is, how far the artifact has drifted, and whether a write may proceed.
package issueopsbodysync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	contract "agent-harness/internal/contract/issueopsbodysync"
)

// ErrAlreadyInSync reports that the merged body equals the live body, so a
// write would change nothing. Callers treat it as a successful no-op, not a
// failure.
var ErrAlreadyInSync = errors.New("remote body is already in sync")

// Managed regions are written by the harness, never by the body author. They
// are lifted out of the live body and spliced back after the proposal so a full
// body replacement cannot destroy them.
//
// The completion block matters most: cleanup readiness readback-checks its
// start marker before destructive local cleanup, so losing it would strand a
// finished cycle.
const (
	devilsAdvocateStart = "<!-- issueops:devils-advocate:start -->"
	devilsAdvocateEnd   = "<!-- issueops:devils-advocate:end -->"
	completionStart     = "<!-- issueops:completion:start -->"
	completionEnd       = "<!-- issueops:completion:end -->"

	// RegionDevilsAdvocate, RegionCompletion and RegionIssueCreate name the
	// managed regions in preview output and durable evidence.
	RegionDevilsAdvocate = "issueops:devils-advocate"
	RegionCompletion     = "issueops:completion"
	RegionIssueCreate    = "agent-harness:issue-create"
)

// issueCreateMarker matches the durable marker that reconcile-issue uses to
// re-find an issue whose creation outcome was unclear. It is a single comment
// line rather than a delimited block.
var issueCreateMarker = regexp.MustCompile(`<!-- agent-harness:issue-create:[0-9a-f]{32} -->`)

// NormalizeBody folds the transformations a provider applies to a body it
// stores, so that a plain round trip through the provider does not read as a
// human edit. GitHub returns issue and pull-request bodies with CRLF line
// endings even when the body was submitted with LF, and both providers drop
// trailing whitespace; comparing raw bytes would report every such artifact as
// edited outside the harness on its first sync.
func NormalizeBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.TrimRight(body, " \t\n")
}

// SHA256Body returns the lowercase hex digest of a normalized body: the identity
// used for every drift comparison and for the compare-and-swap that guards a
// write.
func SHA256Body(body string) string {
	sum := sha256.Sum256([]byte(NormalizeBody(body)))
	return hex.EncodeToString(sum[:])
}

// ManagedRegions lifts every managed region out of a live body, in the order
// the regions appear.
func ManagedRegions(live string) []contract.Region {
	type located struct {
		at     int
		region contract.Region
	}
	var found []located
	for _, delimited := range []struct {
		name, start, end string
	}{
		{RegionDevilsAdvocate, devilsAdvocateStart, devilsAdvocateEnd},
		{RegionCompletion, completionStart, completionEnd},
	} {
		s := strings.Index(live, delimited.start)
		if s < 0 {
			continue
		}
		e := strings.Index(live, delimited.end)
		if e <= s {
			continue
		}
		found = append(found, located{
			at:     s,
			region: contract.Region{Name: delimited.name, Block: live[s : e+len(delimited.end)]},
		})
	}
	if at := issueCreateMarker.FindStringIndex(live); at != nil {
		found = append(found, located{
			at:     at[0],
			region: contract.Region{Name: RegionIssueCreate, Block: live[at[0]:at[1]]},
		})
	}
	for i := 1; i < len(found); i++ {
		for j := i; j > 0 && found[j].at < found[j-1].at; j-- {
			found[j], found[j-1] = found[j-1], found[j]
		}
	}
	regions := make([]contract.Region, 0, len(found))
	for _, entry := range found {
		regions = append(regions, entry.region)
	}
	return regions
}

// Merge splices the managed regions of the live body onto the proposed body.
//
// The proposal owns the authored content and nothing else: a proposal that
// already carries a managed marker is refused, because hand-authored managed
// blocks would duplicate or contradict the ones the harness maintains.
func Merge(live, proposed string) (string, []contract.Region, error) {
	if err := ValidateProposal(proposed); err != nil {
		return "", nil, err
	}
	regions := ManagedRegions(live)
	merged := strings.TrimRight(proposed, "\n")
	for _, region := range regions {
		merged += "\n\n" + region.Block
	}
	return merged + "\n", regions, nil
}

// ValidateProposal checks a body the caller authored, without reading the
// remote. Callers run it before any provider call so an unusable proposal costs
// nothing on the network.
func ValidateProposal(proposed string) error {
	if strings.TrimSpace(proposed) == "" {
		return fmt.Errorf("proposed body is empty")
	}
	if containsManagedMarker(proposed) {
		return fmt.Errorf("proposed body contains a managed section marker; the harness owns those blocks, so leave them out of the authored body")
	}
	return nil
}

func containsManagedMarker(body string) bool {
	for _, marker := range []string{devilsAdvocateStart, devilsAdvocateEnd, completionStart, completionEnd} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return issueCreateMarker.MatchString(body)
}

// ClassifyDrift compares the body the harness last recorded, the body living on
// the provider, and the body a write would produce.
//
// A recorded digest that no longer matches the provider means somebody edited
// the artifact outside the harness, and that verdict outranks staleness: the
// operator has to see the outside edit before anything overwrites it.
func ClassifyDrift(recordedSHA, liveSHA, mergedSHA string) contract.Drift {
	recordedSHA = strings.TrimSpace(recordedSHA)
	if recordedSHA != "" && recordedSHA != liveSHA {
		return contract.DriftRemoteEdited
	}
	if liveSHA == mergedSHA {
		return contract.DriftInSync
	}
	return contract.DriftStale
}

// BuildPlan is the single comparison both preview and confirm run, so the body
// an operator reviews is byte-identical to the body a confirm would write.
func BuildPlan(recordedSHA, live, proposed string) (contract.Plan, error) {
	merged, regions, err := Merge(live, proposed)
	if err != nil {
		return contract.Plan{}, err
	}
	plan := contract.Plan{
		RemoteBodySHA256: SHA256Body(live),
		MergedBody:       merged,
		MergedBodySHA256: SHA256Body(merged),
	}
	plan.Drift = ClassifyDrift(recordedSHA, plan.RemoteBodySHA256, plan.MergedBodySHA256)
	for _, region := range regions {
		plan.PreservedSections = append(plan.PreservedSections, region.Name)
	}
	return plan, nil
}

// ValidateWrite decides whether a confirmed sync may write.
//
// The rule is one compare-and-swap: the caller has to name the exact live body
// it built its proposal from. Naming a digest that no longer matches means the
// artifact moved underneath the caller, so the write is refused instead of
// silently clobbering whatever arrived in between.
func ValidateWrite(plan contract.Plan, expectedSHA string, acceptRemoteEdits bool) error {
	expectedSHA = strings.ToLower(strings.TrimSpace(expectedSHA))
	if expectedSHA == "" {
		return fmt.Errorf("--expected-body-sha256 is required for a confirmed sync; run the preview and pass the expected_body_sha256 it reports")
	}
	if expectedSHA != plan.RemoteBodySHA256 {
		return fmt.Errorf("remote body changed since it was read (expected %s, live %s); re-run the preview and rebuild the body on the current content",
			shortSHA(expectedSHA), shortSHA(plan.RemoteBodySHA256))
	}
	if plan.Drift == contract.DriftRemoteEdited && !acceptRemoteEdits {
		return fmt.Errorf("remote body was edited outside the harness; fold those edits into the proposed body, then pass --accept-remote-edits to confirm you preserved them")
	}
	if plan.Drift == contract.DriftInSync {
		return ErrAlreadyInSync
	}
	return nil
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

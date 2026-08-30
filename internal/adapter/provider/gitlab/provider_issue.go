package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"agent-harness/internal/adapter/provider/issuebody"
	"agent-harness/internal/adapter/provider/providerutil"
	"agent-harness/internal/port"
)

func (Provider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	hostname, projectPath, iid, err := parseGitLabIssueURL(req.IssueURL)
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	endpoint := "projects/" + url.PathEscape(projectPath) + "/issues/" + iid
	if !req.Confirm {
		if _, _, _, err := issuebody.RenderSection(req, ts, gitLabIssueBodyLimit); err != nil {
			return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
		}
		return port.IssueProviderUpdateIssueBodySectionResult{
			OK:      true,
			Preview: fmt.Sprintf("[dry-run] would execute: glab api %s --hostname %s; then --method PUT -f description=<merged %s section>", endpoint, hostname, req.Section),
		}, nil
	}
	current, err := runGlabAPI(req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	var payload struct {
		Description string `json:"description"`
		WebURL      string `json:"web_url"`
	}
	if err := json.Unmarshal(current, &payload); err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, fmt.Errorf("parse glab issue description: %w", err)
	}
	start, end, err := issuebody.SectionMarkers(req.Section)
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	// 병합 결과가 한도를 지키도록 기존 본문을 반영한 예산으로 렌더한다(C3-F1).
	section, start, end, err := issuebody.RenderSection(req, ts, issuebody.SectionBudget(payload.Description, gitLabIssueBodyLimit, start, end))
	if err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	merged := issuebody.MergeManagedSection(payload.Description, section, start, end)
	if _, err := runGlabAPI(req.Repo, hostname, endpoint, "--method", "PUT", "-f", "description="+merged); err != nil {
		return port.IssueProviderUpdateIssueBodySectionResult{OK: false}, err
	}
	return port.IssueProviderUpdateIssueBodySectionResult{OK: true, URL: providerutil.FirstNonEmpty(payload.WebURL, req.IssueURL), Updated: true}, nil
}

// CloseIssue closes the parent/primary issue and verifies the final state by
// readback. Merge-evidence gating is owned by the core caller.
func (Provider) CloseIssue(req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	hostname, projectPath, iid, err := parseGitLabIssueURL(req.IssueURL)
	if err != nil {
		return port.IssueProviderCloseIssueResult{OK: false, Provider: "gitlab"}, err
	}
	endpoint := "projects/" + url.PathEscape(projectPath) + "/issues/" + iid
	if !req.Confirm {
		return port.IssueProviderCloseIssueResult{
			OK: true, Provider: "gitlab", IssueURL: req.IssueURL,
			Preview: fmt.Sprintf("[dry-run] would execute: glab api %s --hostname %s --method PUT -f state_event=close; then readback state", endpoint, hostname),
		}, nil
	}
	state, err := readGlabIssueState(req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderCloseIssueResult{OK: false, Provider: "gitlab"}, err
	}
	if strings.EqualFold(state, "closed") {
		return port.IssueProviderCloseIssueResult{OK: true, Provider: "gitlab", IssueURL: req.IssueURL, Closed: true, AlreadyClosed: true, State: state}, nil
	}
	if _, err := runGlabAPI(req.Repo, hostname, endpoint, "--method", "PUT", "-f", "state_event=close"); err != nil {
		return port.IssueProviderCloseIssueResult{OK: false, Provider: "gitlab"}, err
	}
	state, err = readGlabIssueState(req.Repo, hostname, endpoint)
	if err != nil {
		return port.IssueProviderCloseIssueResult{OK: false, Provider: "gitlab"}, err
	}
	if !strings.EqualFold(state, "closed") {
		return port.IssueProviderCloseIssueResult{OK: false, Provider: "gitlab", IssueURL: req.IssueURL, State: state}, fmt.Errorf("issue close was not verified: state=%s", state)
	}
	return port.IssueProviderCloseIssueResult{OK: true, Provider: "gitlab", IssueURL: req.IssueURL, Closed: true, State: state}, nil
}

func readGlabIssueState(repo, hostname, endpoint string) (string, error) {
	out, err := runGlabAPI(repo, hostname, endpoint)
	if err != nil {
		return "", err
	}
	var payload struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("parse glab issue state: %w", err)
	}
	return payload.State, nil
}

// runGlabAPI runs a REST `glab api <endpoint> --hostname <host> [extra...]` call,
// mirroring the hostname/order shape the verify layer uses for issue reads.
func runGlabAPI(repo, hostname, endpoint string, extra ...string) ([]byte, error) {
	return runGlabAPIContext(context.Background(), repo, hostname, endpoint, extra...)
}

func runGlabAPIContext(ctx context.Context, repo, hostname, endpoint string, extra ...string) ([]byte, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return nil, fmt.Errorf("glab CLI is not installed; install it from https://gitlab.com/gitlab-org/cli")
	}
	cmdArgs := []string{"api", endpoint}
	if strings.TrimSpace(hostname) != "" {
		cmdArgs = append(cmdArgs, "--hostname", strings.TrimSpace(hostname))
	}
	cmdArgs = append(cmdArgs, extra...)
	out, _, err := providerutil.RunBoundedMutationContext(ctx, repo, "glab", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("glab api failed: %w", err)
	}
	return out, nil
}

package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IssueOpsPhase string

const (
	IssueOpsPhaseProblem   IssueOpsPhase = "problem"
	IssueOpsPhasePlan      IssueOpsPhase = "plan"
	IssueOpsPhaseImplement IssueOpsPhase = "implement"
	IssueOpsPhaseFeedback  IssueOpsPhase = "feedback"
)

type IssueOpsStartRequest struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
}

type IssueOpsFeedbackItem struct {
	Source    string `json:"source"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type IssueOpsRecord struct {
	OK        bool                   `json:"ok"`
	ID        string                 `json:"id"`
	Repo      string                 `json:"repo"`
	Branch    string                 `json:"branch,omitempty"`
	Phase     IssueOpsPhase          `json:"phase"`
	IssueURL  string                 `json:"issue_url,omitempty"`
	PlanPath  string                 `json:"plan_path,omitempty"`
	Feedback  []IssueOpsFeedbackItem `json:"feedback,omitempty"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

type IssueOpsReadiness struct {
	OK       bool     `json:"ok"`
	Ready    bool     `json:"ready"`
	Missing  []string `json:"missing"`
	IssueURL string   `json:"issue_url,omitempty"`
	PlanPath string   `json:"plan_path,omitempty"`
	Branch   string   `json:"branch,omitempty"`
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("repo is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := IssueOpsRecord{
		OK:        true,
		ID:        newIssueOpsID(repo, req.Branch, now),
		Repo:      repo,
		Branch:    strings.TrimSpace(req.Branch),
		Phase:     IssueOpsPhaseProblem,
		Feedback:  []IssueOpsFeedbackItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return writeIssueOps(stateRoot, record)
}

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	path := issueopsPath(stateRoot, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	var record IssueOpsRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: file has %q", record.ID)
	}
	record.OK = true
	return record, nil
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (IssueOpsRecord, error) {
	u := strings.TrimSpace(issueURL)
	if err := validateIssueURL(u); err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.IssueURL = u
	record.Phase = IssueOpsPhasePlan
	return touchAndWriteIssueOps(stateRoot, record)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	path := strings.TrimSpace(planPath)
	if path == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path is required")
	}
	if strings.Contains(path, "\x00") || strings.Contains(path, "..") {
		return IssueOpsRecord{OK: false}, fmt.Errorf("plan_path must not contain path traversal")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.PlanPath = path
	record.Phase = IssueOpsPhaseImplement
	return touchAndWriteIssueOps(stateRoot, record)
}

func AddIssueOpsFeedback(stateRoot, id, source, body string) (IssueOpsRecord, error) {
	source = strings.TrimSpace(source)
	body = strings.TrimSpace(body)
	if source == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback source is required")
	}
	if body == "" {
		return IssueOpsRecord{OK: false}, fmt.Errorf("feedback body is required")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.Feedback = append(record.Feedback, IssueOpsFeedbackItem{Source: source, Body: body, CreatedAt: now})
	record.Phase = IssueOpsPhaseFeedback
	record.UpdatedAt = now
	return writeIssueOps(stateRoot, record)
}

func IssueOpsPRReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.PlanPath) == "" {
		missing = append(missing, "plan_path")
	}
	return IssueOpsReadiness{
		OK:       true,
		Ready:    len(missing) == 0,
		Missing:  missing,
		IssueURL: record.IssueURL,
		PlanPath: record.PlanPath,
		Branch:   record.Branch,
	}
}

func IssueOpsStateRoot() string {
	return filepath.Join(StateDir(), "issueops")
}

func newIssueOpsID(repo, branch, now string) string {
	sum := sha256.Sum256([]byte(repo + "\x00" + branch + "\x00" + now))
	return "io-" + hex.EncodeToString(sum[:])[:12]
}

func touchAndWriteIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return writeIssueOps(stateRoot, record)
}

func writeIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, err
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		record.OK = false
		return record, err
	}
	record.OK = true
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		record.OK = false
		return record, err
	}
	path := issueopsPath(stateRoot, record.ID)
	tmp, err := os.CreateTemp(stateRoot, "."+record.ID+"-*.tmp")
	if err != nil {
		record.OK = false
		return record, err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(b); err != nil {
			return err
		}
		if _, err := tmp.Write([]byte{'\n'}); err != nil {
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		record.OK = false
		return record, writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		record.OK = false
		return record, err
	}
	return record, nil
}

func issueopsPath(stateRoot, id string) string {
	return filepath.Join(stateRoot, id+".json")
}

func normalizeIssueOpsID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	if !strings.HasPrefix(id, "io-") {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
}

func validateIssueURL(issueURL string) error {
	if issueURL == "" {
		return fmt.Errorf("issue_url is required")
	}
	parsed, err := url.Parse(issueURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("issue_url must be an http(s) URL")
	}
	return nil
}

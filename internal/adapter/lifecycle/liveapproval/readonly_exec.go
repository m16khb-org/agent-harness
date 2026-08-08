package liveapproval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ReadOnlyExecGrantTTL = 30 * time.Minute

const (
	readOnlyExecSchemaVersion = 2
	readOnlyExecKind          = "readonly_exec"
)

type ReadOnlyExecRequest struct {
	Host      string
	SessionID string
	RepoRoot  string
	CWD       string
	Tool      string
	Command   string
	Context   string
	Namespace string
}

type readonlyExecRecord struct {
	SchemaVersion      int    `json:"schema_version"`
	Kind               string `json:"kind"`
	Status             string `json:"status"`
	Token              string `json:"token,omitempty"`
	RequestFingerprint string `json:"request_fingerprint,omitempty"`
	ScopeFingerprint   string `json:"scope_fingerprint"`
	ExpiresAt          string `json:"expires_at"`
}

func EvaluateReadOnlyExec(store Store, req ReadOnlyExecRequest) Result {
	if !strings.EqualFold(strings.TrimSpace(req.Host), "codex") {
		return Result{}
	}
	if missingReadOnlyExecRequestField(req) {
		return blocked("kubectl read-only exec approval unavailable: host session, workspace, command, context, and namespace identity are required.")
	}
	namespace, err := resolveNamespace(store, req.RepoRoot, true)
	if err != nil || !namespace.Exists || !namespace.Valid || strings.TrimSpace(namespace.RepoRoot) == "" {
		return blocked("kubectl read-only exec approval unavailable: project-scoped lifecycle state could not be initialized.")
	}
	if store.WithLock == nil || store.WriteJSON == nil {
		return blocked("kubectl read-only exec approval unavailable: approval state could not be recorded.")
	}
	now := storeNow(store)
	requestFingerprint := readonlyExecRequestFingerprint(req, namespace.RepoRoot)
	scopeFingerprint := readonlyExecScopeFingerprint(req, namespace.RepoRoot)
	key := approvalKey(req.SessionID)
	path := filepath.Join(namespace.Dir, readOnlyExecApprovalKey(req.SessionID)+".json")
	legacyPath := filepath.Join(namespace.Dir, approvalKey(req.SessionID)+".json")
	legacyFingerprint := requestFingerprintForReadOnlyExec(req, namespace.RepoRoot)
	result := blocked("kubectl read-only exec approval unavailable: approval state could not be recorded.")
	err = store.WithLock(context.Background(), namespace.Dir, key, func(context.Context) error {
		legacy, legacyValid := readRecord(legacyPath, now)
		if legacyValid && legacy.RequestFingerprint == legacyFingerprint {
			if err := removeIfPresent(legacyPath); err != nil {
				return err
			}
		}

		current, valid := readReadonlyExecRecord(path, now)
		if valid && current.ScopeFingerprint == scopeFingerprint {
			switch current.Status {
			case statusPending:
				if current.RequestFingerprint == requestFingerprint {
					result = readOnlyExecPending(current.Token)
					return nil
				}
			case statusGranted:
				current.ExpiresAt = now.Add(ReadOnlyExecGrantTTL).UTC().Format(time.RFC3339Nano)
				if err := store.WriteJSON(path, current, 0o600); err != nil {
					return err
				}
				result = Result{Handled: true, Allowed: true}
				return nil
			}
		}
		if err := removeIfPresent(path); err != nil {
			return err
		}
		token, err := newToken(store)
		if err != nil {
			return err
		}
		next := readonlyExecRecord{
			SchemaVersion:      readOnlyExecSchemaVersion,
			Kind:               readOnlyExecKind,
			Status:             statusPending,
			Token:              token,
			RequestFingerprint: requestFingerprint,
			ScopeFingerprint:   scopeFingerprint,
			ExpiresAt:          now.Add(ApprovalTTL).UTC().Format(time.RFC3339Nano),
		}
		if err := store.WriteJSON(path, next, 0o600); err != nil {
			return err
		}
		result = readOnlyExecPending(token)
		return nil
	})
	if err != nil {
		return blocked("kubectl read-only exec approval unavailable: approval state could not be recorded.")
	}
	return result
}

func missingReadOnlyExecRequestField(req ReadOnlyExecRequest) bool {
	return strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.RepoRoot) == "" ||
		strings.TrimSpace(req.CWD) == "" ||
		strings.TrimSpace(req.Tool) == "" ||
		strings.TrimSpace(req.Command) == "" ||
		strings.TrimSpace(req.Context) == "" ||
		strings.TrimSpace(req.Namespace) == ""
}

func readonlyExecRequestFingerprint(req ReadOnlyExecRequest, canonicalRepo string) string {
	return fingerprintFields(
		"kubectl-readonly-exec-request:v2",
		strings.ToLower(strings.TrimSpace(req.Host)),
		strings.TrimSpace(req.SessionID),
		strings.TrimSpace(canonicalRepo),
		strings.TrimSpace(req.CWD),
		strings.TrimSpace(req.Tool),
		strings.TrimSpace(req.Command),
		strings.TrimSpace(req.Context),
		strings.TrimSpace(req.Namespace),
	)
}

func readonlyExecScopeFingerprint(req ReadOnlyExecRequest, canonicalRepo string) string {
	return fingerprintFields(
		"kubectl-readonly-exec-scope:v1",
		strings.ToLower(strings.TrimSpace(req.Host)),
		strings.TrimSpace(req.SessionID),
		strings.TrimSpace(canonicalRepo),
		strings.TrimSpace(req.Context),
		strings.TrimSpace(req.Namespace),
	)
}

func requestFingerprintForReadOnlyExec(req ReadOnlyExecRequest, canonicalRepo string) string {
	return requestFingerprint(Request{
		Host:      req.Host,
		SessionID: req.SessionID,
		RepoRoot:  req.RepoRoot,
		CWD:       req.CWD,
		Tool:      req.Tool,
		Command:   req.Command,
	}, canonicalRepo)
}

func readOnlyExecApprovalKey(sessionID string) string {
	return "kubectl-readonly-exec-approval-" + sessionSafeName(sessionID)
}

func readReadonlyExecRecord(path string, now time.Time) (readonlyExecRecord, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return readonlyExecRecord{}, false
	}
	var current readonlyExecRecord
	if err := json.Unmarshal(data, &current); err != nil ||
		current.SchemaVersion != readOnlyExecSchemaVersion ||
		current.Kind != readOnlyExecKind ||
		current.ScopeFingerprint == "" ||
		(current.Status != statusPending && current.Status != statusGranted) {
		return readonlyExecRecord{}, false
	}
	switch current.Status {
	case statusPending:
		if current.Token == "" || current.RequestFingerprint == "" {
			return readonlyExecRecord{}, false
		}
	case statusGranted:
		if current.Token != "" || current.RequestFingerprint != "" {
			return readonlyExecRecord{}, false
		}
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, current.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return readonlyExecRecord{}, false
	}
	return current, true
}

func readOnlyExecPending(token string) Result {
	return Result{
		Handled: true,
		Token:   token,
		Reason:  "kubectl read-only exec requires explicit scope approval. 승인 " + token + " 을 입력하면 같은 session/context/namespace의 허용된 진단을 재사용할 수 있습니다.",
	}
}

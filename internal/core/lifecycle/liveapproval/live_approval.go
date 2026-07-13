package liveapproval

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const ApprovalTTL = 10 * time.Minute

const (
	schemaVersion = 1
	statusPending = "pending"
	statusGranted = "granted"
	tokenAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

var approvalPromptPattern = regexp.MustCompile("^승인 (AH-[A-HJ-NP-Z2-9]{6})$")

type Namespace struct {
	Exists   bool
	Valid    bool
	RepoRoot string
	Dir      string
}

type Store struct {
	Resolve   func(repoRoot string) (Namespace, error)
	Init      func(repoRoot string) (Namespace, error)
	WithLock  func(dir, key string, fn func() error) error
	WriteJSON func(path string, value any, perm os.FileMode) error
	Now       func() time.Time
	NewToken  func() (string, error)
}

type Request struct {
	Host      string
	SessionID string
	RepoRoot  string
	CWD       string
	Tool      string
	Command   string
}

type ApprovalRequest struct {
	Host      string
	SessionID string
	RepoRoot  string
	Prompt    string
}

type Result struct {
	Handled           bool
	Allowed           bool
	Token             string
	Reason            string
	AdditionalContext string
}

type record struct {
	SchemaVersion      int    `json:"schema_version"`
	Status             string `json:"status"`
	Token              string `json:"token"`
	RequestFingerprint string `json:"request_fingerprint"`
	ExpiresAt          string `json:"expires_at"`
}

func Evaluate(store Store, req Request) Result {
	if !strings.EqualFold(strings.TrimSpace(req.Host), "codex") {
		return Result{}
	}
	if missingRequestField(req) {
		return blocked("kubectl live-access approval unavailable: host session, workspace, cwd, tool, and command identity are required.")
	}
	namespace, err := resolveNamespace(store, req.RepoRoot, true)
	if err != nil || !namespace.Exists || !namespace.Valid || strings.TrimSpace(namespace.RepoRoot) == "" {
		return blocked("kubectl live-access approval unavailable: project-scoped lifecycle state could not be initialized.")
	}
	now := storeNow(store)
	fingerprint := requestFingerprint(req, namespace.RepoRoot)
	key := approvalKey(req.SessionID)
	path := filepath.Join(namespace.Dir, key+".json")
	result := blocked("kubectl live-access approval unavailable: approval state could not be recorded.")
	if store.WithLock == nil || store.WriteJSON == nil {
		return result
	}
	err = store.WithLock(namespace.Dir, key, func() error {
		current, valid := readRecord(path, now)
		if valid && current.RequestFingerprint == fingerprint {
			switch current.Status {
			case statusPending:
				result = pending(current.Token)
				return nil
			case statusGranted:
				if err := os.Remove(path); err != nil {
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
		next := record{
			SchemaVersion:      schemaVersion,
			Status:             statusPending,
			Token:              token,
			RequestFingerprint: fingerprint,
			ExpiresAt:          now.Add(ApprovalTTL).UTC().Format(time.RFC3339Nano),
		}
		if err := store.WriteJSON(path, next, 0o600); err != nil {
			return err
		}
		result = pending(token)
		return nil
	})
	if err != nil {
		return blocked("kubectl live-access approval unavailable: approval state could not be recorded.")
	}
	return result
}

func Approve(store Store, req ApprovalRequest) Result {
	if !strings.EqualFold(strings.TrimSpace(req.Host), "codex") {
		return Result{}
	}
	match := approvalPromptPattern.FindStringSubmatch(strings.TrimSpace(req.Prompt))
	if len(match) != 2 {
		return Result{}
	}
	if strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.RepoRoot) == "" {
		return approvalRejected()
	}
	namespace, err := resolveNamespace(store, req.RepoRoot, false)
	if err != nil || !namespace.Exists || !namespace.Valid {
		return approvalRejected()
	}
	now := storeNow(store)
	key := approvalKey(req.SessionID)
	path := filepath.Join(namespace.Dir, key+".json")
	result := approvalRejected()
	if store.WithLock == nil || store.WriteJSON == nil {
		return result
	}
	err = store.WithLock(namespace.Dir, key, func() error {
		current, valid := readRecord(path, now)
		if !valid || current.Status != statusPending || current.Token != match[1] {
			return nil
		}
		current.Status = statusGranted
		current.ExpiresAt = now.Add(ApprovalTTL).UTC().Format(time.RFC3339Nano)
		if err := store.WriteJSON(path, current, 0o600); err != nil {
			return err
		}
		result = Result{
			Handled:           true,
			AdditionalContext: "[agent-harness]\n- approval: kubectl live-access 승인이 기록되었습니다. 다음 동일 명령 한 번에만 유효합니다.",
		}
		return nil
	})
	if err != nil {
		return approvalRejected()
	}
	return result
}

func missingRequestField(req Request) bool {
	return strings.TrimSpace(req.SessionID) == "" ||
		strings.TrimSpace(req.RepoRoot) == "" ||
		strings.TrimSpace(req.CWD) == "" ||
		strings.TrimSpace(req.Tool) == "" ||
		strings.TrimSpace(req.Command) == ""
}

func resolveNamespace(store Store, repoRoot string, initialize bool) (Namespace, error) {
	if store.Resolve == nil {
		return Namespace{}, errors.New("approval namespace resolver is unavailable")
	}
	namespace, err := store.Resolve(repoRoot)
	if err != nil || namespace.Exists || !initialize {
		return namespace, err
	}
	if store.Init == nil {
		return Namespace{}, errors.New("approval namespace initializer is unavailable")
	}
	return store.Init(repoRoot)
}

func requestFingerprint(req Request, canonicalRepo string) string {
	fields := []string{
		"kubectl-live-approval:v1",
		strings.ToLower(strings.TrimSpace(req.Host)),
		strings.TrimSpace(req.SessionID),
		strings.TrimSpace(canonicalRepo),
		strings.TrimSpace(req.CWD),
		strings.TrimSpace(req.Tool),
		strings.TrimSpace(req.Command),
	}
	hash := sha256.New()
	for _, field := range fields {
		_ = binary.Write(hash, binary.BigEndian, uint64(len(field)))
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func approvalKey(sessionID string) string {
	return "kubectl-live-approval-" + sessionSafeName(sessionID)
}

func sessionSafeName(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func readRecord(path string, now time.Time) (record, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return record{}, false
	}
	var current record
	if err := json.Unmarshal(data, &current); err != nil ||
		current.SchemaVersion != schemaVersion ||
		(current.Status != statusPending && current.Status != statusGranted) ||
		current.Token == "" ||
		current.RequestFingerprint == "" {
		return record{}, false
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, current.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return record{}, false
	}
	return current, true
}

func removeIfPresent(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func storeNow(store Store) time.Time {
	if store.Now != nil {
		return store.Now().UTC()
	}
	return time.Now().UTC()
}

func newToken(store Store) (string, error) {
	if store.NewToken != nil {
		return store.NewToken()
	}
	var token strings.Builder
	token.WriteString("AH-")
	limit := big.NewInt(int64(len(tokenAlphabet)))
	for range 6 {
		index, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		token.WriteByte(tokenAlphabet[index.Int64()])
	}
	return token.String(), nil
}

func pending(token string) Result {
	return Result{
		Handled: true,
		Token:   token,
		Reason:  "kubectl live cluster access requires explicit user confirmation. 승인 " + token + " 을 입력한 뒤 동일 명령을 다시 실행하세요.",
	}
}

func blocked(reason string) Result {
	return Result{Handled: true, Reason: reason}
}

func approvalRejected() Result {
	return Result{
		Handled:           true,
		AdditionalContext: "[agent-harness]\n- approval: 일치하는 유효한 kubectl live-access 요청이 없어 승인이 기록되지 않았습니다.",
	}
}

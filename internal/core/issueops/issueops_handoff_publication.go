package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/policy"
)

var publicationFullCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var publicationRemotePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)
var publicationSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var publicationGlabVersionPattern = regexp.MustCompile(`(?:^|\s)v?(\d+)\.(\d+)\.(\d+)(?:\s|$)`)

const publicationDiagnosticLimit = 4096

type IssueOpsHandoffPublishRequest struct {
	ID                           string `json:"id"`
	Confirm                      bool   `json:"confirm,omitempty"`
	ApproveLegacyCoordinatorSeal bool   `json:"approve_legacy_coordinator_seal,omitempty"`
	Host                         string `json:"host"`
	SessionID                    string `json:"session_id"`
	AgentID                      string `json:"agent_id,omitempty"`
	SourceCWD                    string `json:"source_cwd"`
}

type IssueOpsHandoffPublicationReader interface {
	LocalRefHead(context.Context, string, string) (string, error)
	RemoteRefHead(context.Context, string, string, string, string) (string, error)
	PushTarget(context.Context, string, string) (IssueOpsPublicationPushTarget, error)
	PushExact(context.Context, string, string, string, string, string) error
}

type IssueOpsPublicationPushTarget struct {
	URL         string
	Fingerprint string
}

type IssueOpsGitLabCapabilityReader interface {
	GitLabVersion(context.Context, string) (string, error)
}

func (GitIssueOpsHandoffPublicationReader) PushTarget(ctx context.Context, repo, remote string) (IssueOpsPublicationPushTarget, error) {
	var target IssueOpsPublicationPushTarget
	err := withPublicationGitConfigLocks(ctx, repo, func() error {
		var err error
		target, err = publicationPushTargetLocked(ctx, repo, remote)
		return err
	})
	return target, err
}

func publicationPushTargetLocked(ctx context.Context, repo, remote string) (IssueOpsPublicationPushTarget, error) {
	if !publicationRemotePattern.MatchString(remote) {
		return IssueOpsPublicationPushTarget{}, fmt.Errorf("publication remote is unsafe")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	code, stdout, stderr := publicationGitCmd(bounded, repo, "remote", "get-url", "--push", "--all", remote)
	if code != 0 {
		return IssueOpsPublicationPushTarget{}, fmt.Errorf("resolve effective publication push target: %s", publicationDiagnostic(stderr))
	}
	pushURLs, err := publicationURLLines(stdout)
	if err != nil {
		return IssueOpsPublicationPushTarget{}, err
	}
	if len(pushURLs) != 1 {
		return IssueOpsPublicationPushTarget{}, fmt.Errorf("publication remote must resolve exactly one effective push target; found %d", len(pushURLs))
	}
	rules, err := publicationGitURLRules(ctx, repo)
	if err != nil {
		return IssueOpsPublicationPushTarget{}, err
	}
	pushURLs[0], err = resolvePublicationURLRewrites(pushURLs[0], rules)
	if err != nil {
		return IssueOpsPublicationPushTarget{}, err
	}
	sum := sha256.Sum256([]byte(pushURLs[0]))
	return IssueOpsPublicationPushTarget{URL: pushURLs[0], Fingerprint: hex.EncodeToString(sum[:])}, nil
}

func (g GitIssueOpsHandoffPublicationReader) PushExact(ctx context.Context, repo, remote, expectedFingerprint, branch, finalHead string) error {
	if !publicationRemotePattern.MatchString(remote) || !safePublicationBranch(branch) || !publicationFullCommitPattern.MatchString(finalHead) {
		return fmt.Errorf("publication remote, branch, or final head is unsafe")
	}
	return withPublicationGitConfigLocks(ctx, repo, func() error {
		target, err := publicationPushTargetLocked(ctx, repo, remote)
		if err != nil {
			return err
		}
		if target.Fingerprint != expectedFingerprint {
			return fmt.Errorf("publication push target changed before push")
		}
		bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		ref := "refs/heads/" + branch
		code, _, stderr := publicationGitCmd(bounded, repo, "push", "--", target.URL, finalHead+":"+ref)
		if code != 0 {
			return fmt.Errorf("push exact publication ref at %s: %s", finalHead, publicationDiagnosticWithoutTarget(stderr, target.URL))
		}
		return nil
	})
}

func publicationURLLines(stdout string) ([]string, error) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.ContainsAny(line, "\x00\r\n\t ") {
			return nil, fmt.Errorf("effective publication push target contains an empty or unsafe URL")
		}
		values = append(values, line)
	}
	return values, nil
}

type GitIssueOpsHandoffPublicationReader struct{}

func (GitIssueOpsHandoffPublicationReader) GitLabVersion(ctx context.Context, repo string) (string, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(bounded, "glab", "--version")
	cmd.Dir = repo
	stdout, stderr := &publicationBoundedBuffer{limit: publicationDiagnosticLimit}, &publicationBoundedBuffer{limit: publicationDiagnosticLimit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("verify GitLab CLI custom-port capability: %s", publicationDiagnostic(stderr.String()+" "+err.Error()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (GitIssueOpsHandoffPublicationReader) LocalRefHead(ctx context.Context, repo, ref string) (string, error) {
	if !safePublicationRef(ref) {
		return "", fmt.Errorf("local publication ref is unsafe")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	code, stdout, stderr := publicationGitCmd(bounded, repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if code != 0 {
		return "", fmt.Errorf("resolve local publication ref %s: %s", ref, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (g GitIssueOpsHandoffPublicationReader) RemoteRefHead(ctx context.Context, repo, remote, expectedFingerprint, ref string) (string, error) {
	if !publicationRemotePattern.MatchString(remote) || !safePublicationRef(ref) {
		return "", fmt.Errorf("remote publication identity is unsafe")
	}
	var head string
	err := withPublicationGitConfigLocks(ctx, repo, func() error {
		target, err := publicationPushTargetLocked(ctx, repo, remote)
		if err != nil {
			return err
		}
		if target.Fingerprint != expectedFingerprint {
			return fmt.Errorf("publication push target changed before remote readback")
		}
		bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		code, stdout, stderr := publicationGitCmd(bounded, repo, "ls-remote", "--heads", "--", target.URL, ref)
		if code != 0 {
			return fmt.Errorf("resolve remote publication ref %s/%s: %s", remote, ref, publicationDiagnosticWithoutTarget(stderr, target.URL))
		}
		fields := strings.Fields(strings.TrimSpace(stdout))
		if len(fields) != 2 || fields[1] != ref {
			return fmt.Errorf("remote publication ref %s/%s did not resolve exactly once", remote, ref)
		}
		head = fields[0]
		return nil
	})
	return head, err
}

type publicationGitURLRule struct {
	origin string
	base   string
	prefix string
}

func publicationGitURLRules(ctx context.Context, repo string) ([]publicationGitURLRule, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	code, stdout, stderr := publicationGitCmd(bounded, repo, "config", "--show-origin", "--get-regexp", `^url\..*\.(insteadOf|pushInsteadOf)$`)
	if (code == 0 || code == 1) && strings.TrimSpace(stdout) == "" {
		return nil, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("enumerate publication URL rewrite authority: %s", publicationDiagnostic(stderr))
	}
	var rules []publicationGitURLRule
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || !strings.HasPrefix(fields[0], "file:") || strings.TrimPrefix(fields[0], "file:") == "" {
			return nil, fmt.Errorf("publication URL rewrite origin is incomplete")
		}
		key := fields[1]
		lower := strings.ToLower(key)
		var suffix string
		switch {
		case strings.HasSuffix(lower, ".insteadof"):
			suffix = key[len(key)-len(".insteadOf"):]
		case strings.HasSuffix(lower, ".pushinsteadof"):
			suffix = key[len(key)-len(".pushInsteadOf"):]
		default:
			return nil, fmt.Errorf("publication URL rewrite key is unsupported")
		}
		base := strings.TrimSuffix(strings.TrimPrefix(key, "url."), suffix)
		if base == "" || fields[2] == "" || strings.ContainsAny(base+fields[2], "\x00\r\n\t ") {
			return nil, fmt.Errorf("publication URL rewrite rule is unsafe")
		}
		origin := strings.TrimPrefix(fields[0], "file:")
		if !filepath.IsAbs(origin) {
			origin = filepath.Join(repo, origin)
		}
		rules = append(rules, publicationGitURLRule{origin: filepath.Clean(origin), base: base, prefix: fields[2]})
	}
	return rules, nil
}

func resolvePublicationURLRewrites(value string, rules []publicationGitURLRule) (string, error) {
	seen := map[string]bool{value: true}
	for depth := 0; depth < 16; depth++ {
		best := -1
		bases := map[string]bool{}
		for _, rule := range rules {
			if strings.HasPrefix(value, rule.prefix) {
				if len(rule.prefix) > best {
					best, bases = len(rule.prefix), map[string]bool{rule.base: true}
				} else if len(rule.prefix) == best {
					bases[rule.base] = true
				}
			}
		}
		if best < 0 {
			return value, nil
		}
		if len(bases) != 1 {
			return "", fmt.Errorf("publication URL rewrite authority is ambiguous")
		}
		base := ""
		for candidate := range bases {
			base = candidate
		}
		value = base + value[best:]
		if seen[value] {
			return "", fmt.Errorf("publication URL rewrite cycle detected")
		}
		seen[value] = true
	}
	return "", fmt.Errorf("publication URL rewrite depth exceeded")
}

func withPublicationGitConfigLocks(ctx context.Context, repo string, fn func() error) error {
	before, err := publicationGitURLRules(ctx, repo)
	if err != nil {
		return err
	}
	beforeOrigins, err := publicationGitConfigOrigins(ctx, repo)
	if err != nil {
		return err
	}
	paths, err := publicationGitConfigPaths(ctx, repo, before, beforeOrigins)
	if err != nil {
		return err
	}
	locks := make([]string, 0, len(paths))
	for _, path := range paths {
		lock := path + ".lock"
		file, openErr := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if openErr != nil {
			for i := len(locks) - 1; i >= 0; i-- {
				_ = os.Remove(locks[i])
			}
			return fmt.Errorf("publication git config lock is unavailable")
		}
		if closeErr := file.Close(); closeErr != nil {
			_ = os.Remove(lock)
			for i := len(locks) - 1; i >= 0; i-- {
				_ = os.Remove(locks[i])
			}
			return fmt.Errorf("publication git config lock could not be sealed")
		}
		locks = append(locks, lock)
	}
	defer func() {
		for i := len(locks) - 1; i >= 0; i-- {
			_ = os.Remove(locks[i])
		}
	}()
	after, err := publicationGitURLRules(ctx, repo)
	afterOrigins, originsErr := publicationGitConfigOrigins(ctx, repo)
	if err != nil || originsErr != nil || !reflect.DeepEqual(before, after) || !reflect.DeepEqual(beforeOrigins, afterOrigins) {
		return fmt.Errorf("publication git URL rewrite authority changed while acquiring config locks")
	}
	return fn()
}

func publicationGitConfigOrigins(ctx context.Context, repo string) ([]string, error) {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	code, stdout, stderr := publicationGitCmd(bounded, repo, "config", "--show-origin", "--includes", "--list")
	if code != 0 {
		return nil, fmt.Errorf("enumerate publication git config origins: %s", publicationDiagnostic(stderr))
	}
	origins := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("publication git config origin is incomplete")
		}
		// Command-line configuration is immutable for this git subprocess. Only
		// file-backed configuration can change while the publication locks are held.
		if !strings.HasPrefix(parts[0], "file:") {
			continue
		}
		origin := strings.TrimPrefix(parts[0], "file:")
		if !filepath.IsAbs(origin) {
			origin = filepath.Join(repo, origin)
		}
		origin = filepath.Clean(origin)
		origins[origin] = true
		key, value, found := strings.Cut(parts[1], "=")
		includeKey := strings.EqualFold(key, "include.path") || strings.HasPrefix(strings.ToLower(key), "includeif.") && strings.HasSuffix(strings.ToLower(key), ".path")
		if found && includeKey {
			included := value
			if strings.HasPrefix(included, "~/") {
				home, homeErr := os.UserHomeDir()
				if homeErr != nil {
					return nil, fmt.Errorf("publication git include origin cannot be resolved")
				}
				included = filepath.Join(home, strings.TrimPrefix(included, "~/"))
			}
			if !filepath.IsAbs(included) {
				included = filepath.Join(filepath.Dir(origin), included)
			}
			origins[filepath.Clean(included)] = true
		}
	}
	ordered := make([]string, 0, len(origins))
	for origin := range origins {
		ordered = append(ordered, origin)
	}
	sort.Strings(ordered)
	return ordered, nil
}

func publicationGitConfigPaths(ctx context.Context, repo string, rules []publicationGitURLRule, origins []string) ([]string, error) {
	paths := map[string]bool{}
	for index, args := range [][]string{{"rev-parse", "--git-common-dir"}, {"rev-parse", "--git-path", "config.worktree"}} {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		code, stdout, stderr := publicationGitCmd(bounded, repo, args...)
		cancel()
		if code != 0 || strings.TrimSpace(stdout) == "" {
			return nil, fmt.Errorf("resolve publication git config authority: %s", publicationDiagnostic(stderr))
		}
		path := strings.TrimSpace(stdout)
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		if index == 0 {
			path = filepath.Join(path, "config")
		}
		paths[filepath.Clean(path)] = true
	}
	home, homeErr := os.UserHomeDir()
	if homeErr != nil || strings.TrimSpace(home) == "" {
		return nil, fmt.Errorf("publication git user config authority cannot be resolved")
	}
	paths[filepath.Join(home, ".gitconfig")] = true
	xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	xdgConfig := filepath.Join(xdg, "git", "config")
	if info, err := os.Stat(filepath.Dir(xdgConfig)); err == nil && info.IsDir() {
		paths[xdgConfig] = true
	}
	for _, name := range []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM"} {
		configured := strings.TrimSpace(os.Getenv(name))
		if configured == "" || configured == os.DevNull || name == "GIT_CONFIG_SYSTEM" && strings.TrimSpace(os.Getenv("GIT_CONFIG_NOSYSTEM")) != "" {
			continue
		}
		if !filepath.IsAbs(configured) {
			configured = filepath.Join(repo, configured)
		}
		paths[filepath.Clean(configured)] = true
	}
	for _, rule := range rules {
		path := rule.origin
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("publication URL rewrite origin is not absolute")
		}
		paths[filepath.Clean(path)] = true
	}
	for _, origin := range origins {
		if !filepath.IsAbs(origin) {
			return nil, fmt.Errorf("publication git config origin is not absolute")
		}
		paths[filepath.Clean(origin)] = true
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		parent := filepath.Dir(path)
		info, err := os.Stat(parent)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("publication git config authority parent is unavailable")
		}
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	return ordered, nil
}

func publicationGitCmd(ctx context.Context, repo string, args ...string) (int, string, string) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_TERMINAL_PROMPT=") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0")
	stdout, stderr := &publicationBoundedBuffer{limit: publicationDiagnosticLimit}, &publicationBoundedBuffer{limit: publicationDiagnosticLimit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if err == nil {
		return 0, strings.TrimSpace(stdout.String()), publicationDiagnostic(stderr.String())
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), strings.TrimSpace(stdout.String()), publicationDiagnostic(stderr.String())
	}
	return 1, strings.TrimSpace(stdout.String()), publicationDiagnostic(err.Error())
}

type publicationBoundedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *publicationBoundedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	if original > remaining {
		b.truncated = true
	}
	return original, nil
}

func (b *publicationBoundedBuffer) String() string {
	value := string(b.data)
	if b.truncated {
		value += "...[truncated]"
	}
	return value
}

func publicationDiagnostic(value string) string {
	value = policy.RedactFreeform(strings.TrimSpace(value))
	if len(value) > publicationDiagnosticLimit {
		value = value[:publicationDiagnosticLimit] + "...[truncated]"
	}
	return value
}

func publicationDiagnosticWithoutTarget(value, target string) string {
	return publicationDiagnostic(strings.ReplaceAll(value, target, "[REDACTED_REMOTE]"))
}

func safePublicationRef(ref string) bool {
	return strings.HasPrefix(ref, "refs/heads/") && safePublicationBranch(strings.TrimPrefix(ref, "refs/heads/"))
}

func safePublicationBranch(branch string) bool {
	return branch != "" && len(branch) <= 1024 && branch == strings.TrimSpace(branch) && strings.IndexFunc(branch, unicode.IsControl) < 0 && !strings.HasPrefix(branch, "-") && !strings.HasPrefix(branch, "/") && !strings.HasSuffix(branch, "/") && !strings.HasSuffix(branch, ".") && !strings.Contains(branch, "..") && !strings.Contains(branch, "@{") && !strings.ContainsAny(branch, " ~^:?*[\\")
}

type issueOpsPublicationIdentity struct {
	Provider         string
	ProjectKey       string
	Remote           string
	PushTargetSHA256 string
	Branch           string
	Base             string
	LocalRef         string
	RemoteRef        string
	FinalHead        string
}

func RecordIssueOpsHandoffPublishReceipt(ctx context.Context, stateRoot string, req IssueOpsHandoffPublishRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, clock IssueOpsHandoffPrepareClock) (IssueOpsRecord, error) {
	if !req.Confirm {
		return IssueOpsRecord{}, fmt.Errorf("publish receipt requires --confirm")
	}
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		migrated, handled, migrationErr := reattestRawV5Publication(ctx, stateRoot, req, reader, lease, clock)
		if handled {
			return migrated, migrationErr
		}
		return IssueOpsRecord{}, err
	}
	identity, err := issueOpsAcceptedPublicationIdentity(validated)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if !handoff.CoordinatorIdentityMatches(validated, model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.SourceCWD) {
		return IssueOpsRecord{}, fmt.Errorf("handoff publish requires the sealed coordinator native session from the exact source checkout")
	}
	if lease == nil {
		return IssueOpsRecord{}, fmt.Errorf("publication sole-writer dependency is unavailable")
	}
	validated, err = attestIssueOpsPublicationSoleWriter(ctx, stateRoot, validated, lease, issueOpsHandoffNow(clock))
	if err != nil {
		return IssueOpsRecord{}, err
	}
	identity, err = resolveIssueOpsPublicationPushTarget(ctx, validated, identity, reader)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if validated.ExecutionHandoff.PublishReceipt != nil {
		if err := validateIssueOpsPublishReceipt(validated, identity); err != nil {
			return IssueOpsRecord{}, err
		}
		if err := verifyIssueOpsLocalPublicationHead(ctx, validated.Repo, identity, reader); err != nil {
			return IssueOpsRecord{}, err
		}
		if err := verifyIssueOpsRemotePublicationHead(ctx, validated.Repo, identity, reader); err != nil {
			return IssueOpsRecord{}, err
		}
		return validated, nil
	}
	if err := verifyIssueOpsLocalPublicationHead(ctx, validated.Repo, identity, reader); err != nil {
		return IssueOpsRecord{}, err
	}
	if err := reader.PushExact(ctx, validated.Repo, identity.Remote, identity.PushTargetSHA256, identity.Branch, identity.FinalHead); err != nil {
		return IssueOpsRecord{}, err
	}
	if err := verifyIssueOpsRemotePublicationHead(ctx, validated.Repo, identity, reader); err != nil {
		return IssueOpsRecord{}, err
	}
	now := issueOpsHandoffNow(clock)
	var persisted IssueOpsRecord
	err = withIssueOpsLock(ctx, stateRoot, validated.ID, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, validated.ID)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("accepted handoff changed during publication verification")
		}
		current.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{
			Provider: identity.Provider, ProjectKey: identity.ProjectKey, Remote: identity.Remote, PushTargetSHA256: identity.PushTargetSHA256, Branch: identity.Branch, Base: identity.Base,
			RemoteRef: identity.RemoteRef, FinalHead: identity.FinalHead, VerifiedAt: now,
		}
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return persisted, err
}

func reattestRawV5Publication(ctx context.Context, stateRoot string, req IssueOpsHandoffPublishRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, clock IssueOpsHandoffPrepareClock) (IssueOpsRecord, bool, error) {
	id := req.ID
	var persisted IssueOpsRecord
	handled := false
	err := withIssueOpsLock(ctx, stateRoot, id, func(spanCtx context.Context) error {
		raw, err := readRawIssueOpsBytes(stateRoot, id)
		if err != nil {
			return err
		}
		var header struct {
			SchemaVersion     int             `json:"schema_version"`
			RemoteCreateClaim json.RawMessage `json:"remote_create_claim"`
			ExecutionHandoff  *struct {
				CoordinatorSession json.RawMessage `json:"coordinator_session"`
				PublishReceipt     json.RawMessage `json:"publish_receipt"`
			} `json:"execution_handoff"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return err
		}
		if header.SchemaVersion != 5 || header.ExecutionHandoff == nil || !rawIssueOpsAuthorityPresent(header.ExecutionHandoff.PublishReceipt) || rawIssueOpsAuthorityPresent(header.RemoteCreateClaim) {
			return nil
		}
		handled = true
		if rawIssueOpsAuthorityPresent(header.ExecutionHandoff.CoordinatorSession) {
			return fmt.Errorf("raw schema-v5 publication cannot contain coordinator_session durable mutation authority")
		}
		var record IssueOpsRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if record.ID != id || record.ExecutionHandoff == nil || record.ExecutionHandoff.PublishReceipt == nil {
			return fmt.Errorf("raw schema-v5 publication record is incomplete")
		}
		native := model.IssueOpsHostSessionIdentity{Host: strings.TrimSpace(req.Host), SessionID: strings.TrimSpace(req.SessionID), AgentID: strings.TrimSpace(req.AgentID)}
		if !handoff.CoordinatorIdentityMatches(record, native, req.SourceCWD) {
			if !req.ApproveLegacyCoordinatorSeal || !handoff.LegacyCoordinatorIdentityCanBeSealed(record, native, req.SourceCWD) {
				return fmt.Errorf("raw schema-v5 publication requires explicit legacy coordinator seal approval from the exact source checkout")
			}
			record.ExecutionHandoff.CoordinatorSession = &native
		}
		legacyReceipt := *record.ExecutionHandoff.PublishReceipt
		record.SchemaVersion = model.IssueOpsCurrentSchemaVersion
		record.ExecutionHandoff.PublishReceipt = nil
		if err := handoff.ValidateEnvelope(record); err != nil {
			return fmt.Errorf("raw schema-v5 publication authority cannot be re-attested: %w", err)
		}
		identity, err := issueOpsAcceptedPublicationIdentity(record)
		if err != nil {
			return err
		}
		if lease == nil || reader == nil {
			return fmt.Errorf("raw schema-v5 publication re-attestation dependencies are unavailable")
		}
		if err := attestHandoffSoleWriter(spanCtx, record, lease, ""); err != nil {
			return fmt.Errorf("raw schema-v5 publication sole-writer re-attestation failed: %w", err)
		}
		identity, err = resolveIssueOpsPublicationPushTarget(spanCtx, record, identity, reader)
		if err != nil {
			return err
		}
		if legacyReceipt.Provider != identity.Provider || legacyReceipt.Remote != identity.Remote || legacyReceipt.Branch != identity.Branch || legacyReceipt.RemoteRef != identity.RemoteRef || legacyReceipt.FinalHead != identity.FinalHead {
			return fmt.Errorf("raw schema-v5 publication receipt differs from current durable authority")
		}
		if err := verifyIssueOpsLocalPublicationHead(spanCtx, record.Repo, identity, reader); err != nil {
			return err
		}
		if err := verifyIssueOpsRemotePublicationHead(spanCtx, record.Repo, identity, reader); err != nil {
			return err
		}
		now := issueOpsHandoffNow(clock)
		record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{
			Provider: identity.Provider, ProjectKey: identity.ProjectKey, Remote: identity.Remote, PushTargetSHA256: identity.PushTargetSHA256,
			Branch: identity.Branch, Base: identity.Base, RemoteRef: identity.RemoteRef, FinalHead: identity.FinalHead, VerifiedAt: now,
		}
		record.ExecutionHandoff.UpdatedAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, handled, err
}

func ValidateIssueOpsHandoffPublication(ctx context.Context, stateRoot string, record IssueOpsRecord, provider, head, base string, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient) error {
	if err := handoff.ValidateEnvelope(record); err != nil {
		return err
	}
	identity, err := issueOpsAcceptedPublicationIdentity(record)
	if err != nil {
		return err
	}
	if lease == nil {
		return fmt.Errorf("publication sole-writer dependency is unavailable")
	}
	if strings.ToLower(strings.TrimSpace(provider)) != identity.Provider || strings.TrimSpace(head) != identity.Branch || strings.TrimSpace(base) != identity.Base {
		return fmt.Errorf("publication provider, head branch, or base branch differs from durable IssueOps authority")
	}
	current, err := attestIssueOpsPublicationSoleWriter(ctx, stateRoot, record, lease, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	identity, err = resolveIssueOpsPublicationPushTarget(ctx, current, identity, reader)
	if err != nil {
		return err
	}
	if err := validateIssueOpsPublishReceipt(current, identity); err != nil {
		return err
	}
	return verifyIssueOpsPublicationHeads(ctx, current.Repo, identity, reader)
}

func attestIssueOpsPublicationSoleWriter(ctx context.Context, stateRoot string, expected IssueOpsRecord, lease IssueOpsOrcaDispatchClient, now string) (IssueOpsRecord, error) {
	err := attestHandoffSoleWriter(ctx, expected, lease, "")
	if err == nil && expected.ExecutionHandoff.PublicationRecovery == nil {
		return expected, nil
	}
	var recoveryErr handoffSoleWriterRecoveryError
	var conflictErr handoffSoleWriterConflictError
	recoveryCode := ""
	if errors.As(err, &recoveryErr) {
		recoveryCode = "publication_inventory_ambiguous"
	} else if errors.As(err, &conflictErr) {
		recoveryCode = "publication_writer_conflict"
	} else if err != nil {
		return expected, fmt.Errorf("publication sole-writer re-attestation failed: %w", err)
	}
	var persisted IssueOpsRecord
	persistErr := withIssueOpsLock(ctx, stateRoot, expected.ID, func(context.Context) error {
		current, readErr := ReadIssueOps(stateRoot, expected.ID)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("accepted handoff changed during publication attestation")
		}
		if err == nil {
			current.ExecutionHandoff.PublicationRecovery = nil
		} else {
			current.ExecutionHandoff.PublicationRecovery = &model.IssueOpsExecutionHandoffFailure{Code: recoveryCode, Message: soleWriterRecoveryDiagnostic(err.Error()), At: now}
		}
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	if persistErr != nil {
		return expected, persistErr
	}
	if err != nil {
		return persisted, fmt.Errorf("publication sole-writer re-attestation failed: %w", err)
	}
	return persisted, nil
}

func issueOpsAcceptedPublicationIdentity(record IssueOpsRecord) (issueOpsPublicationIdentity, error) {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateClosed || h.ClosedDisposition != handoff.DispositionAccepted || h.Result == nil || h.Orca == nil || record.BranchPrepare == nil {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication requires a closed accepted execution handoff")
	}
	provider := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
	projectKey := remote.ProjectKey(record.IssueURL, provider, "issue")
	if provider != "github" && provider != "gitlab" || projectKey == "" || record.BranchPrepare.IssueURL != record.IssueURL {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication provider must be github or gitlab")
	}
	branch := strings.TrimSpace(record.Branch)
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	finalHead := strings.TrimSpace(h.Result.FinalHead)
	baseRef := strings.TrimSpace(h.Orca.BaseRef)
	prefix, suffix := "refs/remotes/", "/"+branch
	if branch == "" || base == "" || !publicationFullCommitPattern.MatchString(finalHead) || !strings.HasPrefix(baseRef, prefix) || !strings.HasSuffix(baseRef, suffix) {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication branch, base, final head, or remote authority is incomplete")
	}
	remote := strings.TrimSuffix(strings.TrimPrefix(baseRef, prefix), suffix)
	if remote == "" || strings.ContainsAny(remote, " \t\r\n") {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication remote authority is invalid")
	}
	return issueOpsPublicationIdentity{
		Provider: provider, ProjectKey: projectKey, Remote: remote, Branch: branch, Base: base,
		LocalRef: "refs/heads/" + branch, RemoteRef: "refs/heads/" + branch, FinalHead: finalHead,
	}, nil
}

func resolveIssueOpsPublicationPushTarget(ctx context.Context, record IssueOpsRecord, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) (issueOpsPublicationIdentity, error) {
	if reader == nil {
		return identity, fmt.Errorf("publication ref verification dependency is unavailable")
	}
	target, err := reader.PushTarget(ctx, record.Repo, identity.Remote)
	if err != nil {
		return identity, err
	}
	if err := remote.ValidateGitRemoteMatchesIssue(record.IssueURL, target.URL, identity.Provider); err != nil {
		return identity, err
	}
	if err := validateGitLabCustomPortCapability(ctx, record, identity, reader); err != nil {
		return identity, err
	}
	identity.PushTargetSHA256 = target.Fingerprint
	return identity, nil
}

func validateGitLabCustomPortCapability(ctx context.Context, record IssueOpsRecord, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	if identity.Provider != "gitlab" {
		return nil
	}
	parsed, err := url.Parse(record.IssueURL)
	needsCapability := err == nil && (strings.Contains(parsed.Hostname(), ":") || parsed.Port() != "" && parsed.Port() != "443")
	if !needsCapability {
		return nil
	}
	capability, ok := reader.(IssueOpsGitLabCapabilityReader)
	if !ok {
		return fmt.Errorf("GitLab custom web port requires proven glab >= 1.82.0 capability")
	}
	version, err := capability.GitLabVersion(ctx, record.Repo)
	if err != nil || !glabVersionAtLeast182(version) {
		return fmt.Errorf("GitLab custom web port requires proven glab >= 1.82.0 capability")
	}
	return nil
}

func glabVersionAtLeast182(value string) bool {
	match := publicationGlabVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) != 4 {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	return major > 1 || major == 1 && minor >= 82
}

func verifyIssueOpsPublicationHeads(ctx context.Context, repo string, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	if err := verifyIssueOpsLocalPublicationHead(ctx, repo, identity, reader); err != nil {
		return err
	}
	return verifyIssueOpsRemotePublicationHead(ctx, repo, identity, reader)
}

func verifyIssueOpsLocalPublicationHead(ctx context.Context, repo string, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	if reader == nil {
		return fmt.Errorf("publication ref verification dependency is unavailable")
	}
	localHead, err := reader.LocalRefHead(ctx, repo, identity.LocalRef)
	if err != nil {
		return err
	}
	if strings.TrimSpace(localHead) != identity.FinalHead {
		return fmt.Errorf("local publication ref head does not equal accepted final_head")
	}
	return nil
}

func verifyIssueOpsRemotePublicationHead(ctx context.Context, repo string, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	remoteHead, err := reader.RemoteRefHead(ctx, repo, identity.Remote, identity.PushTargetSHA256, identity.RemoteRef)
	if err != nil {
		return err
	}
	if strings.TrimSpace(remoteHead) != identity.FinalHead {
		return fmt.Errorf("remote publication ref head does not equal accepted final_head")
	}
	return nil
}

func validateIssueOpsPublishReceipt(record IssueOpsRecord, identity issueOpsPublicationIdentity) error {
	receipt := record.ExecutionHandoff.PublishReceipt
	if receipt == nil {
		return fmt.Errorf("durable publication receipt is required")
	}
	if receipt.Provider != identity.Provider || receipt.ProjectKey != identity.ProjectKey || receipt.Remote != identity.Remote || receipt.PushTargetSHA256 != identity.PushTargetSHA256 || !publicationSHA256Pattern.MatchString(receipt.PushTargetSHA256) || receipt.Branch != identity.Branch || receipt.Base != identity.Base || receipt.RemoteRef != identity.RemoteRef || receipt.FinalHead != identity.FinalHead || strings.TrimSpace(receipt.VerifiedAt) == "" {
		return fmt.Errorf("durable publication receipt does not match current accepted authority")
	}
	return nil
}

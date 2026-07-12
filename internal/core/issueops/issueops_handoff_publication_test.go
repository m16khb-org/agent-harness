package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

type publicationRefFake struct {
	localHead       string
	remoteHead      string
	remoteHeads     []string
	remoteHeadCalls int
	remoteHeadHook  func(int)
	localRef        string
	remote          string
	remoteRef       string
	pushCalls       int
	pushHead        string
	pushTarget      string
	pushTargets     []string
	pushTargetCalls int
	glabVersion     string
	trace           []string
}

func (f *publicationRefFake) GitLabVersion(_ context.Context, _ string) (string, error) {
	if f.glabVersion == "" {
		return "1.92.0", nil
	}
	return f.glabVersion, nil
}

func (f *publicationRefFake) PushTarget(_ context.Context, _, _ string) (IssueOpsPublicationPushTarget, error) {
	f.trace = append(f.trace, "target")
	target := f.pushTarget
	if len(f.pushTargets) > 0 {
		index := f.pushTargetCalls
		if index >= len(f.pushTargets) {
			index = len(f.pushTargets) - 1
		}
		target = f.pushTargets[index]
	}
	f.pushTargetCalls++
	if target == "" {
		target = "https://github.com/acme/repo.git"
	}
	sum := sha256.Sum256([]byte(target))
	return IssueOpsPublicationPushTarget{URL: target, Fingerprint: hex.EncodeToString(sum[:])}, nil
}

func (f *publicationRefFake) PushExact(_ context.Context, _, _, _, _, finalHead string) error {
	f.pushCalls++
	f.pushHead = finalHead
	return nil
}

func TestAcceptedHandoffPublicationPersistsAmbiguousInventoryRecovery(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	client := handoffDispatchFake(record)
	client.terminalListErr = errors.New("truncated terminal inventory")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("ambiguous publication inventory was accepted")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.PublicationRecovery == nil || persisted.ExecutionHandoff.PublicationRecovery.Code != "publication_inventory_ambiguous" || persisted.ExecutionHandoff.PublishReceipt != nil {
		t.Fatalf("publication ambiguity was not persisted: %#v", persisted.ExecutionHandoff)
	}
}

func TestLegacyNilHandoffPublishRefusesWithoutMutation(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	reader := &publicationRefFake{}
	client := handoffDispatchFake(record)
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "closed accepted execution handoff") {
		t.Fatalf("legacy nil-handoff publish error = %v", err)
	}
	if reader.pushCalls != 0 {
		t.Fatalf("legacy nil-handoff crossed push boundary: calls=%d", reader.pushCalls)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("legacy nil-handoff publish changed durable state")
	}
}

func (f *publicationRefFake) LocalRefHead(_ context.Context, _ string, ref string) (string, error) {
	f.trace = append(f.trace, "local")
	f.localRef = ref
	return f.localHead, nil
}

func (f *publicationRefFake) RemoteRefHead(_ context.Context, _ string, remote, _, ref string) (string, error) {
	f.trace = append(f.trace, "remote")
	f.remote, f.remoteRef = remote, ref
	if f.remoteHeadHook != nil {
		f.remoteHeadHook(f.remoteHeadCalls)
	}
	head := f.remoteHead
	if len(f.remoteHeads) > 0 {
		index := f.remoteHeadCalls
		if index >= len(f.remoteHeads) {
			index = len(f.remoteHeads) - 1
		}
		head = f.remoteHeads[index]
	}
	f.remoteHeadCalls++
	return head, nil
}

func TestGitPublicationPushUsesImmutableFinalHeadArgv(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "push-argv")
	envLogPath := filepath.Join(t.TempDir(), "push-env")
	script := filepath.Join(bin, "git")
	target := "https://github.com/acme/repo.git"
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = remote ]; then printf '%s\\n' '" + target + "'; exit 0; fi\n" +
		"if [ \"$1 $2\" = 'rev-parse --git-common-dir' ]; then printf '.git\\n'; exit 0; fi\n" +
		"if [ \"$1 $2 $3\" = 'rev-parse --git-path config.worktree' ]; then printf '.git/config.worktree\\n'; exit 0; fi\n" +
		"if [ \"$1 $2 $3 $4\" = 'config --show-origin --includes --list' ]; then exit 0; fi\n" +
		"if [ \"$1\" = config ]; then exit 1; fi\n" +
		"printf '%s\\n' \"$@\" > \"$PUSH_ARGV_LOG\"\n" +
		"printf '%s' \"$GIT_TERMINAL_PROMPT\" > \"$PUSH_ENV_LOG\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PUSH_ARGV_LOG", logPath)
	t.Setenv("PUSH_ENV_LOG", envLogPath)
	finalHead := strings.Repeat("a", 40)
	sum := sha256.Sum256([]byte(target))
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := (GitIssueOpsHandoffPublicationReader{}).PushExact(context.Background(), repo, "origin", hex.EncodeToString(sum[:]), "16-demo", finalHead); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "push\n--\n" + target + "\n" + finalHead + ":refs/heads/16-demo\n"
	if string(got) != want {
		t.Fatalf("push argv = %q, want %q", got, want)
	}
	if got, err := os.ReadFile(envLogPath); err != nil || string(got) != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, err=%v", got, err)
	}
}

func TestGitPublicationPushTargetUsesRealGitInsteadOfAndDistinctPushURL(t *testing.T) {
	repo := t.TempDir()
	runPublicationGitTest(t, repo, "init", "-q")
	runPublicationGitTest(t, repo, "remote", "add", "origin", "https://github.com/acme/fetch-only.git")
	runPublicationGitTest(t, repo, "config", "url.https://github.com/.insteadOf", "work:")
	runPublicationGitTest(t, repo, "remote", "set-url", "--push", "origin", "work:acme/repo.git")
	target, err := (GitIssueOpsHandoffPublicationReader{}).PushTarget(context.Background(), repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if target.URL != "https://github.com/acme/repo.git" {
		t.Fatalf("effective push target after insteadOf = %q", target.URL)
	}
	fetchURL := strings.TrimSpace(runPublicationGitTest(t, repo, "remote", "get-url", "origin"))
	if fetchURL != "https://github.com/acme/fetch-only.git" {
		t.Fatalf("fetch URL was confused with push authority: %q", fetchURL)
	}
}

func TestNestedGitRewriteToWrongAuthorityBlocksBeforePushOrProvider(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	if err := os.MkdirAll(record.Repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(record.Repo, "init", "-q"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(record.Repo, "remote", "add", "origin", "https://github.com/acme/repo.git"); code != 0 {
		t.Fatalf("git remote add failed: %s", stderr)
	}
	for _, args := range [][]string{
		{"remote", "set-url", "origin", "alias:acme/repo.git"},
		{"config", "url.https://github.com/acme/.insteadOf", "alias:acme/"},
		{"config", "url.https://evil.example/.insteadOf", "https://github.com/acme/"},
	} {
		if code, _, stderr := preflight.GitCmd(record.Repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	lease := handoffDispatchFake(record)
	lease.terminals = nil
	providerCalls := 0
	_, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, Title: "title", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch,
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
	}, GitIssueOpsHandoffPublicationReader{}, lease, func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		return port.IssueProviderCreatePullRequestResult{}, nil
	})
	if err == nil || providerCalls != 0 {
		t.Fatalf("nested alias->good->evil error=%v providerCalls=%d", err, providerCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || persisted.RemoteCreateClaim != nil {
		t.Fatalf("nested rewrite crossed claim boundary: claim=%#v readErr=%v", persisted.RemoteCreateClaim, readErr)
	}
}

func TestPublicationURLRewriteCycleFailsClosed(t *testing.T) {
	_, err := resolvePublicationURLRewrites("alias:repo", []publicationGitURLRule{{base: "good:", prefix: "alias:"}, {base: "alias:", prefix: "good:"}})
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("rewrite cycle error = %v", err)
	}
}

func TestPublicationRejectsPreexistingEffectiveIncludeConfigLock(t *testing.T) {
	repo := t.TempDir()
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q"); code != 0 {
		t.Fatal(stderr)
	}
	include := filepath.Join(t.TempDir(), "publication-include.config")
	if err := os.WriteFile(include, []byte("[user]\n\tname = publication fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "config", "include.path", include); code != 0 {
		t.Fatal(stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "remote", "add", "origin", "https://github.com/acme/repo.git"); code != 0 {
		t.Fatal(stderr)
	}
	if err := os.WriteFile(include+".lock", []byte("busy"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (GitIssueOpsHandoffPublicationReader{}).PushTarget(context.Background(), repo, "origin")
	if err == nil || !strings.Contains(err.Error(), "config lock") {
		t.Fatalf("pre-existing include config lock error = %v", err)
	}
}

func TestPublicationRejectsPreexistingUnrelatedEffectiveGlobalConfigLock(t *testing.T) {
	repo := t.TempDir()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = publication fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runPublicationGitTest(t, repo, "init", "-q")
	runPublicationGitTest(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	if err := os.WriteFile(global+".lock", []byte("busy"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (GitIssueOpsHandoffPublicationReader{}).PushTarget(context.Background(), repo, "origin")
	if err == nil || !strings.Contains(err.Error(), "config lock") {
		t.Fatalf("pre-existing unrelated effective global config lock error = %v", err)
	}
}

func TestPublicationGlobalRewriteRaceCannotRedirectPush(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	repo, good, evil := t.TempDir(), t.TempDir(), t.TempDir()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = publication fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runPublicationGitTest(t, repo, "init", "-q")
	runPublicationGitTest(t, repo, "config", "user.name", "Publication Test")
	runPublicationGitTest(t, repo, "config", "user.email", "publication@example.test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("publication\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runPublicationGitTest(t, repo, "add", "README.md")
	runPublicationGitTest(t, repo, "commit", "-q", "-m", "test: publication")
	finalHead := strings.TrimSpace(runPublicationGitTest(t, repo, "rev-parse", "HEAD"))
	runPublicationGitTest(t, good, "init", "-q", "--bare")
	runPublicationGitTest(t, evil, "init", "-q", "--bare")
	runPublicationGitTest(t, repo, "remote", "add", "origin", good)

	reader := GitIssueOpsHandoffPublicationReader{}
	target, err := reader.PushTarget(context.Background(), repo, "origin")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "rewrite-succeeded")
	script := filepath.Join(bin, "git")
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = push ]; then \"$REAL_GIT\" config --global \"url.$EVIL_REPO.insteadOf\" \"$GOOD_REPO\" && printf succeeded > \"$REWRITE_MARKER\"; fi\n" +
		"exec \"$REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REAL_GIT", realGit)
	t.Setenv("GOOD_REPO", good)
	t.Setenv("EVIL_REPO", evil)
	t.Setenv("REWRITE_MARKER", marker)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := reader.PushExact(context.Background(), repo, "origin", target.Fingerprint, "16-demo", finalHead); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("global rewrite raced through held config lock: %v", err)
	}
	if got := strings.TrimSpace(runPublicationGitTest(t, good, "rev-parse", "refs/heads/16-demo")); got != finalHead {
		t.Fatalf("good destination head = %q, want %q", got, finalHead)
	}
	cmd := exec.Command(realGit, "--git-dir", evil, "rev-parse", "--verify", "refs/heads/16-demo")
	if err := cmd.Run(); err == nil {
		t.Fatal("global rewrite race changed evil destination")
	}
}

func TestPublicationMissingConfigAuthorityParentFailsBeforePushAndLeavesDestinationsUnchanged(t *testing.T) {
	for _, mode := range []string{"absent XDG", "absent include parent"} {
		t.Run(mode, func(t *testing.T) {
			repo, good, evil := t.TempDir(), t.TempDir(), t.TempDir()
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			xdg := filepath.Join(t.TempDir(), "absent-xdg")
			if mode == "absent include parent" {
				xdg = t.TempDir()
				if err := os.MkdirAll(filepath.Join(xdg, "git"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("XDG_CONFIG_HOME", xdg)
			runPublicationGitTest(t, repo, "init", "-q")
			runPublicationGitTest(t, repo, "config", "user.name", "Publication Test")
			runPublicationGitTest(t, repo, "config", "user.email", "publication@example.test")
			if mode == "absent include parent" {
				missingInclude := filepath.Join(t.TempDir(), "absent-parent", "included.config")
				runPublicationGitTest(t, repo, "config", "include.path", missingInclude)
			}
			if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("publication\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			runPublicationGitTest(t, repo, "add", "README.md")
			runPublicationGitTest(t, repo, "commit", "-q", "-m", "test: publication")
			finalHead := strings.TrimSpace(runPublicationGitTest(t, repo, "rev-parse", "HEAD"))
			runPublicationGitTest(t, good, "init", "-q", "--bare")
			runPublicationGitTest(t, evil, "init", "-q", "--bare")
			runPublicationGitTest(t, repo, "remote", "add", "origin", good)
			targetSum := sha256.Sum256([]byte(good))
			err := (GitIssueOpsHandoffPublicationReader{}).PushExact(context.Background(), repo, "origin", hex.EncodeToString(targetSum[:]), "16-demo", finalHead)
			if err == nil || !strings.Contains(err.Error(), "config authority parent") {
				t.Fatalf("missing authority parent error = %v", err)
			}
			for name, destination := range map[string]string{"good": good, "evil": evil} {
				cmd := exec.Command("git", "--git-dir", destination, "rev-parse", "--verify", "refs/heads/16-demo")
				if err := cmd.Run(); err == nil {
					t.Fatalf("%s destination changed despite missing config authority parent", name)
				}
			}
		})
	}
}

func TestGitPublicationMultiplePushURLsRejectsAndLeavesBothDestinationsUnchanged(t *testing.T) {
	repo, first, second := t.TempDir(), t.TempDir(), t.TempDir()
	runPublicationGitTest(t, repo, "init", "-q")
	runPublicationGitTest(t, repo, "config", "user.name", "Publication Test")
	runPublicationGitTest(t, repo, "config", "user.email", "publication@example.test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("publication\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runPublicationGitTest(t, repo, "add", "README.md")
	runPublicationGitTest(t, repo, "commit", "-q", "-m", "test: publication")
	finalHead := strings.TrimSpace(runPublicationGitTest(t, repo, "rev-parse", "HEAD"))
	runPublicationGitTest(t, first, "init", "-q", "--bare")
	runPublicationGitTest(t, second, "init", "-q", "--bare")
	runPublicationGitTest(t, repo, "remote", "add", "origin", first)
	runPublicationGitTest(t, repo, "config", "--add", "remote.origin.pushurl", first)
	runPublicationGitTest(t, repo, "config", "--add", "remote.origin.pushurl", second)
	reader := GitIssueOpsHandoffPublicationReader{}
	if _, err := reader.PushTarget(context.Background(), repo, "origin"); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("multiple push URLs were accepted: %v", err)
	}
	if err := reader.PushExact(context.Background(), repo, "origin", strings.Repeat("0", 64), "16-demo", finalHead); err == nil {
		t.Fatal("multiple push URLs crossed push boundary")
	}
	for _, destination := range []string{first, second} {
		cmd := exec.Command("git", "--git-dir", destination, "rev-parse", "--verify", "refs/heads/16-demo")
		if err := cmd.Run(); err == nil {
			t.Fatalf("rejected multiple-pushurl publication changed destination %s", destination)
		}
	}
}

func TestGitPublicationEqualDuplicatePushURLsStillReject(t *testing.T) {
	repo := t.TempDir()
	runPublicationGitTest(t, repo, "init", "-q")
	runPublicationGitTest(t, repo, "remote", "add", "origin", "https://github.com/acme/repo.git")
	runPublicationGitTest(t, repo, "config", "--add", "remote.origin.pushurl", "https://github.com/acme/repo.git")
	runPublicationGitTest(t, repo, "config", "--add", "remote.origin.pushurl", "https://github.com/acme/repo.git")
	if _, err := (GitIssueOpsHandoffPublicationReader{}).PushTarget(context.Background(), repo, "origin"); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("equal duplicate push URLs were collapsed: %v", err)
	}
}

func runPublicationGitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return string(out)
}

func TestGitPublicationAdapterBoundsRedactsTimesOutAndRejectsUnsafeTokens(t *testing.T) {
	bin := t.TempDir()
	script := filepath.Join(bin, "git")
	body := "#!/bin/sh\n" +
		"if [ \"$GIT_TEST_MODE\" = timeout ]; then exec sleep 2; fi\n" +
		"if [ \"$GIT_TEST_MODE\" = stderr ]; then { printf 'api_key=abcdefghijklmnopqrstuvwxyz123456\\n'; i=0; while [ $i -lt 6000 ]; do printf x; i=$((i+1)); done; printf '\\n'; } >&2; exit 1; fi\n" +
		"printf '%s\\n' \"$@\" > \"$PUSH_ARGV_LOG\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath := filepath.Join(t.TempDir(), "argv")
	t.Setenv("PUSH_ARGV_LOG", logPath)
	reader := GitIssueOpsHandoffPublicationReader{}

	t.Setenv("GIT_TEST_MODE", "stderr")
	_, err := reader.LocalRefHead(context.Background(), t.TempDir(), "refs/heads/16-demo")
	if err == nil || len(err.Error()) > publicationDiagnosticLimit+256 || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("oversized secret diagnostic was not bounded/redacted: len=%d err=%v", len(errString(err)), err)
	}

	t.Setenv("GIT_TEST_MODE", "timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := reader.RemoteRefHead(ctx, t.TempDir(), "origin", strings.Repeat("0", 64), "refs/heads/16-demo"); err == nil || time.Since(started) > time.Second {
		t.Fatalf("publication timeout was not honored promptly: elapsed=%s err=%v", time.Since(started), err)
	}

	t.Setenv("GIT_TEST_MODE", "")
	_ = os.Remove(logPath)
	if err := reader.PushExact(context.Background(), t.TempDir(), "--upload-pack=evil", strings.Repeat("0", 64), "16-demo", strings.Repeat("a", 40)); err == nil {
		t.Fatal("option-like remote was accepted")
	}
	if _, err := reader.RemoteRefHead(context.Background(), t.TempDir(), "origin", strings.Repeat("0", 64), "refs/heads/bad\nref"); err == nil {
		t.Fatal("control-containing ref was accepted")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("unsafe token reached git execution: %v", err)
	}
}

func TestGitPublicationLocalRefHeadUsesRealGitEndOfOptions(t *testing.T) {
	repo := initIssueOpsRepo(t)
	want := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "refs/heads/main"))
	got, err := (GitIssueOpsHandoffPublicationReader{}).LocalRefHead(context.Background(), repo, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("local ref head = %s, want %s", got, want)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAcceptedHandoffPublicationReceiptRequiresExactLocalAndRemoteFinalHead(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			stateRoot, record := acceptedPublicationHandoff(t, provider)
			finalHead := record.ExecutionHandoff.Result.FinalHead
			for _, tt := range []struct {
				name       string
				localHead  string
				remoteHead string
			}{
				{name: "wrong local", localHead: strings.Repeat("1", 40), remoteHead: finalHead},
				{name: "wrong remote", localHead: finalHead, remoteHead: strings.Repeat("2", 40)},
			} {
				t.Run(tt.name, func(t *testing.T) {
					reader := &publicationRefFake{localHead: tt.localHead, remoteHead: tt.remoteHead, pushTarget: publicationTargetForProvider(provider)}
					client := handoffDispatchFake(record)
					client.terminals = nil
					if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock()); err == nil {
						t.Fatalf("%s publication accepted mismatched heads", provider)
					}
					persisted, err := ReadIssueOps(stateRoot, record.ID)
					if err != nil {
						t.Fatal(err)
					}
					if persisted.ExecutionHandoff.PublishReceipt != nil {
						t.Fatalf("mismatched head persisted receipt: %#v", persisted.ExecutionHandoff.PublishReceipt)
					}
				})
			}
		})
	}
}

func TestInitialPublishUsesTargetThenLocalThenRemoteOrdering(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: publicationTargetForProvider("github")}
	client := handoffDispatchFake(record)
	client.terminals = nil
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	if reader.pushCalls != 1 || !reflect.DeepEqual(reader.trace, []string{"target", "local", "remote"}) {
		t.Fatalf("initial publication order=%v pushCalls=%d", reader.trace, reader.pushCalls)
	}
}

func TestGitLabCustomPortPublicationRequiresGlab182BeforePush(t *testing.T) {
	for _, authority := range []struct {
		name, issueURL, target string
	}{
		{name: "custom-port", issueURL: "https://gitlab.example:8443/acme/repo/-/issues/16", target: "https://gitlab.example:8443/acme/repo.git"},
		{name: "ipv6-implicit-port", issueURL: "https://[2001:db8::1]/acme/repo/-/issues/16", target: "https://[2001:db8::1]/acme/repo.git"},
		{name: "ipv6-explicit-443", issueURL: "https://[2001:db8::1]:443/acme/repo/-/issues/16", target: "https://[2001:db8::1]:443/acme/repo.git"},
	} {
		for _, tt := range []struct {
			version   string
			wantPush  int
			wantError bool
		}{
			{version: "1.81.0", wantError: true},
			{version: "1.82.0", wantPush: 1},
		} {
			t.Run(authority.name+"/"+tt.version, func(t *testing.T) {
				stateRoot, record := acceptedPublicationHandoff(t, "gitlab")
				record.IssueURL = authority.issueURL
				record.BranchPrepare.IssueURL = record.IssueURL
				record, err := WriteIssueOps(stateRoot, record)
				if err != nil {
					t.Fatal(err)
				}
				finalHead := record.ExecutionHandoff.Result.FinalHead
				reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: authority.target, glabVersion: tt.version}
				client := handoffDispatchFake(record)
				client.terminals = nil
				_, err = RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock())
				if (err != nil) != tt.wantError {
					t.Fatalf("glab %s error = %v, wantError=%v", tt.version, err, tt.wantError)
				}
				if reader.pushCalls != tt.wantPush {
					t.Fatalf("glab %s push calls = %d, want %d", tt.version, reader.pushCalls, tt.wantPush)
				}
			})
		}
	}
}

func TestAcceptedHandoffPublicationReceiptHasGitHubGitLabParityAndFailsStale(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			stateRoot, record := acceptedPublicationHandoff(t, provider)
			finalHead := record.ExecutionHandoff.Result.FinalHead
			reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: publicationTargetForProvider(provider)}
			client := handoffDispatchFake(record)
			client.terminals = nil
			persisted, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			receipt := persisted.ExecutionHandoff.PublishReceipt
			if receipt == nil || receipt.Provider != provider || receipt.Branch != record.Branch || receipt.FinalHead != finalHead || receipt.Remote != "origin" || receipt.RemoteRef != "refs/heads/"+record.Branch {
				t.Fatalf("%s receipt = %#v", provider, receipt)
			}
			if reader.pushCalls != 1 || reader.pushHead != finalHead {
				t.Fatalf("%s push did not use immutable final head: %#v", provider, reader)
			}
			if reader.localRef != "refs/heads/"+record.Branch || reader.remote != "origin" || reader.remoteRef != "refs/heads/"+record.Branch {
				t.Fatalf("%s ref verification = %#v", provider, reader)
			}
			reader.trace = nil
			if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, persisted, provider, record.Branch, record.BranchPrepare.BaseBranch, reader, client); err != nil {
				t.Fatalf("%s matching publication receipt: %v", provider, err)
			}
			if !reflect.DeepEqual(reader.trace, []string{"target", "local", "remote"}) {
				t.Fatalf("%s valid receipt validation order = %#v", provider, reader.trace)
			}
			reader.remoteHead = strings.Repeat("3", 40)
			if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, persisted, provider, record.Branch, record.BranchPrepare.BaseBranch, reader, client); err == nil {
				t.Fatalf("%s stale remote receipt authorized publication", provider)
			}
			reader.remoteHead = finalHead
			for _, mismatch := range []struct{ provider, head, base string }{
				{provider: otherProvider(provider), head: record.Branch, base: record.BranchPrepare.BaseBranch},
				{provider: provider, head: "wrong-branch", base: record.BranchPrepare.BaseBranch},
				{provider: provider, head: record.Branch, base: "wrong-base"},
			} {
				if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, persisted, mismatch.provider, mismatch.head, mismatch.base, reader, client); err == nil {
					t.Fatalf("%s caller mismatch authorized publication: %#v", provider, mismatch)
				}
			}
		})
	}
}

func TestAcceptedHandoffPublicationRequiresDurableReceipt(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
	client := handoffDispatchFake(record)
	client.terminals = nil
	if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, record, "github", record.Branch, record.BranchPrepare.BaseBranch, reader, client); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("missing receipt error = %v", err)
	}
}

func TestAcceptedHandoffPublicationRejectsDifferentNativeCoordinatorBeforePush(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: publicationTargetForProvider("github")}
	lease := handoffDispatchFake(record)
	lease.terminals = nil
	req := coordinatorPublishRequest(record)
	req.SessionID = "different-coordinator-session"
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, req, reader, lease, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "sealed coordinator native session") {
		t.Fatalf("different native coordinator publish = %v", err)
	}
	if reader.pushCalls != 0 || len(reader.trace) != 0 {
		t.Fatalf("different-session publish reached publication reader: trace=%v pushCalls=%d", reader.trace, reader.pushCalls)
	}
}

func TestRawSchemaV5PublishReceiptHasExecutableLockedReattestationToV6(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	coordinator := *record.ExecutionHandoff.CoordinatorSession
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var frozen map[string]json.RawMessage
	if err := json.Unmarshal(raw, &frozen); err != nil {
		t.Fatal(err)
	}
	frozen["schema_version"] = json.RawMessage(`5`)
	var execution map[string]json.RawMessage
	if err := json.Unmarshal(frozen["execution_handoff"], &execution); err != nil {
		t.Fatal(err)
	}
	delete(execution, "coordinator_session")
	execution["publish_receipt"] = json.RawMessage(`{"provider":"github","remote":"origin","branch":"16-demo","remote_ref":"refs/heads/16-demo","final_head":"` + record.ExecutionHandoff.Result.FinalHead + `","verified_at":"2026-07-12T00:00:00Z"}`)
	frozen["execution_handoff"], err = json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: publicationTargetForProvider("github")}
	lease := handoffDispatchFake(record)
	lease.terminals = nil
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	withoutApproval := IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true, Host: coordinator.Host, SessionID: coordinator.SessionID, AgentID: coordinator.AgentID, SourceCWD: record.Repo}
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, withoutApproval, reader, lease, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "legacy coordinator seal") {
		t.Fatalf("literal HEAD-era v5 publish without explicit seal approval = %v", err)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !bytes.Equal(after, before) {
		t.Fatal("rejected legacy coordinator seal rewrote schema-v5 bytes")
	}
	approvedJSON, err := json.Marshal(map[string]any{
		"id": record.ID, "confirm": true, "host": coordinator.Host, "session_id": coordinator.SessionID,
		"agent_id": coordinator.AgentID, "source_cwd": record.Repo, "approve_legacy_coordinator_seal": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var approved IssueOpsHandoffPublishRequest
	if err := json.Unmarshal(approvedJSON, &approved); err != nil {
		t.Fatal(err)
	}
	got, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, approved, reader, lease, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != 6 || got.ExecutionHandoff.CoordinatorSession == nil || *got.ExecutionHandoff.CoordinatorSession != coordinator || got.ExecutionHandoff.PublishReceipt == nil || !publicationSHA256Pattern.MatchString(got.ExecutionHandoff.PublishReceipt.PushTargetSHA256) || reader.pushCalls != 0 {
		t.Fatalf("raw v5 re-attestation = %#v pushCalls=%d", got.ExecutionHandoff.PublishReceipt, reader.pushCalls)
	}
	if !reflect.DeepEqual(reader.trace, []string{"target", "local", "remote"}) {
		t.Fatalf("raw v5 re-attestation order = %v", reader.trace)
	}
}

func TestRawSchemaV5ReattestationRejectsInjectedCoordinatorSessionWithoutRewrite(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	coordinator := *record.ExecutionHandoff.CoordinatorSession
	record.SchemaVersion = 5
	record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", Remote: "origin", Branch: record.Branch, RemoteRef: "refs/heads/" + record.Branch,
		FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-12T00:00:00Z",
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	reader := &publicationRefFake{localHead: record.ExecutionHandoff.Result.FinalHead, remoteHead: record.ExecutionHandoff.Result.FinalHead, pushTarget: publicationTargetForProvider("github")}
	lease := handoffDispatchFake(record)
	lease.terminals = nil
	req := IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true, Host: coordinator.Host, SessionID: coordinator.SessionID, AgentID: coordinator.AgentID, SourceCWD: record.Repo, ApproveLegacyCoordinatorSeal: true}
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, req, reader, lease, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "coordinator_session") {
		t.Fatalf("raw schema-v5 injected coordinator session re-attestation = %v", err)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !bytes.Equal(after, before) {
		t.Fatal("raw schema-v5 injected coordinator session was rewritten")
	}
	if len(reader.trace) != 0 || reader.pushCalls != 0 {
		t.Fatalf("raw schema-v5 injected coordinator session reached publication: trace=%v push=%d", reader.trace, reader.pushCalls)
	}
}

func TestAcceptedHandoffPublicationRejectsAnyPossibleWriterAndDispatchedAssignment(t *testing.T) {
	for _, tt := range []struct {
		name     string
		wantCode string
		setup    func(IssueOpsRecord, *dispatchOrcaFake)
	}{
		{name: "connected only terminal", wantCode: "publication_writer_conflict", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.terminals = []port.OrcaTerminal{{Handle: "term-other", PTYID: "pty-other", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true}}
		}},
		{name: "writable only terminal", wantCode: "publication_writer_conflict", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.terminals = []port.OrcaTerminal{{Handle: "term-other", PTYID: "pty-other", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Writable: true}}
		}},
		{name: "dispatched persisted terminal", wantCode: "publication_writer_conflict", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
			client.dispatchByTask = map[string]port.OrcaDispatch{"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, Status: "dispatched"}}
		}},
		{name: "dispatched vanished terminal", wantCode: "publication_inventory_ambiguous", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
			client.dispatchByTask = map[string]port.OrcaDispatch{"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: "term-vanished", Status: "dispatched"}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := acceptedPublicationHandoff(t, "github")
			client := handoffDispatchFake(record)
			client.terminals = nil
			tt.setup(record, client)
			finalHead := record.ExecutionHandoff.Result.FinalHead
			reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
			if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, coordinatorPublishRequest(record), reader, client, handoffPrepareTestClock()); err == nil {
				t.Fatal("possible writer authorized publication")
			}
			if reader.pushCalls != 0 {
				t.Fatalf("possible writer crossed push boundary: calls=%d", reader.pushCalls)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.PublicationRecovery == nil || persisted.ExecutionHandoff.PublicationRecovery.Code != tt.wantCode {
				t.Fatalf("known writer conflict was not persisted: %#v", persisted.ExecutionHandoff.PublicationRecovery)
			}
		})
	}
}

func acceptedPublicationHandoff(t *testing.T, provider string) (string, IssueOpsRecord) {
	t.Helper()
	if provider == "github" {
		stateRoot, record, claim, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
		accepted, err := AcceptIssueOpsHandoff(stateRoot, coordinatorAcceptRequest(record, IssueOpsHandoffAcceptRequest{
			ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
			ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
		}))
		if err != nil {
			t.Fatal(err)
		}
		return stateRoot, accepted
	}

	stateRoot, preparing := handoffDispatchRecord(t)
	preparing.IssueURL = "https://gitlab.example/acme/repo/-/issues/16"
	preparing.BranchPrepare.Provider = "gitlab"
	preparing.BranchPrepare.IssueURL = preparing.IssueURL
	preparing.ExecutionHandoff.Orca.ProviderIssueLinkStatus = "gitlab_native_unavailable"
	preparing, err := WriteIssueOps(stateRoot, preparing)
	if err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(preparing)
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, preparing.ID), client, handoffStartTestClock()); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(stateRoot, preparing.ID)
	if err != nil {
		t.Fatal(err)
	}
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/demo.go", "package internal\n")
	report := filepath.Join(record.WorktreePath, ".agent-harness", "research", "report.md")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte("# Turing evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: worker result"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	finish := handoffFinishRequest(claim, claimed)
	finish.FinalHead = strings.TrimSpace(preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD"))
	finish.TuringReportPath = ".agent-harness/research/report.md"
	finish.ChangedFiles = []string{"internal/demo.go", ".agent-harness/research/report.md"}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err != nil {
		t.Fatal(err)
	}
	accepted, err := AcceptIssueOpsHandoff(stateRoot, coordinatorAcceptRequest(record, IssueOpsHandoffAcceptRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
		ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, accepted
}

func otherProvider(provider string) string {
	if provider == "github" {
		return "gitlab"
	}
	return "github"
}

func publicationTargetForProvider(provider string) string {
	if provider == "gitlab" {
		return "https://gitlab.example/acme/repo.git"
	}
	return "https://github.com/acme/repo.git"
}

func coordinatorPublishRequest(record IssueOpsRecord) IssueOpsHandoffPublishRequest {
	req := IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true, SourceCWD: record.Repo}
	if record.ExecutionHandoff != nil && record.ExecutionHandoff.CoordinatorSession != nil {
		req.Host = record.ExecutionHandoff.CoordinatorSession.Host
		req.SessionID = record.ExecutionHandoff.CoordinatorSession.SessionID
		req.AgentID = record.ExecutionHandoff.CoordinatorSession.AgentID
	}
	return req
}

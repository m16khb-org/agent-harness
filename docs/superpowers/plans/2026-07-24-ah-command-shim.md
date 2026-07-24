# `ah` Command Shim Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `agent-harness` install/update가 안전한 `ah` command shim을 관리하고, symlink로 실행된 CLI가 repo 밖에서도 현재 checkout을 찾아 `ah update`를 수행하게 한다.

**Architecture:** `pathutil`이 executable symlink chain을 해석해 harness root 후보를 만든다. 공통 `installcli` path plan은 canonical `agent-harness` shim을 먼저 관리하고, 고정 target의 `ah` shim을 충돌 안전 정책으로 추가한다. Host adapter에는 로직을 복제하지 않는다.

**Tech Stack:** Go 1.26.3, 표준 라이브러리 `os`/`path/filepath`, 기존 `internal/adapter/installutil`, 표준 `testing`, Markdown 운영 문서.

## Global Constraints

- Canonical binary와 public command identity는 계속 `agent-harness`다.
- `ah`는 `~/.local/bin/agent-harness`를 가리키는 installer-owned convenience symlink다.
- 기존 `ah` regular file, directory, unrelated symlink를 덮어쓰지 않는다.
- Shell alias, wrapper script, 새 dependency를 추가하지 않는다.
- `auto`, `manual`, `skip` 모두 command shim 두 개를 관리하고 shell rc 처리만 달라진다.
- 실제 사용자 `~/.local/bin`과 host integration은 별도 명시 실행 전까지 변경하지 않는다.
- 사용자 소유 untracked 파일을 수정·stage·삭제하지 않는다.
- Commit/push는 별도 명시 권한이 없으므로 수행하지 않는다.

---

## File Map

- `cmd/harness/pathutil/path_helpers.go`: executable 원본/resolved path에서 harness root를 찾는 공통 로직.
- `cmd/harness/pathutil/path_helpers_test.go`: symlink chain root 탐색 regression.
- `cmd/harness/installcli/install_native_path.go`: canonical/short command shim plan과 `ah` 충돌 정책.
- `cmd/harness/installcli/install_command_test.go`: path mode별 두 shim 및 충돌/no-op 계약.
- `cmd/harness/installcli/install_command_helpers_test.go`: install result link assertion 재사용.
- `scripts/install-native.sh`: installer help에 `ah` command shim 표시.
- `README.md`: 사용자-facing `ah update` 사용법.
- `.agent-harness/operations/install.md`: install/update command shim 운영 계약.
- `.agent-harness/CONVENTIONS.md`: installer-owned command shim symlink 예외.
- `.agent-harness/ADR.md`: canonical identity + managed shorthand 결정과 기각 대안.
- `cmd/harness/testdata/response_contracts.golden.json`: `.agent-harness` 문서 변경으로 발생한 docs-index projection만 갱신.

---

### Task 1: Executable symlink를 따라 harness root 찾기

**Files:**

- Modify: `cmd/harness/pathutil/path_helpers.go:17-40`
- Test: `cmd/harness/pathutil/path_helpers_test.go`

**Interfaces:**

- Consumes: `FindUp(start, marker string) (string, bool)`
- Produces: `harnessRootFrom(marker, envRoot, cwd, executable string) string`
- Preserves: `HarnessRoot(marker string) string`

- [ ] **Step 1: symlink chain regression test를 추가한다**

```go
func TestHarnessRootFromFollowsExecutableSymlink(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join("skills", "atomic-commit-push", "SKILL.md")
	markerPath := filepath.Join(root, marker)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "agent-harness")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	userBin := filepath.Join(t.TempDir(), ".local", "bin")
	if err := os.MkdirAll(userBin, 0o755); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(userBin, "agent-harness")
	short := filepath.Join(userBin, "ah")
	if err := os.Symlink(binary, canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, short); err != nil {
		t.Fatal(err)
	}

	got := harnessRootFrom(marker, "", t.TempDir(), short)
	if got != root {
		t.Fatalf("harnessRootFrom symlink = %q, want %q", got, root)
	}
}
```

- [ ] **Step 2: RED를 확인한다**

Run:

```bash
go test ./cmd/harness/pathutil -run TestHarnessRootFromFollowsExecutableSymlink -count=1
```

Expected: compile FAIL with `undefined: harnessRootFrom`.

- [ ] **Step 3: 최소 root resolver를 구현한다**

```go
func HarnessRoot(marker string) string {
	envRoot := os.Getenv("HARNESS_ROOT")
	cwd, _ := os.Getwd()
	executable, _ := os.Executable()
	return harnessRootFrom(marker, envRoot, cwd, executable)
}

func harnessRootFrom(marker, envRoot, cwd, executable string) string {
	if envRoot != "" {
		if root, err := filepath.Abs(envRoot); err == nil {
			return root
		}
	}
	starts := []string{}
	if cwd != "" {
		starts = append(starts, cwd)
	}
	if executable != "" {
		executableDir := filepath.Dir(executable)
		starts = append(starts, executableDir, filepath.Dir(executableDir))
		if resolved, err := filepath.EvalSymlinks(executable); err == nil && resolved != executable {
			resolvedDir := filepath.Dir(resolved)
			starts = append(starts, resolvedDir, filepath.Dir(resolvedDir))
		}
	}
	for _, start := range starts {
		if root, ok := FindUp(start, marker); ok {
			return root
		}
	}
	if cwd != "" {
		return cwd
	}
	return "."
}
```

- [ ] **Step 4: GREEN과 기존 pathutil tests를 확인한다**

Run:

```bash
go test ./cmd/harness/pathutil -count=1
```

Expected: PASS.

---

### Task 2: 안전한 `ah` command shim 설치

**Files:**

- Modify: `cmd/harness/installcli/install_native_path.go:13-51`
- Test: `cmd/harness/installcli/install_command_test.go`

**Interfaces:**

- Consumes: `installutil.EnsureSymlinkPlan(target, path string, dryRun bool) (port.InstallLink, error)`
- Produces: `ensureShortCommandShimPlan(target, path string, dryRun bool) (port.InstallLink, error)`
- Preserves: `applyInstallPathPlan(result *port.NativeInstallResult, req port.NativeInstallRequest, mode string) error`

- [ ] **Step 1: 모든 path mode가 두 shim을 계획하는 failing assertions를 추가한다**

각 auto/manual/skip test에서 다음 assertion을 사용한다.

```go
canonical := filepath.Join(home, ".local", "bin", "agent-harness")
short := filepath.Join(home, ".local", "bin", "ah")
if !hasInstallLink(result.Links, short, canonical, true) {
	t.Fatalf("%s path mode did not plan ah command shim: %+v", mode, result.Links)
}
```

`skip` test는 `root := configureInstallCommandTest(t, home)`을 보존하고
canonical/short link 두 개도 함께 단언한다.

- [ ] **Step 2: path mode RED를 확인한다**

Run:

```bash
go test ./cmd/harness/installcli -run 'TestInstallCommandDryRun(Auto|Manual|Skip)PathMode' -count=1
```

Expected: FAIL because no link has path ending in `/.local/bin/ah`.

- [ ] **Step 3: matching/no-conflict와 두 collision regression을 추가한다**

```go
func TestInstallCommandShortShimKeepsMatchingLink(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	canonical := filepath.Join(home, ".local", "bin", "agent-harness")
	short := filepath.Join(home, ".local", "bin", "ah")
	if err := os.MkdirAll(filepath.Dir(short), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(canonical, short); err != nil {
		t.Fatal(err)
	}
	result := runInstallDryRunJSON(t, home, "install", "skip")
	if !hasInstallLink(result.Links, short, canonical, false) {
		t.Fatalf("matching ah shim was not preserved: %+v", result.Links)
	}
}

func TestInstallCommandShortShimRefusesExistingFile(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	short := filepath.Join(home, ".local", "bin", "ah")
	if err := os.MkdirAll(filepath.Dir(short), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(short, []byte("user command"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstallCommand("install", []string{"--dry-run", "--json", "--path-mode=skip"})
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace existing ah command") {
		t.Fatalf("existing ah file error = %v", err)
	}
	body, readErr := os.ReadFile(short)
	if readErr != nil || string(body) != "user command" {
		t.Fatalf("existing ah file changed: body=%q err=%v", body, readErr)
	}
}

func TestInstallCommandShortShimRefusesUnrelatedSymlink(t *testing.T) {
	home := t.TempDir()
	configureInstallCommandTest(t, home)
	short := filepath.Join(home, ".local", "bin", "ah")
	if err := os.MkdirAll(filepath.Dir(short), 0o755); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(home, "bin", "another-ah")
	if err := os.MkdirAll(filepath.Dir(unrelated), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(unrelated, short); err != nil {
		t.Fatal(err)
	}
	_, _, err := captureInstallCommandOutput(t, nil, func() error {
		return RunInstallCommand("install", []string{"--dry-run", "--json", "--path-mode=skip"})
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace existing ah command") {
		t.Fatalf("unrelated ah symlink error = %v", err)
	}
	target, readErr := os.Readlink(short)
	if readErr != nil || target != unrelated {
		t.Fatalf("unrelated ah symlink changed: target=%q err=%v", target, readErr)
	}
}
```

- [ ] **Step 4: collision RED를 확인한다**

Run:

```bash
go test ./cmd/harness/installcli -run 'TestInstallCommandShortShim' -count=1
```

Expected: matching-link test misses the planned link and collision tests do not return the required `ah`-specific error.

- [ ] **Step 5: strict short-shim helper와 path plan을 구현한다**

```go
func ensureShortCommandShimPlan(target, path string, dryRun bool) (port.InstallLink, error) {
	link := port.InstallLink{Path: path, Target: target}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return installutil.EnsureSymlinkPlan(target, path, dryRun)
	}
	if err != nil {
		return link, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return link, fmt.Errorf("refusing to replace existing ah command: %s", path)
	}
	current, err := os.Readlink(path)
	if err != nil {
		return link, err
	}
	if current != target {
		return link, fmt.Errorf("refusing to replace existing ah command symlink %s -> %s", path, current)
	}
	return link, nil
}
```

`applyInstallPathPlan`의 canonical link 성공 직후 추가한다.

```go
shortCommandPath := filepath.Join(userBin, "ah")
shortLink, shortErr := ensureShortCommandShimPlan(commandPath, shortCommandPath, req.DryRun)
result.Links = append(result.Links, shortLink)
if shortErr != nil {
	return shortErr
}
```

manual/skip message의 단일 `command shim` 표현을 `command shims`와 두 path가
드러나는 문구로 바꾼다.

- [ ] **Step 6: installcli GREEN을 확인한다**

Run:

```bash
go test ./cmd/harness/installcli -count=1
```

Expected: PASS.

---

### Task 3: Repo 밖 `ah update` smoke를 고정한다

**Files:**

- Modify: `cmd/harness/pathutil/path_helpers_test.go`
- Test: `cmd/harness/updatecli/update_bootstrap_test.go`

**Interfaces:**

- Consumes: Task 1의 `harnessRootFrom`
- Consumes: Task 2의 `ah -> agent-harness -> repo binary` link shape
- Produces: repo 밖 cwd에서도 resolved root가 update installer path를 선택한다는 regression evidence

- [ ] **Step 1: update wrapper가 resolved root의 script를 쓰는 regression test를 추가한다**

`updatecli`는 composition root가 제공하는 `HarnessRoot`를 소비하므로, 실제
symlink 해석은 Task 1 unit test가 담당한다. Wrapper test에는 repo 밖 cwd와
resolved root를 명시해 그 root의 script가 runner에 전달되는지 고정한다.

```go
func TestRunUpdateUsesResolvedHarnessRootOutsideCheckout(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "scripts", "install-native.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCWD) })
	Configure(Deps{HarnessRoot: func() string { return root }})
	t.Cleanup(Reset)

	var got string
	restore := stubInstallScriptCommandRunner(t, func(name string, args ...string) error {
		got = name
		return nil
	})
	defer restore()
	restoreDaemon := stubPostInstallDaemonRefresh(t, func() (bool, error) { return false, nil })
	defer restoreDaemon()
	restoreMCP := stubPostInstallMCPProxyRefresh(t, func() (int, error) { return 0, nil })
	defer restoreMCP()

	if err := runUpdate([]string{"--dry-run", "--path-mode=skip"}); err != nil {
		t.Fatal(err)
	}
	if got != script {
		t.Fatalf("update script = %q, want %q", got, script)
	}
}
```

- [ ] **Step 2: wrapper regression을 실행한다**

Run:

```bash
go test ./cmd/harness/updatecli -run TestRunUpdateUsesResolvedHarnessRootOutsideCheckout -count=1
```

Expected: PASS once the test uses the injected resolved root; this pins update's consumption boundary while Task 1 holds the RED/GREEN root defect.

- [ ] **Step 3: 세 package의 통합 GREEN을 확인한다**

Run:

```bash
go test ./cmd/harness/pathutil ./cmd/harness/installcli ./cmd/harness/updatecli -count=1
```

Expected: PASS.

---

### Task 4: 사용자·운영·결정 문서를 현재 계약과 맞춘다

**Files:**

- Modify: `scripts/install-native.sh:25-28`
- Modify: `README.md:48-56`
- Modify: `.agent-harness/operations/install.md`
- Modify: `.agent-harness/CONVENTIONS.md:140-145`
- Modify: `.agent-harness/ADR.md`
- Update generated projection: `cmd/harness/testdata/response_contracts.golden.json`

**Interfaces:**

- Consumes: Tasks 1–3의 최종 command/root/collision behavior
- Produces: 사람용 install 문서와 agent docs-index contract

- [ ] **Step 1: installer help contract test를 먼저 강화한다**

`internal/adapter/install_contract_matrix_test.go`의 script contract test에 다음
문구 존재 assertion을 추가한다.

```go
for _, want := range []string{
	"~/.local/bin/agent-harness",
	"~/.local/bin/ah",
} {
	if !strings.Contains(script, want) {
		t.Fatalf("install-native.sh user command help missing %q", want)
	}
}
```

- [ ] **Step 2: help RED를 확인한다**

Run:

```bash
go test ./internal/adapter -run TestInstallNativeScript -count=1
```

Expected: FAIL because current help does not mention `~/.local/bin/ah`.

- [ ] **Step 3: script와 문서를 수정한다**

`scripts/install-native.sh` help:

```text
The default auto mode creates ~/.local/bin/agent-harness plus the safe
~/.local/bin/ah shorthand, and adds ~/.local/bin to the detected shell rc
when it is not already on PATH.
```

README와 operations에는 다음 public examples를 포함한다.

```bash
ah update
ah inspect --json
```

문서에는 `agent-harness`가 canonical identity이고 `ah` 충돌은 overwrite 없이
실패하며, `ah`는 shell alias가 아닌 managed symlink임을 명시한다.

`CONVENTIONS.md`의 symlink 규칙은 다음으로 좁힌다.

```text
기본 symlink는 user skill 원본 연결과 installer-owned command shim
(`~/.local/bin/agent-harness`, `~/.local/bin/ah`)에만 사용한다.
```

`.agent-harness/ADR.md`에는 날짜 `2026-07-24`, source `user directive`, decision,
rationale, rejected shell alias/wrapper, collision policy, verification을 기록한다.

- [ ] **Step 4: docs-index golden을 의도적으로 갱신한다**

Run:

```bash
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -update -count=1
```

Expected: PASS and only the `.agent-harness/ADR.md` / `CONVENTIONS.md` docs-index
bytes, headings, or digest projection changes.

- [ ] **Step 5: 문서·adapter contract GREEN을 확인한다**

Run:

```bash
go test ./internal/adapter -run TestInstallNativeScript -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
git diff --check
```

Expected: all PASS.

---

### Task 5: 격리 smoke와 전체 검증

**Files:**

- No source changes expected.
- Inspect: all files listed in the File Map.

**Interfaces:**

- Consumes: Tasks 1–4
- Produces: completion evidence without mutating real user installation

- [ ] **Step 1: `gofmt`와 focused suite를 실행한다**

Run:

```bash
gofmt -w cmd/harness/pathutil/path_helpers.go cmd/harness/pathutil/path_helpers_test.go cmd/harness/installcli/install_native_path.go cmd/harness/installcli/install_command_test.go cmd/harness/updatecli/update_bootstrap_test.go internal/adapter/install_contract_matrix_test.go
go test ./cmd/harness/pathutil ./cmd/harness/installcli ./cmd/harness/updatecli ./internal/adapter -count=1
```

Expected: PASS.

- [ ] **Step 2: isolated install/update smoke를 실행한다**

Use a `mktemp -d` root, create only fixture `HOME/.local/bin` links, invoke the
freshly built binary from an outside cwd, and remove only that exact temp root.

Run:

```bash
go build -o bin/agent-harness ./cmd/harness
tmp_root="$(mktemp -d)"
mkdir -p "$tmp_root/home/.local/bin" "$tmp_root/outside"
ln -s "$PWD/bin/agent-harness" "$tmp_root/home/.local/bin/agent-harness"
ln -s "$tmp_root/home/.local/bin/agent-harness" "$tmp_root/home/.local/bin/ah"
(cd "$tmp_root/outside" && HOME="$tmp_root/home" "$tmp_root/home/.local/bin/ah" update --dry-run --path-mode=skip --json)
rm -rf "$tmp_root"
```

Expected: exit 0; JSON `root` equals the current checkout and `links` contains
both fixture-home command shims. The command is dry-run and writes no host config.

- [ ] **Step 3: 마지막 all-or-nothing verification wave를 실행한다**

Run in this order, restarting from the first command if any command fails:

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
git diff --check
```

Expected:

- all commands exit 0;
- self-verify top-level `ok=true`, `termination_eligible=true`, and
  `summary.termination_eligible=true`;
- no unrelated user-owned file appears in the diff.

- [ ] **Step 4: final diff와 user installation 비변경을 확인한다**

Run:

```bash
git status --short
git diff --stat
command -v ah || true
```

Expected: only planned source/docs/golden changes plus pre-existing unrelated
untracked files. 실제 `~/.local/bin/ah`는 생성하지 않았으므로 live `command -v ah`
결과는 작업 전 상태와 같다.

# `io` command shim 설계

## 상태

사용자 승인 후 작성된 구현 전 설계다. `issueops`를 canonical command로
유지하면서 `io`를 짧은 convenience command로 제공한다.

## 배경

현재 native installer는 `~/.local/bin/issueops` symlink를 만들고
`--path-mode=auto`에서 필요하면 `~/.local/bin`을 shell `PATH`에 추가한다.
`install`, `install-native`, `bootstrap`, `update`는 이 설치 경로를 공유한다.

현재 설치 상태에서 `issueops`는 `PATH`에서 발견되지만 `io`는 없다. 또한
repo 밖에서 `issueops update --dry-run`을 실행하면 executable symlink의
실제 target을 따라가지 않아 현재 디렉터리를 harness root로 잘못 선택하고
`scripts/install-native.sh`를 찾지 못한다. 따라서 `io` link만 추가해서는
"어디서든 `io update`" 계약이 완성되지 않는다.

## 목표

- native install/update 뒤 어느 디렉터리에서든 `io <subcommand>`를 실행한다.
- `issueops`를 canonical binary와 public command identity로 유지한다.
- 기존 사용자 소유 `io` command를 덮어쓰지 않는다.
- `auto`, `manual`, `skip` path mode 모두 동일한 command shim 두 개를 관리한다.
- Codex/Claude host adapter와 무관한 공통 installer 경로에서 한 번만 구현한다.

## 비목표

- binary 이름이나 CLI 출력의 `issueops` identity를 `io`로 변경하지 않는다.
- `.zshrc`, `.bashrc`, `.profile`에 shell `alias io=...`를 추가하지 않는다.
- `io` 전용 wrapper script나 별도 binary를 만들지 않는다.
- 설치 검증을 위해 사용자의 실제 home integration을 자동 변경하지 않는다.

## 검토한 접근

### 1. Managed command symlink — 채택

`~/.local/bin/io`가 canonical command shim인
`~/.local/bin/issueops`를 가리키게 한다.

장점:

- interactive/non-interactive shell과 script에서 같은 방식으로 동작한다.
- 기존 `PATH` 설정을 그대로 재사용한다.
- checkout이 변경되어 canonical shim target이 바뀌어도 `io` target은 고정된다.
- install/update dry-run 결과의 `links`로 계획과 readback을 검증할 수 있다.

### 2. Shell alias — 기각

shell별 rc 수정이 필요하고 non-interactive process, hook, script에서 일관되게
동작하지 않는다. 기존 command shim과 별도 surface가 되어 drift를 만든다.

### 3. Wrapper script — 기각

`ISSUEOPS_ROOT` 설정이나 exec forwarding을 wrapper에 중복하게 된다. root 탐색
결함을 숨길 뿐 canonical binary 자체의 "어디서든 update" 동작을 고치지 않는다.

## 상세 설계

### Harness root 탐색

`pathutil.HarnessRoot`는 다음 순서로 marker를 찾는다.

1. 명시된 `ISSUEOPS_ROOT`
2. 현재 작업 디렉터리
3. `os.Executable()`이 반환한 경로
4. `filepath.EvalSymlinks`로 해석한 실제 executable 경로

원본 executable 경로와 resolved 경로가 같으면 중복하지 않는다.
`EvalSymlinks`가 실패하면 기존 원본 executable 탐색은 유지한다. 이 변경은
`issueops`와 `io` 어느 이름으로 실행하더라도 같은 checkout root를 찾게 한다.

### Command shim

installer path plan은 다음 순서로 link를 계획하거나 적용한다.

1. `~/.local/bin/issueops -> <checkout>/bin/issueops`
2. `~/.local/bin/io -> ~/.local/bin/issueops`

`--path-mode=manual`과 `--path-mode=skip`도 현재처럼 shell rc만 생략하고 두 command
shim은 모두 관리한다. 사용자 안내 문구는 두 command를 함께 설명한다.

### 충돌 정책

`io`의 짧고 일반적인 이름은 기존 command와 충돌할 가능성이 있으므로 canonical
shim보다 엄격하게 다룬다.

- `~/.local/bin/io`가 없으면 생성한다.
- 정확히 `~/.local/bin/issueops`를 가리키는 symlink면 no-op이다.
- regular file, directory, 또는 다른 target의 symlink면 덮어쓰지 않고 설치를
  실패시킨다.
- 오류에는 충돌 path와 수동 확인이 필요하다는 사실을 포함한다.

canonical `issueops` shim의 기존 refresh 동작은 변경하지 않는다.

### 문서 계약

- README와 install operations에 `io` convenience command를 추가한다.
- active ADR에는 canonical identity를 유지하면서 managed shorthand를 허용한 결정,
  shell alias/wrapper를 기각한 이유, 충돌 정책을 기록한다.
- `CONVENTIONS.md`의 symlink 규칙은 user skill link 외에도 installer-owned command
  shim 두 개가 허용된다는 현재 코드 계약으로 바로잡는다.
- installer help는 두 command shim과 PATH 동작을 설명한다.

## 오류 처리

- canonical shim 생성이 실패하면 기존처럼 install result를 실패로 반환한다.
- `io` 충돌도 install result `ok=false`와 non-zero exit로 전파한다.
- `--dry-run`도 실제 파일을 쓰지 않지만 같은 충돌을 감지하고 실패한다.
- root 탐색은 symlink 해석 실패만으로 중단하지 않고 기존 fallback을 유지한다.

## 테스트

TDD 순서로 다음 regression을 먼저 실패시킨다.

1. symlink로 실행된 executable path가 marker가 있는 실제 checkout root를 찾는다.
2. install dry-run의 `auto`, `manual`, `skip` 모두 canonical/short shim 두 개를
   보고한다.
3. 정확한 기존 `io` symlink는 no-op이다.
4. regular file과 unrelated symlink `io`는 보존되고 설치가 실패한다.
5. repo 밖 임시 디렉터리에서 symlink chain을 통해 실행한
   `io update --dry-run --path-mode=skip --json`이 checkout root를 사용한다.

구현 후 검증:

```bash
go test ./cmd/issueops/pathutil ./cmd/issueops/installcli ./cmd/issueops/updatecli -count=1
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/issueops ./cmd/issueops
./bin/issueops install --dry-run --path-mode=skip --json
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

마지막 smoke는 임시 home/bin과 repo 밖 cwd를 사용하며 사용자의 실제
`~/.local/bin`이나 host integration을 변경하지 않는다.

## 완료 기준

- install/update dry-run이 `issueops`와 `io` link를 모두 machine-readable하게
  보고한다.
- 기존 unrelated `io` command는 byte/path identity가 변하지 않는다.
- repo 밖에서 `io update --dry-run`이 현재 checkout의 installer를 찾는다.
- targeted/full/race/vet/build/self-verify가 마지막 단일 검증 파동에서 통과한다.
- 실제 사용자 home install/update는 별도 명시 실행 전까지 변경하지 않는다.

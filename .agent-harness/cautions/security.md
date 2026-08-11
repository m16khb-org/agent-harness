---
name: cautions/security.md
description: Cautions for command policy, secret handling, contributor identity, and publication Git config authority.
---

# Security, command policy, and trust-boundary cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for shell
execution, secret leakage, contributor identity, and publication Git config
authority.

## 3. 위험한 shell 실행

에이전트 하네스에서 shell runner는 가장 위험한 기능이다.

주의:
- argv 실행을 기본으로 하고 shell string 실행은 예외로 둔다.
- cwd, timeout, env, write/network 허용 여부를 명시한다.
- stdout/stderr는 redaction 후 저장/반환한다.
- workspace root 밖 파일 접근을 기본 거부한다. `cwd`뿐 아니라 path-like argv(`../`, `/abs/path`, `--flag=/abs/path`, `~/path`, symlink escape)도 경계 검사를 통과해야 한다.

## 4. Secret leakage

agent prompt, logs, MCP responses, test failures에 secret이 쉽게 섞일 수 있다.

주의:
- token/key/password-like pattern은 adapter 경계에서 마스킹한다.
- fixture secret은 실제 값을 쓰지 않는다.
- command echo와 verbose log를 기본 비활성화한다.

## 19. Verify git identity before contributor-sensitive pushes

GitHub contributor attribution follows commit author/committer email, not just the displayed author name.

주의:
- Before committing or pushing contributor-sensitive history, run `git config --show-origin --get-regexp '^user\.'` and `git var GIT_AUTHOR_IDENT && git var GIT_COMMITTER_IDENT`.
- In this repo, `m16khb@bubbletap.com` maps to the unwanted `habinkim-bubbletap` contributor. Use `m16khb@gmail.com` or `43867832+m16khb@users.noreply.github.com` instead.
- If a tool may bypass repo-local config, set `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_COMMITTER_NAME`, and `GIT_COMMITTER_EMAIL` explicitly for the commit command.
- After push, verify `git log --all --format='%an <%ae> %cn <%ce>' | rg 'bubbletap'` is empty and check GitHub contributors when attribution matters.

## publication Git config authority를 diagnostic buffer나 platform 암묵성에 맡기지 말 것

- current-user writable 또는 owner-controlled config는 sibling `O_EXCL` lock 없이 publication을 진행하지 않는다. parent가 non-writable이라는 사실만으로 immutable이라고 분류하면 transient rewrite-and-restore 공격을 놓친다.
- immutable fallback은 canonical regular file이고 file/path chain 전체가 root(uid 0) 소유이며 current UID가 어느 것도 소유하거나 쓸 수 없고 sibling lock 실패가 permission/read-only filesystem인 경우로 제한한다. 임의의 ordinary non-current UID 소유자는 transient rewrite-and-restore가 가능하므로 defensible system authority가 아니며 fail-closed한다. protected callback 전후에 file identity, content fingerprint, origin/rewrite inventory를 다시 확인한다.
- origin/include/URL rewrite inventory는 diagnostic 4096-byte 출력 helper를 재사용하지 않는다. 별도 bounded-complete read를 공유하고 상한 초과는 partial parse 없이 fail-closed한다.
- Git은 conditional include key를 canonical lowercase `includeif`로 출력한다. active empty include는 origin inventory에 자체 entry가 없으므로 directive inventory가 이 canonical form을 놓치면 sibling lock authority도 사라진다.
- include path의 `~/`, `~user/`, `%(prefix)/` interpolation을 부분 재구현하지 않는다. `git config --type=path`로 Git 자신의 canonical 확장 결과를 inventory로 봉인하고, 확장되지 않은 `~`/`%(prefix)/` residue는 fail-closed한다. unresolvable `~user`는 git 자체가 nonzero로 실패하므로 그대로 fail-closed 전파한다.
- 최초 부재 default XDG git config는 authority set에서 생략하지 않는다. 부재 parent chain을 transient로 생성해 sibling lock을 잡고, release 시 생성분만 역순 제거하며, post-operation 검증에서 해당 config가 여전히 absent인지 확인한다. 그 외 authority path의 missing parent는 계속 fail-closed다.
- Unix의 `Stat_t`, access, effective UID, errno 판정은 build-tagged helper 안에 둔다. metadata/access 계약을 지원하지 않는 platform은 immutable fallback을 추측하지 않고 fail-closed한다.
- implementation evidence는 valid `branch_prepare.base_sha`를 immutable diff base로 사용한다. SHA가 없거나 검증 불가능하면 moving base ref로 추정하지 않고 fail closed한다.

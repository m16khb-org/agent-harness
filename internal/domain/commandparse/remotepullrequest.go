package commandparse

import "strings"

// RemotePullRequestCreate는 `gh pr create` / `glab mr create` argv에서 타겟
// 브랜치 판정에 필요한 것만 뽑은 형태다. 원격 쓰기 전에 타겟이 준비된 부모
// 작업 브랜치와 맞는지 보기 위한 입력이며, 그 비교 자체는 호출자가 한다.
type RemotePullRequestCreate struct {
	// Provider는 github 또는 gitlab이다.
	Provider string
	// Kind는 pr 또는 mr이다.
	Kind string
	// BaseBranch는 --base / -B / --target-branch 로 지정된 값이다. 플래그가
	// 없으면 빈 문자열이며 HasBaseFlag가 false다.
	BaseBranch string
	// HasBaseFlag는 타겟 플래그가 argv에 나타났는지다. 값이 빈 문자열인
	// 경우와 플래그 자체가 없는 경우를 호출자가 구분할 수 있게 한다.
	HasBaseFlag bool
}

// ParseRemotePullRequestCreate는 argv가 PR/MR 생성 명령일 때만 참을 돌려준다.
//
// 생성이 아닌 하위 명령(view, list, merge 등)과 다른 CLI는 관심 밖이다.
// 판정을 좁게 유지하는 이유는 이 파서의 소비자가 거부 사유를 만들기 때문이다.
// 형태를 넓게 잡으면 타겟과 무관한 명령까지 막게 된다.
func ParseRemotePullRequestCreate(argv []string) (RemotePullRequestCreate, bool) {
	cli, kind, rest, ok := remotePullRequestHead(argv)
	if !ok {
		return RemotePullRequestCreate{}, false
	}

	parsed := RemotePullRequestCreate{Kind: kind}
	switch cli {
	case "gh":
		parsed.Provider = "github"
	case "glab":
		parsed.Provider = "gitlab"
	}

	for index := 0; index < len(rest); index++ {
		arg := rest[index]
		switch {
		case arg == "--base" || arg == "-B" || arg == "--target-branch":
			parsed.HasBaseFlag = true
			if index+1 < len(rest) {
				parsed.BaseBranch = strings.TrimSpace(rest[index+1])
				index++
			}
		case strings.HasPrefix(arg, "--base="):
			parsed.HasBaseFlag = true
			parsed.BaseBranch = strings.TrimSpace(strings.TrimPrefix(arg, "--base="))
		case strings.HasPrefix(arg, "--target-branch="):
			parsed.HasBaseFlag = true
			parsed.BaseBranch = strings.TrimSpace(strings.TrimPrefix(arg, "--target-branch="))
		}
	}
	return parsed, true
}

// remotePullRequestHead는 argv 앞부분이 지원하는 CLI의 PR/MR create인지 본다.
// gh는 pr, glab은 mr만 받는다. 서로의 하위 명령을 섞어 받으면 존재하지 않는
// 명령 형태를 인식하게 되어 판정이 느슨해진다.
func remotePullRequestHead(argv []string) (cli, kind string, rest []string, ok bool) {
	if len(argv) < 3 {
		return "", "", nil, false
	}
	cli = strings.TrimSpace(argv[0])
	kind = strings.TrimSpace(argv[1])
	action := strings.TrimSpace(argv[2])
	if action != "create" {
		return "", "", nil, false
	}
	switch {
	case cli == "gh" && kind == "pr":
	case cli == "glab" && kind == "mr":
	default:
		return "", "", nil, false
	}
	return cli, kind, argv[3:], true
}

package draftwiki

// repo root 정규화는 디렉터리 존재를 실제로 확인한다. 그 구현은
// composition root가 설치한다.
var NormalizeRepoRoot func(root string) (string, error)

// ResolveRepoFile은 repo 안의 파일 경로를 확정한다.
var ResolveRepoFile func(root, candidate string) (string, error)

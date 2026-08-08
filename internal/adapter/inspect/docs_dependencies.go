package inspect

// 문서 목록 조회는 composition root가 설치한다. inspect는 문서 탐색 구현을
// 소유하지 않는다.
var ListDocs func(root string) []string

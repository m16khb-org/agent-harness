package draftwiki

// 문서 heading 읽기는 composition root가 설치한다.
var ReadHeadings func(path string) (string, []string)

package hookcli

// lifecycle 문서를 건드릴 수 있는 도구인지 판정하는 규칙은 composition root가
// 설치한다.
var ToolUseMayMutateLifecycleFiles func(tool, command string) bool

package draftwiki

// harness state 접근은 composition root가 설치한다. 이 adapter는 state를 어디에
// 저장하고 어떻게 잠그는지 알지 않는다.
var (
	StateDir func() string
)

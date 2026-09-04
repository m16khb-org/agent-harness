package issueopscli

import "strings"

// repeatedFlag는 issueopscli 패키지의 표준 반복 가능 문자열 flag다. --flag가
// 등장할 때마다 값 하나를 append하고, String()은 usage/default 표시용으로 모인
// 값들을 ","로 잇는다.
type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

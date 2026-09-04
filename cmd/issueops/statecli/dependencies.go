package statecli

import (
	"encoding/json"
	"os"
	"time"

	statecontract "issueops/internal/contract/state"
)

// Dependencies는 state CLI가 필요로 하는 state 연산을 함수로 받는다.
//
// CLI는 inbound adapter이므로 flag 해석과 출력만 소유하고, 저장소 구현은 모른다.
// concrete outbound adapter의 조립은 composition root(`cmd/issueops/issueopsapp`)에만 둔다.
type Dependencies struct {
	Write    func(key, content string) (statecontract.StateResult, error)
	Read     func(key string) (statecontract.StateResult, error)
	List     func() (statecontract.StateListResult, error)
	Prune    func(maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error)
	Doctor   func() (statecontract.StateDoctorResult, error)
	Maintain func() (statecontract.StateMaintainResult, error)
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

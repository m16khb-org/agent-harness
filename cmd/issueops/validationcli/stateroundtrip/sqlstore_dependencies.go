package stateroundtrip

// StateDatabase는 이 package가 실제로 쓰는 저장소 연산만 선언한다. 구현을 고르는 것은
// composition root의 결정이고, 여기서는 필요한 만큼만 안다.
type StateDatabase interface {
	Put(bucket, id string, data []byte) error
}

// 저장소 열기와 존재하는 record 조회는 composition root가 설치한다.
var (
	OpenStateDatabase func(dir string) (StateDatabase, error)
)

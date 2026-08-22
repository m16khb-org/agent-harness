// Package channel은 세션 간 메시지 채널 capability의 DTO를 소유한다.
//
// Codex/Claude/Omo 세션이 같은 harness state를 공유한다는 사실 위에
// durable한 발신/수신 원시를 얹는다. CLI와 MCP가 같은 계약을 공유한다.
package channel

// SchemaVersion는 channel 응답 계약의 현재 버전이다.
const SchemaVersion = 1

// 기본 대기 상수.
const (
	// WaitPollInterval는 recv --wait가 저장소를 관찰하는 간격이다.
	WaitPollIntervalMS = 250
	// DefaultWaitTimeoutSeconds는 --wait의 기본 대기 한도다.
	DefaultWaitTimeoutSeconds = 300
)

// Message는 채널에 append된 메시지 하나다.
type Message struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Channel       string `json:"channel"`
	From          string `json:"from"`
	Body          string `json:"body"`
	CreatedAt     string `json:"created_at"`
}

// SendRequest는 channel send 요청이다.
type SendRequest struct {
	Channel string `json:"channel"`
	From    string `json:"from"`
	Body    string `json:"body"`
}

// SendResult는 channel send의 응답이다.
type SendResult struct {
	OK            bool    `json:"ok"`
	SchemaVersion int     `json:"schema_version"`
	Channel       string  `json:"channel"`
	Message       Message `json:"message"`
	Error         string  `json:"error,omitempty"`
}

// RecvRequest는 channel recv 요청이다. SinceID가 비어 있으면 채널의 처음부터,
// 아니면 해당 ID(제외) 이후의 메시지를 반환한다. Wait이 true면 TimeoutSeconds
// 안에 새 메시지가 올 때까지 대기한다.
type RecvRequest struct {
	Channel        string `json:"channel"`
	SinceID        string `json:"since_id,omitempty"`
	Wait           bool   `json:"wait"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Limit          int    `json:"limit,omitempty"`
}

// RecvResult는 channel recv의 응답이다.
type RecvResult struct {
	OK            bool      `json:"ok"`
	SchemaVersion int       `json:"schema_version"`
	Channel       string    `json:"channel"`
	Messages      []Message `json:"messages"`
	LastID        string    `json:"last_id,omitempty"`
	Waited        bool      `json:"waited"`
	TimedOut      bool      `json:"timed_out"`
	Error         string    `json:"error,omitempty"`
}

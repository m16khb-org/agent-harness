// Package channel은 세션 간 메시지 채널의 저장소 연산을 소유한다.
//
// looprun과 같은 조립 구조다: 저장소 열기·조회는 함수 변수로 주입받고
// composition root가 배선한다. 메시지는 append-only이며 채널별 필터는
// 읽기 시점에 한다. 로컬 개인 하네스의 규모를 가정하므로 전체 스캔으로
// 충분하다 — 인덱스 최적화는 실제 요구가 확인된 뒤에만 고려한다.
package channel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	channelcontract "issueops/internal/contract/channel"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const channelBucket = "channel_v1"

// ErrWaitTimeout은 recv --wait가 시간 안에 새 메시지를 못 본 결과다.
var ErrWaitTimeout = errors.New("wait timeout")

// StateDatabase는 이 package가 실제로 쓰는 저장소 연산만 선언한다.
type StateDatabase interface {
	Get(bucket, id string) ([]byte, bool, error)
	Put(bucket, id string, data []byte) error
}

// issueops state 접근과 저장소 열기는 composition root가 설치한다.
var (
	StateDir          func() string
	OpenStateDatabase func(dir string) (StateDatabase, error)
	GetExisting       func(dir, bucket, id string) ([]byte, bool, error)
	ListExisting      func(dir, bucket string) ([]string, error)
	// channelNow는 주입형 clock이다(테스트가 시간을 고정할 때 교체).
	channelNow  = time.Now
	channelWait = time.Sleep
)

func StateRoot() string {
	return filepath.Join(StateDir(), "channel")
}

// Send는 메시지를 채널에 append하고 저장된 메시지를 반환한다.
func Send(req channelcontract.SendRequest) (channelcontract.SendResult, error) {
	result := channelcontract.SendResult{SchemaVersion: channelcontract.SchemaVersion, Channel: strings.TrimSpace(req.Channel)}
	req.Channel = strings.TrimSpace(req.Channel)
	req.From = strings.TrimSpace(req.From)
	if req.Channel == "" {
		result.Error = "channel_required"
		return result, fmt.Errorf("channel_required")
	}
	if req.From == "" {
		result.Error = "from_required"
		return result, fmt.Errorf("from_required")
	}
	if strings.TrimSpace(req.Body) == "" {
		result.Error = "body_required"
		return result, fmt.Errorf("body_required")
	}
	db, err := OpenStateDatabase(StateRoot())
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	msg := channelcontract.Message{
		OK:            true,
		SchemaVersion: channelcontract.SchemaVersion,
		ID:            newMessageID(),
		Channel:       req.Channel,
		From:          req.From,
		Body:          req.Body,
		CreatedAt:     channelNow().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	if err := db.Put(channelBucket, msg.ID, data); err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.OK = true
	result.Message = msg
	return result, nil
}

// Recv는 채널의 메시지를 SinceID 이후로 반환한다. Wait이면 새 메시지가
// 생기거나 타임아웃까지 대기한다. 대기 후에도 비었으면 TimedOut을 세운다.
func Recv(req channelcontract.RecvRequest) (channelcontract.RecvResult, error) {
	result := channelcontract.RecvResult{SchemaVersion: channelcontract.SchemaVersion, Channel: strings.TrimSpace(req.Channel)}
	req.Channel = strings.TrimSpace(req.Channel)
	if req.Channel == "" {
		result.Error = "channel_required"
		return result, fmt.Errorf("channel_required")
	}
	timeout := channelcontract.DefaultWaitTimeoutSeconds
	if req.TimeoutSeconds > 0 {
		timeout = req.TimeoutSeconds
	}
	if req.Wait {
		result.Waited = true
		deadline := channelNow().Add(time.Duration(timeout) * time.Second)
		observed := map[string]struct{}{}
		for {
			messages, err := readChannelUnseen(req.Channel, req.SinceID, req.Limit, observed)
			if err != nil {
				result.Error = err.Error()
				return result, err
			}
			if len(messages) > 0 {
				fillRecvResult(&result, messages)
				return result, nil
			}
			if !channelNow().Before(deadline) {
				result.OK = true
				result.TimedOut = true
				return result, nil
			}
			remaining := deadline.Sub(channelNow())
			poll := time.Millisecond * time.Duration(channelcontract.WaitPollIntervalMS)
			if remaining < poll {
				poll = remaining
			}
			if poll > 0 {
				// Cross-process writers make in-process notification
				// non-authoritative; channelWait keeps polling testable.
				channelWait(poll)
			}
		}
	}
	messages, err := readChannel(req.Channel, req.SinceID, req.Limit)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	fillRecvResult(&result, messages)
	return result, nil
}

func fillRecvResult(result *channelcontract.RecvResult, messages []channelcontract.Message) {
	result.OK = true
	result.Messages = messages
	if len(messages) > 0 {
		result.LastID = messages[len(messages)-1].ID
	}
}

func readChannel(channelName, sinceID string, limit int) ([]channelcontract.Message, error) {
	return readChannelUnseen(channelName, sinceID, limit, nil)
}

func readChannelUnseen(channelName, sinceID string, limit int, observed map[string]struct{}) ([]channelcontract.Message, error) {
	ids, err := ListExisting(StateRoot(), channelBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []channelcontract.Message{}, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(ids) // 메시지 ID는 시간 기반이라 정렬이 곧 도착 순서다.
	// sinceID 위치를 먼저 찾는다. 가비지 컬렉션 등으로 사라졌다면 위치를
	// 알 수 없으므로 처음부터 반환한다 — “없음”과 “아직 도달 안 함”을 구분할
	// 근거가 없기 때문이다.
	startAt := 0
	if trimmed := strings.TrimSpace(sinceID); trimmed != "" {
		for i, id := range ids {
			if id == trimmed {
				startAt = i + 1
				break
			}
		}
	}
	messages := []channelcontract.Message{}
	for _, id := range ids[startAt:] {
		if _, alreadyObserved := observed[id]; alreadyObserved {
			continue
		}
		data, ok, err := GetExisting(StateRoot(), channelBucket, id)
		if err != nil || !ok {
			continue // 동시성 경합으로 사라진 레코드는 건너뛴다.
		}
		var msg channelcontract.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue // 깨진 레코드도 채널 스트림을 끊지 않는다.
		}
		if observed != nil {
			// Channel records are append-only: Send allocates a unique ID and
			// never rewrites it. A successfully decoded nonmatching record
			// cannot later become a message for this channel.
			observed[id] = struct{}{}
		}
		if msg.Channel != channelName {
			continue
		}
		if limit > 0 && len(messages) >= limit {
			break
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

// newMessageID는 시간 정렬 가능한 ID를 만든다: 나노초 hex + 난수.
// 같은 나노초에 만들어진 메시지는 난수로 구분되고, ListExisting의 키 정렬이
// 도착 순서를 보존한다.
func newMessageID() string {
	now := channelNow().UTC().UnixNano()
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		// 난수 실패는 처리를 계속한다 — ID 충돌 위험만 감수한다.
		for i := range nonce {
			nonce[i] = byte(now >> (i * 8))
		}
	}
	return fmt.Sprintf("msg-%016x-%s", now, hex.EncodeToString(nonce[:]))
}

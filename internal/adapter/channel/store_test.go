package channel

import (
	"agent-harness/internal/adapter/outbound/sqlstore"
	statestore "agent-harness/internal/adapter/outbound/state"
	channelcontract "agent-harness/internal/contract/channel"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// production wiring과 같은 저장소를 설치한다.
func init() {
	StateDir = statestore.StateDir
	GetExisting = sqlstore.GetExisting
	ListExisting = sqlstore.ListExisting
	OpenStateDatabase = func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }
}

func TestSendRecvRoundTripAcrossChannelIsolation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := Send(channelcontract.SendRequest{Channel: "contract", From: "server", Body: "GET /users -> 200 {id,name}"}); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	if _, err := Send(channelcontract.SendRequest{Channel: "contract", From: "front", Body: "need email too"}); err != nil {
		t.Fatalf("send 2: %v", err)
	}
	if _, err := Send(channelcontract.SendRequest{Channel: "other", From: "server", Body: "noise"}); err != nil {
		t.Fatalf("send 3: %v", err)
	}
	recv, err := Recv(channelcontract.RecvRequest{Channel: "contract"})
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if !recv.OK || len(recv.Messages) != 2 {
		t.Fatalf("recv messages = %d, want 2 (cross-channel isolation): %+v", len(recv.Messages), recv)
	}
	if recv.Messages[0].From != "server" || recv.Messages[0].Body != "GET /users -> 200 {id,name}" {
		t.Fatalf("first message wrong: %+v", recv.Messages[0])
	}
	if recv.LastID != recv.Messages[1].ID {
		t.Fatalf("LastID must be the newest message: %q vs %q", recv.LastID, recv.Messages[1].ID)
	}
	for _, msg := range recv.Messages {
		if !msg.OK || msg.SchemaVersion != channelcontract.SchemaVersion || msg.ID == "" || msg.CreatedAt == "" {
			t.Fatalf("message contract fields incomplete: %+v", msg)
		}
	}
}

func TestRecvSinceIDReturnsOnlyNewMessages(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	first, _ := Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: "1"})
	if _, err := Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: "2"}); err != nil {
		t.Fatal(err)
	}
	recv, err := Recv(channelcontract.RecvRequest{Channel: "c", SinceID: first.Message.ID})
	if err != nil {
		t.Fatalf("recv since: %v", err)
	}
	if len(recv.Messages) != 1 || recv.Messages[0].Body != "2" {
		t.Fatalf("since filter wrong: %+v", recv.Messages)
	}
	// 사라진 sinceID는 "처음부터"로 해석한다(위치를 알 근거가 없음).
	all, err := Recv(channelcontract.RecvRequest{Channel: "c", SinceID: "msg-does-not-exist"})
	if err != nil {
		t.Fatalf("recv missing-since: %v", err)
	}
	if len(all.Messages) != 2 {
		t.Fatalf("missing sinceID should return all: %+v", all.Messages)
	}
}

func TestRecvLimitBoundsResult(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, body := range []string{"1", "2", "3"} {
		if _, err := Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	recv, err := Recv(channelcontract.RecvRequest{Channel: "c", Limit: 2})
	if err != nil {
		t.Fatalf("recv limit: %v", err)
	}
	if len(recv.Messages) != 2 || recv.LastID != recv.Messages[1].ID {
		t.Fatalf("limit wrong: %+v", recv)
	}
}

func TestRecvWaitReturnsImmediatelyWhenMessageExists(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := Send(channelcontract.SendRequest{Channel: "c", From: "server", Body: "ready"}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	recv, err := Recv(channelcontract.RecvRequest{Channel: "c", Wait: true, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("recv wait: %v", err)
	}
	if len(recv.Messages) != 1 || recv.TimedOut {
		t.Fatalf("existing message must return without waiting: %+v", recv)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("wait with existing message took %v; should be immediate", elapsed)
	}
}

func TestRecvWaitTimesOutEmpty(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	recv, err := Recv(channelcontract.RecvRequest{Channel: "c", Wait: true, TimeoutSeconds: 1})
	if err != nil {
		t.Fatalf("recv wait timeout: %v", err)
	}
	if recv.OK != true || len(recv.Messages) != 0 || !recv.TimedOut {
		t.Fatalf("timeout result wrong: %+v", recv)
	}
}

func TestRecvWaitBlocksUntilConcurrentSend(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	arrived := make(chan channelcontract.RecvResult, 1)
	go func() {
		recv, err := Recv(channelcontract.RecvRequest{Channel: "contract", Wait: true, TimeoutSeconds: 15})
		if err != nil {
			t.Errorf("wait recv: %v", err)
		}
		arrived <- recv
	}()
	// 수신자가 폴링을 시작할 시간을 준 뒤 발신한다. 이 대기는 테스트의
	// 판정 대상이 아니라 송신 타이밍을 만드는 장치다.
	time.Sleep(600 * time.Millisecond)
	if _, err := Send(channelcontract.SendRequest{Channel: "contract", From: "server", Body: "contract v1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case recv := <-arrived:
		if len(recv.Messages) != 1 || recv.TimedOut || recv.Messages[0].Body != "contract v1" {
			t.Fatalf("waited recv wrong: %+v", recv)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("recv --wait did not observe the concurrent send")
	}
}

func TestSendValidation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	cases := []struct {
		name string
		req  channelcontract.SendRequest
	}{
		{"empty channel", channelcontract.SendRequest{From: "a", Body: "x"}},
		{"empty from", channelcontract.SendRequest{Channel: "c", Body: "x"}},
		{"empty body", channelcontract.SendRequest{Channel: "c", From: "a"}},
	}
	for _, tc := range cases {
		result, err := Send(tc.req)
		if err == nil || result.OK {
			t.Fatalf("%s must fail: %+v %v", tc.name, result, err)
		}
	}
	if _, err := Recv(channelcontract.RecvRequest{}); err == nil {
		t.Fatal("recv without channel must fail")
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

func TestMessageIDsSortChronologically(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	// 저장소에 직접 ID 순서를 강제하는 메시지를 써서 정렬 가정을 검증한다.
	ids := []string{"msg-000000000000000a-01", "msg-000000000000000b-01", "msg-000000000000000c-01"}
	db, err := OpenStateDatabase(StateRoot())
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		msg := channelcontract.Message{OK: true, SchemaVersion: channelcontract.SchemaVersion, ID: id, Channel: "sort", From: "a", Body: string(rune('0' + i)), CreatedAt: "2026-08-22T00:00:00Z"}
		data, _ := jsonMarshal(msg)
		if err := db.Put(channelBucket, id, data); err != nil {
			t.Fatal(err)
		}
	}
	_ = filepath.Join(dir)
	recv, err := Recv(channelcontract.RecvRequest{Channel: "sort"})
	if err != nil {
		t.Fatal(err)
	}
	if len(recv.Messages) != 3 || recv.Messages[0].ID != ids[0] || recv.Messages[2].ID != ids[2] {
		t.Fatalf("chronological order broken: %+v", recv.Messages)
	}
	_ = os.Setenv("HARNESS_STATE_DIR", dir)
}

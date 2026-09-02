package providerutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunBoundedReadbackRejectsOversizedOutputAndRedactsFailure(t *testing.T) {
	bin := t.TempDir()
	script := filepath.Join(bin, "readback")
	// 초과 출력은 문자열을 지수적으로 늘려 한 번에 쓴다. 바이트마다 printf를
	// 부르는 셸 루프는 같은 300KB를 만드는 데 ~1.6s가 들어, -race로 전체 패키지를
	// 병렬 실행할 때 providerReadbackTimeout(15s)을 넘겨 이 테스트를 flaky하게
	// 만들었다. 검증 대상은 limit 초과 거부지 명령의 실행 속도가 아니다.
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = large ]; then s=x; i=0; while [ $i -lt 5 ]; do s=\"$s$s$s$s$s$s$s$s$s$s\"; i=$((i+1)); done; printf '%s%s%s' \"$s\" \"$s\" \"$s\"; exit 0; fi\n" +
		"printf 'api_key=abcdefghijklmnopqrstuvwxyz123456\\n' >&2\n" +
		"exit 2\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := RunBoundedReadback(t.TempDir(), script, "large"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized readback error = %v", err)
	}
	if _, err := RunBoundedReadback(t.TempDir(), script, "secret"); err == nil || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") || len(err.Error()) > providerDiagnosticLimit+128 {
		t.Fatalf("secret readback diagnostic was not bounded/redacted: %v", err)
	}
}

func TestRunBoundedReadbackContextCancelsStartedCommand(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "started")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "slow")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf started > \"$1\"\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	go func() {
		data, _ := os.ReadFile(fifo)
		if string(data) == "started" {
			close(started)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := RunBoundedReadbackContext(ctx, dir, script, fifo)
		result <- err
	}()
	<-started
	cancel()

	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled provider command did not stop")
	}
}

func TestRunBoundedCommandReportsPostStartTimeoutWithoutRetryAuthority(t *testing.T) {
	script := filepath.Join(t.TempDir(), "slow")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexec sleep 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, invoked, err := runBoundedCommand(t.TempDir(), script, nil, 20*time.Millisecond, 1024)
	if err == nil || !invoked || time.Since(started) > time.Second {
		t.Fatalf("timeout classification invoked=%v elapsed=%s err=%v", invoked, time.Since(started), err)
	}
}

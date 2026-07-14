package hostprobe

import (
	"strings"
	"testing"
)

func TestObservedModelFromOutputReadsOnlyStructuredModelFields(t *testing.T) {
	output := []byte("{\"type\":\"system\",\"subtype\":\"init\",\"model\":\"claude-opus-4-8\"}\n{\"model\":\"later\"}\n")
	if got := observedModelFromOutput(output); got != "claude-opus-4-8" {
		t.Fatalf("observed model=%q", got)
	}
	if got := observedModelFromOutput([]byte(`{"text":"model=secret"}`)); got != "" {
		t.Fatalf("freeform text produced model=%q", got)
	}
}

func TestBoundedBufferReportsTruncationWithoutShortWrite(t *testing.T) {
	buffer := &boundedBuffer{limit: 4}
	value := []byte("123456")
	written, err := buffer.Write(value)
	if err != nil || written != len(value) || buffer.String() != "1234" || !buffer.truncated {
		t.Fatalf("written=%d err=%v buffer=%q truncated=%t", written, err, buffer.String(), buffer.truncated)
	}
	if strings.Contains(buffer.String(), "56") {
		t.Fatal("bounded buffer retained truncated suffix")
	}
}

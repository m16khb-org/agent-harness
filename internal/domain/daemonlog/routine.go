// Package daemonlog는 데몬 진단 로그의 루틴 이벤트 판정을 소유한다.
// mcpcli(로그 필터)와 daemoncli(serve 루프)가 같은 기준을 공유한다.
package daemonlog

import (
	"context"
	"log/slog"
	"strings"
)

// RoutineSessionEvents는 데몬 logFile에서 DEBUG로 강등할 go-sdk 세션
// 수명 주기 이벤트다. 실측 2026-08-21: 27만 줄 로그의 99.9%가 이 이벤트
// 들이었다(연결당 반복되는 volume, 운영 신호 아님).
var RoutineSessionEvents = map[string]bool{
	"server run start":            true,
	"server connecting":           true,
	"server session connected":    true,
	"session initialized":         true,
	"server session disconnected": true,
}

// IsRoutineShutdownError는 데몬 accept/close 경로에서 정상 종료로 처리하는
// 네트워크 에러를 판별한다(accept 루프의 기존 기준과 동일한 텍스트 매칭에
// 프록시 클라이언트 정상 종료 \"server is closing\"을 더한다).
func IsRoutineShutdownError(err error) bool {
	if err == nil {
		return false
	}
	return isRoutineShutdownErrorText(err.Error())
}

func isRoutineShutdownErrorText(text string) bool {
	return strings.Contains(text, "use of closed network connection") ||
		strings.Contains(text, "server is closing")
}

// FilteringHandler는 RoutineSessionEvents에 해당하는 INFO 레코드를
// DEBUG로 강등하는 slog.Handler 래퍼다.
type FilteringHandler struct {
	inner slog.Handler
}

// NewFilteringHandler은 inner 위에 루틴 세션 이벤트 강등 필터를 얹는다.
func NewFilteringHandler(inner slog.Handler) slog.Handler {
	return FilteringHandler{inner: inner}
}

func (h FilteringHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h FilteringHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level == slog.LevelInfo && RoutineSessionEvents[record.Message] {
		record.Level = slog.LevelDebug
	}
	// go-sdk는 세션 종료 에러를 무조건 ERROR로 기록한다. 그 에러가 정상
	// shutdown 경로("server is closing: EOF" 등)면 실제 결함이 아니므로
	// 강등한다. 진짜 세션 장애는 여전히 ERROR로 남는다.
	if record.Level == slog.LevelError && record.Message == "server session ended with error" {
		var errText string
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Key == "error" {
				errText = attr.Value.String()
			}
			return true
		})
		if isRoutineShutdownErrorText(errText) {
			record.Level = slog.LevelDebug
		}
	}
	return h.inner.Handle(ctx, record)
}

func (h FilteringHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return FilteringHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h FilteringHandler) WithGroup(name string) slog.Handler {
	return FilteringHandler{inner: h.inner.WithGroup(name)}
}

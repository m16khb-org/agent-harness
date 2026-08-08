// Package install는 install capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package install

type NativeRuntimeDiagnostic struct {
	Stale           bool   `json:"stale"`
	Observed        string `json:"observed,omitempty"`
	Expected        string `json:"expected,omitempty"`
	RestartRequired bool   `json:"restart_required"`
}

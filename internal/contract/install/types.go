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
	// GenerationSkew는 경로는 같은데 **빌드 세대가 다른** 상태다. 파일이
	// 교체돼도 실행 중인 프로세스는 교체 이전 세대를 계속 쓰므로, 경로 비교만으로는
	// 잡히지 않는다. 그 상태에서 새 typed command를 쓰면 이전 세대 hook이
	// 그것을 모르고 차단해 복구가 교착된다(#328).
	GenerationSkew bool `json:"generation_skew,omitempty"`
	// ObservedGeneration과 ExpectedGeneration은 진단에 남길 짧은 표기다.
	// 관측하지 못하면 "unknown"이며, 그 경우 skew로 판정하지 않는다.
	ObservedGeneration string `json:"observed_generation,omitempty"`
	ExpectedGeneration string `json:"expected_generation,omitempty"`
}

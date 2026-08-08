package commandparse

import "testing"

// TestGofmtListingIsReadOnly는 #319 dogfood가 드러낸 결함을 고정한다.
//
// 저장소가 문서화한 검증 명령은 `gofmt -l internal cmd`인데, 예전에는
// `gofmt -d <파일.go>` 하나만 인정했다. 그래서 active lease를 든 owner가
// **자기 저장소의 검증을 실행할 수 없었다** — 실측: PR #429의 owner가
// gofmt 축을 UNVERIFIED로 보고하며 "lifecycle guard가 gofmt를 정적으로
// 분류할 수 없어 unsafe_mutation으로 거부한다"고 적었다.
//
// `-l`은 목록만 출력하고 `-d`는 diff만 출력한다. 둘 다 파일을 건드리지
// 않으므로 read-only다. 디렉터리 피연산자도 마찬가지다.
func TestGofmtListingIsReadOnly(t *testing.T) {
	for _, command := range []string{
		"gofmt -l internal cmd",
		"gofmt -l internal/adapter/orca/client.go",
		"gofmt -d internal/adapter/orca/client.go",
		"gofmt -l -d internal cmd",
		"gofmt -s -l internal",
	} {
		if !ExactReadOnlyShellCommand(command) {
			t.Fatalf("read-only gofmt 형태여야 한다: %q", command)
		}
	}
}

// TestGofmtWritingFormsStayBlocked는 완화가 쓰기 표면까지 열지 않음을
// 고정한다. `-w`는 파일을 덮어쓰고 `-r`는 코드를 재작성한다.
func TestGofmtWritingFormsStayBlocked(t *testing.T) {
	for _, command := range []string{
		"gofmt -w internal",
		"gofmt -l -w internal",
		"gofmt -r 'a[b:len(a)] -> a[b:]' -l internal",
		"gofmt",
		"gofmt -l",
		"gofmt -l /etc",
		"gofmt -l ../outside",
		"gofmt -l -",
		"gofmt -cpuprofile out.prof -l internal",
		"gofmt -d README.md",
		"gofmt -l docs/guide.md",
	} {
		if ExactReadOnlyShellCommand(command) {
			t.Fatalf("허용하면 안 되는 형태다: %q", command)
		}
	}
}

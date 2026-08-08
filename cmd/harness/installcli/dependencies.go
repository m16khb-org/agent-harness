package installcli

import (
	"encoding/json"
	"os"

	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

// Deps holds host-provided dependencies for the install CLI. The composition
// root injects implementations via Configure; defaults support standalone
// use/tests.
type Deps struct {
	HarnessRoot       func() string
	ActivationBackend activationport.Backend

	// ExecutablePath는 현재 실행 파일 경로를 돌려준다. managed command file을
	// 안전하게 채택하려면 후보가 자기 자신인지 가려야 한다.
	ExecutablePath func() (string, error)

	// NativeInstallRequest와 InstallNative는 composition root가 주입한다.
	// 어떤 host installer를 조립할지는 CLI가 알 필요가 없다.
	NativeInstallRequest func(root, home, codexHome, binPath string) port.NativeInstallRequest
	InstallNative        func(port.NativeInstallRequest) (port.NativeInstallResult, error)

	// ActivationReadback은 host별 활성화 증적을 모으는 verifier를 만든다.
	// 어떤 host adapter를 조립할지는 composition root가 정한다.
	ActivationReadback func(port.NativeInstallRequest) activationport.ReadbackVerifier
}

var deps = defaultDeps()

// Configure installs host-provided dependencies (called once by the composition
// root); Reset restores defaults for tests via t.Cleanup.
func Configure(d Deps) {
	if d.ExecutablePath == nil {
		d.ExecutablePath = os.Executable
	}
	deps = d
}

// Reset restores standalone defaults.
func Reset() { deps = defaultDeps() }

func defaultDeps() Deps {
	return Deps{HarnessRoot: defaultHarnessRoot, ExecutablePath: os.Executable}
}

func defaultHarnessRoot() string {
	if root := os.Getenv("HARNESS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

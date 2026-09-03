package harnessapp

import (
	"context"
	"fmt"

	"agent-harness/cmd/harness/installcli"
	agyadapter "agent-harness/internal/adapter/agy"
	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/adapter/install"
	"agent-harness/internal/adapter/installutil"
	omoadapter "agent-harness/internal/adapter/omo"
	mcpadapter "agent-harness/internal/domain/mcp"
	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

// installDependencies는 native install CLI에 host installer를 조립해 넘긴다.
//
// 어떤 host를 설치하고 어떤 증적을 읽을지는 composition root의 결정이다.
// CLI는 flag 해석과 출력만 소유한다.
func installDependencies() installcli.Deps {
	return installcli.Deps{
		HarnessRoot:          harnessRoot,
		ActivationBackend:    nativeActivationBackend(),
		NativeInstallRequest: install.DefaultNativeInstallRequest,
		InstallNative: func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
			return install.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller(), omoadapter.NewInstaller(), agyadapter.NewInstaller())
		},
		ActivationReadback: func(req port.NativeInstallRequest) activationport.ReadbackVerifier {
			return hostActivationReadback{request: req}
		},
		SyncUpstream: syncUpstream,
	}
}

type hostActivationReadback struct{ request port.NativeInstallRequest }

func (readback hostActivationReadback) Verify(_ context.Context, harnessRoot, targetBinary string) (activationport.Readback, error) {
	if readback.request.Root != harnessRoot || readback.request.BinPath != targetBinary {
		return activationport.Readback{}, fmt.Errorf("native activation readback target changed")
	}
	codexEvidence, err := codexadapter.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	claudeEvidence, err := claudeadapter.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	omoEvidence, err := omoadapter.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	agyEvidence, err := agyadapter.VerifyActivation(readback.request)
	if err != nil {
		return activationport.Readback{}, err
	}
	tools := mcpadapter.IssueOpsBasicTools()
	if len(tools) != 1 || tools[0].Name != "issueops_execution" {
		return activationport.Readback{}, fmt.Errorf("IssueOps v1 MCP activation catalog must contain exactly issueops_execution")
	}
	catalogSHA, err := installutil.SemanticSHA256(tools)
	if err != nil {
		return activationport.Readback{}, err
	}
	evidence := append(codexEvidence, claudeEvidence...)
	evidence = append(evidence, omoEvidence...)
	evidence = append(evidence, agyEvidence...)
	result := make([]activationport.Evidence, 0, len(evidence))
	for _, item := range evidence {
		result = append(result, activationport.Evidence{
			Host: item.Host, Surface: item.Surface, Path: item.Path, SemanticSHA256: item.SemanticSHA256,
			SHA256: item.SHA256, Mode: item.Mode, Size: item.Size, Device: item.Device, Inode: item.Inode,
		})
	}
	return activationport.Readback{CatalogSHA256: catalogSHA, Evidence: result}, nil
}

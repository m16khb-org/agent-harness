package upstream

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudePluginHostInstallPluginUsesSupportedArguments(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "claude")
	script := `#!/bin/sh
actual="$(printf '%s\n' "$@")"
expected='plugin
install
eli5@claude-community
--scope
user'
if [ "$actual" != "$expected" ]; then
	printf 'unexpected args:\n%s\n' "$actual" >&2
	exit 64
fi
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	host := ClaudePluginHost{Binary: binary}
	if err := host.InstallPlugin(context.Background(), "eli5@claude-community"); err != nil {
		t.Fatal(err)
	}
}

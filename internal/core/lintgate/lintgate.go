// Package lintgate provides a DETERMINISTIC, fail-open lint check for the
// PostToolUse hook (B3). It deliberately has NO dependency on host-agent
// provider settings, so the per-edit critical-path hook can never invoke a
// networked judgement path.
package lintgate

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// lintGateTimeout bounds the per-edit check so a slow/hanging formatter can
// never stall the agent's tool loop (PostToolUse runs under a ~5s host budget).
const lintGateTimeout = 2 * time.Second

// LintEditedGoFiles runs `gofmt -l` on the edited .go files and reports whether
// any are unformatted plus a concise feedback string naming them.
//
// It is DETERMINISTIC (gofmt -l is list-only: no compile, no package load) and
// strictly FAIL-OPEN: an absent toolchain, exec/start error, non-format failure,
// or timeout all yield (false, "") so a successful edit is never turned into a
// hook failure or a block. The error is intentionally absent from the signature
// so the single caller cannot accidentally surface it.
func LintEditedGoFiles(repo string, paths []string) (failed bool, feedback string) {
	goFiles := make([]string, 0, len(paths))
	for _, p := range paths {
		if strings.HasSuffix(strings.TrimSpace(p), ".go") {
			goFiles = append(goFiles, p)
		}
	}
	if len(goFiles) == 0 {
		return false, ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), lintGateTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gofmt", append([]string{"-l"}, goFiles...)...)
	if root := strings.TrimSpace(repo); root != "" {
		cmd.Dir = root
	}
	out, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		// fail-open: no Go toolchain, exec start error, or timeout.
		return false, ""
	}

	var names []string
	for line := range strings.FieldsSeq(strings.TrimSpace(string(out))) {
		names = append(names, filepath.Base(line))
	}
	if len(names) == 0 {
		return false, ""
	}
	return true, "gofmt: unformatted Go file(s) after edit (run `gofmt -w`): " + strings.Join(names, ", ")
}

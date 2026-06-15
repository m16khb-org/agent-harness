package codex

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/port"
)

func patchCodexPluginHookCompatibility(req port.NativeInstallRequest) ([]port.InstallFile, []string, error) {
	patches := []hookCompatibilityPatch{
		{
			Kind: "codex_plugin_hook_compat_llm_wiki",
			Globs: []string{
				filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki-marketplace", "llm-wiki", "*", "hooks", "llm-wiki-hook.cjs"),
				filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki", "llm-wiki", "*", "hooks", "llm-wiki-hook.cjs"),
				filepath.Join(req.CodexHome, "plugins", "cache", "llm-wiki", "wiki", "*", "hooks", "llm_wiki_session.py"),
				filepath.Join(req.CodexHome, ".tmp", "marketplaces", "llm-wiki", "plugins", "llm-wiki", "hooks", "llm_wiki_session.py"),
			},
			Replacements: append([]textReplacement{
				{Old: "    },\n    suppressOutput: true\n  }));", New: "    }\n  }));"},
			}, llmWikiSessionHookReplacements()...),
		},
		{
			Kind: "codex_plugin_hook_compat_claude_mem",
			Globs: []string{
				filepath.Join(req.CodexHome, "plugins", "cache", "*", "claude-mem", "*", "hooks", "codex-hooks.json"),
				filepath.Join(req.CodexHome, "plugins", "cache", "*", "claude-mem", "*", "scripts", "worker-service.cjs"),
				filepath.Join(req.CodexHome, "plugins", "cache", "*", "claude-mem", "*", "scripts", "worker-cli.js"),
			},
			Replacements: []textReplacement{
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" start`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" start || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex context`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex context || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex session-init`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex session-init || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex file-context`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex file-context || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex observation`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex observation || true`},
				{Old: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex summarize`, New: `node \"$_P/scripts/bun-runner.js\" \"$_P/scripts/worker-service.cjs\" hook codex summarize || true`},
				{Old: "function fZ(t,e){return{continue:!0,suppressOutput:!0,status:t,...e&&{message:e}}}", New: "function fZ(t,e){return{continue:!0,...e&&{systemMessage:e}}}"},
				{Old: "{continue:!0,suppressOutput:!0}", New: "{continue:!0}"},
				{Old: ",suppressOutput:!0", New: ""},
				{Old: "suppressOutput:!0,", New: ""},
				{Old: `O='{"continue": true, "suppressOutput": true}'`, New: `O='{"continue": true}'`},
				{Old: `O='{"continue":true,"suppressOutput":true}'`, New: `O='{"continue":true}'`},
			},
		},
	}
	var files []port.InstallFile
	var messages []string
	var errs []error
	for _, patch := range patches {
		paths := expandPatchGlobs(patch.Globs)
		if len(paths) == 0 {
			continue
		}
		for _, path := range paths {
			file, changed, err := applyHookCompatibilityPatch(path, patch.Kind, patch.Replacements, req.DryRun)
			if file.Path != "" {
				files = append(files, file)
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if changed {
				if req.DryRun {
					messages = append(messages, "dry-run: would patch Codex plugin hook compatibility: "+path)
				} else {
					messages = append(messages, "patched Codex plugin hook compatibility: "+path)
				}
			}
		}
	}
	return files, messages, joinErrors(errs)
}

func llmWikiSessionHookReplacements() []textReplacement {
	return []textReplacement{
		{Old: "import sys\nimport textwrap", New: "import sys\nimport tempfile\nimport textwrap"},
		{Old: "import sys\nfrom pathlib import Path", New: "import sys\nimport tempfile\nfrom pathlib import Path"},
		{
			Old: `def atomic_write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f".{path.name}.tmp")
    tmp.write_text(text, encoding="utf-8")
    tmp.replace(path)`,
			New: `def atomic_write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path: Path | None = None
    try:
        with tempfile.NamedTemporaryFile(
            "w",
            encoding="utf-8",
            dir=path.parent,
            prefix=f".{path.name}.",
            suffix=".tmp",
            delete=False,
        ) as handle:
            tmp_path = Path(handle.name)
            handle.write(text)
        tmp_path.replace(path)
    finally:
        if tmp_path is not None:
            try:
                tmp_path.unlink(missing_ok=True)
            except OSError:
                pass`,
		},
		{
			Old: `    except BrokenPipeError:
        return 1`,
			SkipIfContains: `except Exception as exc:`,
			New: `    except BrokenPipeError:
        return 1
    except Exception as exc:
        if getattr(args, "command", None) == "hook":
            if os.environ.get("LLM_WIKI_HOOK_DEBUG"):
                message = redact_scalar(str(exc))
                print(f"llm-wiki hook skipped after {exc.__class__.__name__}: {message}", file=sys.stderr)
            return 0
        raise`,
		},
	}
}

type hookCompatibilityPatch struct {
	Kind         string
	Globs        []string
	Replacements []textReplacement
}

type textReplacement struct {
	Old            string
	New            string
	SkipIfContains string
}

func expandPatchGlobs(globs []string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, pattern := range globs {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func applyHookCompatibilityPatch(path, kind string, replacements []textReplacement, dryRun bool) (port.InstallFile, bool, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	b, err := os.ReadFile(path)
	if err != nil {
		return file, false, err
	}
	text := string(b)
	next := text
	for _, replacement := range replacements {
		if replacement.SkipIfContains != "" && strings.Contains(next, replacement.SkipIfContains) {
			continue
		}
		next = strings.ReplaceAll(next, replacement.Old, replacement.New)
	}
	next = collapseDuplicateShellTrueFallbacks(next)
	if next == text {
		return file, false, nil
	}
	if dryRun {
		file.WouldWrite = true
		return file, true, nil
	}
	backup := path + ".harness.bak"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		if err := os.WriteFile(backup, b, 0o600); err != nil {
			return file, false, err
		}
	} else if err != nil {
		return file, false, err
	}
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		return file, false, err
	}
	file.Written = true
	return file, true, nil
}

func collapseDuplicateShellTrueFallbacks(text string) string {
	for strings.Contains(text, " || true || true") {
		text = strings.ReplaceAll(text, " || true || true", " || true")
	}
	return text
}

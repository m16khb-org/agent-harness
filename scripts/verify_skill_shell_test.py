#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import io
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path


SCRIPT = Path(__file__).with_name("verify-skill-shell.py")
SPEC = importlib.util.spec_from_file_location("verify_skill_shell", SCRIPT)
assert SPEC is not None
verify_skill_shell = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(verify_skill_shell)


class VerifySkillShellTest(unittest.TestCase):
    def write_skill(self, body: str) -> tuple[Path, Path]:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name) / "sample-skill"
        root.mkdir()
        path = root / "SKILL.md"
        path.write_text(body, encoding="utf-8")
        return root, path

    def test_checks_syntax_without_executing_snippet(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            "touch should-not-exist\n"
            'printf "%s\\n" "ok"\n'
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(violations, [])
        self.assertFalse((root / "should-not-exist").exists())

    def test_rejects_failure_swallowing_and_fabricated_defaults(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            "measure | grep result || true\n"
            "COUNT=$(grep -c result input || echo 0)\n"
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(
            [violation.code for violation in violations],
            ["failure-swallow", "fabricated-default"],
        )

    def test_rejects_unquoted_bisect_command_expansion(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            'TEST_CMD="go test ./..."\n'
            "git bisect run $TEST_CMD\n"
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual([violation.code for violation in violations], ["command-expansion"])

    def test_destructive_example_requires_recovery_annotation(self) -> None:
        root, path = self.write_skill(
            "```bash\n"
            "git branch -D backup/example\n"
            "```\n"
        )
        violations = verify_skill_shell.verify_paths([root])
        self.assertEqual([violation.code for violation in violations], ["destructive-unannotated"])

        path.write_text(
            '<!-- skill-shell: destructive recovery="verified backup branch remains reachable" -->\n'
            "```bash\n"
            "git branch -D backup/example\n"
            "```\n",
            encoding="utf-8",
        )
        self.assertEqual(verify_skill_shell.verify_paths([root]), [])

    def test_reports_invalid_shell_syntax(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            "if true; then\n"
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual([violation.code for violation in violations], ["syntax"])

    def test_documented_placeholders_do_not_hide_structural_syntax(self) -> None:
        root, _ = self.write_skill(
            '<!-- skill-shell: destructive recovery="verified backup branch remains reachable" -->\n'
            "```bash\n"
            "git rebase -i <base-branch>\n"
            "git show :2:<file>\n"
            "```\n"
        )

        self.assertEqual(verify_skill_shell.verify_paths([root]), [])

    def test_cli_rejects_missing_input_path(self) -> None:
        missing = Path(tempfile.gettempdir()) / "verify-skill-shell-does-not-exist"

        self.assertEqual(verify_skill_shell.main([str(missing)]), 2)

    def test_cli_help_is_explicit_and_successful(self) -> None:
        output = io.StringIO()

        with redirect_stdout(output):
            result = verify_skill_shell.main(["--help"])

        self.assertEqual(result, 0)
        self.assertIn("usage: verify-skill-shell.py", output.getvalue())

    def test_skip_annotation_does_not_bypass_destructive_policy(self) -> None:
        root, _ = self.write_skill(
            '<!-- skill-shell: skip reason="syntax is intentionally illustrative" -->\n'
            "```bash\n"
            "git reset --hard HEAD~1\n"
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual([violation.code for violation in violations], ["destructive-unannotated"])

    def test_rejects_broad_destructive_and_dynamic_shell_forms(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            "git clean -xdf\n"
            "git rebase main\n"
            "git bisect start\n"
            'eval "$COMMAND"\n'
            'bash -c "$COMMAND"\n'
            'source "$SCRIPT"\n'
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(
            [violation.code for violation in violations],
            [
                "destructive-unannotated",
                "destructive-unannotated",
                "destructive-unannotated",
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
            ],
        )

    def test_rejects_colon_zero_fallback_variable_loop_and_weak_recovery(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            "COUNT=$(measure || :)\n"
            'printf "%s\\n" "${COUNT:-0}"\n'
            "for file in $FILES; do printf '%s\\n' \"$file\"; done\n"
            "```\n"
            '<!-- skill-shell: destructive recovery="recover later" -->\n'
            "```bash\n"
            "git reset --hard HEAD~1\n"
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(
            [violation.code for violation in violations],
            [
                "failure-swallow",
                "fabricated-default",
                "word-splitting-loop",
                "destructive-recovery-weak",
            ],
        )

    def test_rejects_multiline_quoted_global_option_and_subshell_bypasses(self) -> None:
        root, _ = self.write_skill(
            "```bash (verification)\n"
            "git \\\n"
            "  reset --hard HEAD~1\n"
            'git "reset" --hard HEAD~1\n'
            "git -C /tmp/repo reset --hard HEAD~1\n"
            "git push --force origin main\n"
            "rm -rf \"$HOME/.cache/demo\"\n"
            'value=$(bash -c "$COMMAND")\n'
            'other=$(eval "$COMMAND")\n'
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(
            [violation.code for violation in violations],
            [
                "destructive-unannotated",
                "destructive-unannotated",
                "destructive-unannotated",
                "destructive-unannotated",
                "destructive-unannotated",
                "dynamic-shell",
                "dynamic-shell",
            ],
        )

    def test_scans_console_fences_for_destructive_commands(self) -> None:
        root, _ = self.write_skill(
            "```console\n"
            "git reset --hard HEAD~1\n"
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual([violation.code for violation in violations], ["destructive-unannotated"])

    def test_rejects_symlinked_reference_trees(self) -> None:
        root, _ = self.write_skill("# safe\n")
        outside = root.parent / "outside"
        outside.mkdir()
        (outside / "unsafe.md").write_text(
            "```bash\n"
            "git reset --hard HEAD~1\n"
            "```\n",
            encoding="utf-8",
        )
        (root / "references").symlink_to(outside, target_is_directory=True)

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual([violation.code for violation in violations], ["symlink-not-allowed"])

    def test_rejects_remaining_dynamic_and_split_option_bypasses(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            'git "re\\\nset" --hard HEAD~1\n'
            '"$GIT" reset --hard HEAD~1\n'
            "rm -r -f \"$HOME/.cache/demo\"\n"
            '"ba\\\nsh" -c "echo unsafe"\n'
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(
            [violation.code for violation in violations],
            [
                "destructive-unannotated",
                "dynamic-shell",
                "destructive-unannotated",
                "dynamic-shell",
            ],
        )

    def test_rejects_path_qualified_and_alternate_shell_launchers(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            '/bin/bash -c "$COMMAND"\n'
            '/bin/sh -c "$COMMAND"\n'
            'dash -c "$COMMAND"\n'
            '/usr/bin/fish -c "$COMMAND"\n'
            '/bin/bash -lc "$COMMAND"\n'
            '/bin/sh -ec "$COMMAND"\n'
            'dash -ec "$COMMAND"\n'
            '/usr/bin/fish --command "$COMMAND"\n'
            "```\n"
        )

        violations = verify_skill_shell.verify_paths([root])

        self.assertEqual(
            [violation.code for violation in violations],
            [
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
                "dynamic-shell",
            ],
        )

    def test_multiline_quoted_payload_is_one_non_command_token(self) -> None:
        root, _ = self.write_skill(
            "```bash\n"
            "gh api graphql -f query='\n"
            "  mutation($issueId: ID!) {\n"
            "    node(id: $issueId) { id }\n"
            "  }'\n"
            "```\n"
        )

        self.assertEqual(verify_skill_shell.verify_paths([root]), [])

if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3

from __future__ import annotations

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
SHANNON = ROOT / "skills" / "shannon" / "SKILL.md"
TORVALDS = ROOT / "skills" / "torvalds" / "SKILL.md"
BISECT = ROOT / "skills" / "torvalds" / "references" / "bisect-protocol.md"


class P0SkillSafetyTest(unittest.TestCase):
    def test_shannon_shell_measurement_is_alias_safe_and_total(self) -> None:
        content = SHANNON.read_text(encoding="utf-8")

        self.assertIn("## Safe Shell Measurement Contract", content)
        self.assertIn("measure_added_lines()", content)
        self.assertIn("LC_ALL=C awk", content)
        self.assertIn("empty diff", content)
        self.assertIn("no-match", content)
        self.assertIn("space-containing path", content)
        self.assertIn("ugrep alias", content)
        self.assertNotIn("|| true", content)
        self.assertNotIn("for f in $(git diff", content)

    def test_torvalds_bisect_uses_an_argv_safe_script_boundary(self) -> None:
        content = BISECT.read_text(encoding="utf-8")

        self.assertIn("BISECT_SCRIPT", content)
        self.assertIn('git bisect run "$BISECT_SCRIPT"', content)
        self.assertNotIn("git bisect run $TEST_CMD", content)
        self.assertIn("script boundary", content)

    def test_torvalds_clean_requires_preview_stash_and_confirmation(self) -> None:
        content = TORVALDS.read_text(encoding="utf-8")

        self.assertIn("### 7. Clean Safety Protocol", content)
        self.assertIn("git clean -nd", content)
        self.assertIn("git clean -ndx", content)
        self.assertIn("git stash push -u", content)
        self.assertIn("explicit user confirmation", content)
        self.assertIn("git clean -fd -- <approved-pathspec>", content)


if __name__ == "__main__":
    unittest.main()

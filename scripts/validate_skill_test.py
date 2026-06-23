#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("validate-skill.py")
SPEC = importlib.util.spec_from_file_location("validate_skill", SCRIPT)
assert SPEC is not None
validate_skill = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(validate_skill)


class ValidateSkillTest(unittest.TestCase):
    def write_skill(self, body: str) -> Path:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name) / "sample-skill"
        root.mkdir()
        (root / "SKILL.md").write_text(body, encoding="utf-8")
        return root

    def test_validates_quoted_description_without_pyyaml(self) -> None:
        skill = self.write_skill(
            '---\n'
            'name: sample-skill\n'
            'description: "Validate a description with: colon and \\"quotes\\"."\n'
            '---\n'
        )

        ok, message = validate_skill.validate_skill(skill)

        self.assertTrue(ok)
        self.assertEqual(message, "Skill is valid!")

    def test_rejects_unexpected_key(self) -> None:
        skill = self.write_skill(
            "---\n"
            "name: sample-skill\n"
            "description: sample\n"
            "unexpected: value\n"
            "---\n"
        )

        ok, message = validate_skill.validate_skill(skill)

        self.assertFalse(ok)
        self.assertIn("Unexpected key", message)

    def test_rejects_bad_name(self) -> None:
        skill = self.write_skill(
            "---\n"
            "name: Bad_Name\n"
            "description: sample\n"
            "---\n"
        )

        ok, message = validate_skill.validate_skill(skill)

        self.assertFalse(ok)
        self.assertIn("hyphen-case", message)


if __name__ == "__main__":
    unittest.main()

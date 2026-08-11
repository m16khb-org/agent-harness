from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from typing import cast

SKILL_ROOT = Path(__file__).parents[1]


class DocumentationCheckTest(unittest.TestCase):
    def test_valid_document_family_passes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=False)

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = self._payload(result)
            self.assertTrue(payload["ok"])
            self.assertEqual(payload["violations"], [])

    def test_oversized_module_and_broken_link_fail(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self._write_fixture(root, broken=True)

            result = self._run(root, "check")

            self.assertEqual(result.returncode, 1, result.stderr)
            raw_violations = self._payload(result)["violations"]
            self.assertIsInstance(raw_violations, list)
            violations = cast("list[dict[str, object]]", raw_violations)
            codes = {item["code"] for item in violations}
            self.assertIn("line_budget_exceeded", codes)
            self.assertIn("broken_link", codes)

    def _run(
        self,
        root: Path,
        mode: str,
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [
                sys.executable,
                "-m",
                "scripts.check",
                "--root",
                str(root),
                "--mode",
                mode,
                "--json",
            ],
            check=False,
            capture_output=True,
            text=True,
            cwd=SKILL_ROOT,
        )

    def _write_fixture(self, root: Path, *, broken: bool) -> None:
        docs = root / ".agent-harness"
        modules = docs / "testing"
        modules.mkdir(parents=True)
        root_link = "testing/unit.md"
        _ = (docs / "TESTING.md").write_text(
            f"# Testing\n\n[Unit]({root_link})\n",
            encoding="utf-8",
        )
        module_lines = [
            "# Unit",
            "",
            "[Testing index](../TESTING.md)",
            "",
            "`Candidates [](Index/Recommended/Text)` is not a Markdown link.",
        ]
        if broken:
            module_lines.extend(["", "one", "two", "three"])
            module_lines.extend(["", "[Missing](missing.md)"])
        _ = (modules / "unit.md").write_text(
            "\n".join(module_lines) + "\n",
            encoding="utf-8",
        )
        manifest: dict[str, object] = {
            "schema_version": 1,
            "max_root_lines": 100,
            "max_module_lines": 5,
            "families": [
                {
                    "root": ".agent-harness/TESTING.md",
                    "module_dir": ".agent-harness/testing",
                    "responsibility": "testing",
                }
            ],
            "single_owner_topics": dict[str, str](),
        }
        manifest_dir = docs / "documentation"
        manifest_dir.mkdir()
        _ = (manifest_dir / "manifest.json").write_text(
            json.dumps(manifest),
            encoding="utf-8",
        )

    def _payload(
        self,
        result: subprocess.CompletedProcess[str],
    ) -> dict[str, object]:
        payload = cast("object", json.loads(result.stdout))
        self.assertIsInstance(payload, dict)
        return cast("dict[str, object]", payload)


if __name__ == "__main__":
    _ = unittest.main()

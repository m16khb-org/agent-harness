#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("e2e_stability_audit.py")
SPEC = importlib.util.spec_from_file_location("e2e_stability_audit", SCRIPT)
assert SPEC is not None
audit = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(audit)


class StabilityAuditScriptTest(unittest.TestCase):
    def test_run_returns_structured_timeout_result(self) -> None:
        result = audit.run(
            [sys.executable, "-c", "import time; time.sleep(2)"],
            timeout=0.1,
        )

        self.assertEqual(result["returncode"], -1)
        self.assertTrue(result["timeout"])
        self.assertIn("timed out", result["stderr"])
        self.assertGreaterEqual(result["duration_ms"], 0)

    def test_full_self_verify_timeout_has_full_gate_budget(self) -> None:
        self.assertGreaterEqual(audit.FULL_SELF_VERIFY_TIMEOUT_SECONDS, 600)


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("api_doc_gate.py")


def load_api_doc_gate():
    spec = importlib.util.spec_from_file_location("api_doc_gate", SCRIPT)
    if spec is None or spec.loader is None:
        raise RuntimeError("failed to load api_doc_gate.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ApiDocGateTest(unittest.TestCase):
    def test_finds_canonical_issueops_binary(self) -> None:
        module = load_api_doc_gate()

        found = Path(module.find_harness_binary())

        self.assertEqual(found.name, "issueops")
        self.assertEqual(found.parent.name, "bin")


if __name__ == "__main__":
    unittest.main()

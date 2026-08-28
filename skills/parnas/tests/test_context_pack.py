from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).parents[1] / "scripts"
spec = importlib.util.spec_from_file_location("mr_context", SCRIPTS / "mr_context.py")
mr_context = importlib.util.module_from_spec(spec)
assert spec.loader
spec.loader.exec_module(mr_context)

PATCH = """diff --git a/src/a.ts b/src/a.ts
--- a/src/a.ts
+++ b/src/a.ts
@@ -10,4 +10,6 @@ export class UserService {
+  async findMasked(id: string): Promise<User> {
+    return this.repo.findOne(id)
+  }
+  const x = 1
 }
diff --git a/pkg/b.go b/pkg/b.go
--- a/pkg/b.go
+++ b/pkg/b.go
@@ -1,2 +1,4 @@
+func cleanupWorkspace(ctx context.Context) error {
+\treturn nil
+}
 package b
diff --git a/scripts/c.py b/scripts/c.py
--- a/scripts/c.py
+++ b/scripts/c.py
@@ -1,1 +1,3 @@
+def parse_hunks(diff_text: str) -> dict:
+    return {}
+import os
"""


class ChangedSymbolsTest(unittest.TestCase):
    def test_extracts_definitions_and_hunk_context_only(self) -> None:
        syms = mr_context.changed_symbols(PATCH)
        self.assertEqual(syms, ["UserService", "findMasked", "cleanupWorkspace", "parse_hunks"])

    def test_test_files_come_last_and_skip_call_shaped_lines(self) -> None:
        patch = (
            "+++ b/src/a.spec.ts\n@@ -1,1 +1,3 @@\n+  expect(\n+describe('x', () => {\n+function helperFixture() {\n"
            "+++ b/src/svc.ts\n@@ -1,1 +1,2 @@\n+  maskGenderAgeForList(rows: Row[]): Row[] {\n"
        )
        self.assertEqual(mr_context.changed_symbols(patch), ["maskGenderAgeForList", "helperFixture"])

    def test_caps_symbol_count(self) -> None:
        many = "\n".join(f"+def f{i}():" for i in range(100))
        patch = "@@ -1,1 +1,100 @@\n" + many
        self.assertEqual(len(mr_context.changed_symbols(patch, cap=7)), 7)


class PerLensCapTest(unittest.TestCase):
    def test_distributes_candidate_budget_over_lenses(self) -> None:
        self.assertEqual(mr_context.per_lens_cap(24, 10), 4)
        self.assertEqual(mr_context.per_lens_cap(12, 10), 3)
        self.assertEqual(mr_context.per_lens_cap(24, 5), 6)
        self.assertEqual(mr_context.per_lens_cap(12, 21), 3)
        self.assertEqual(mr_context.per_lens_cap(12, 0), 3)


class DefsFallbackTest(unittest.TestCase):
    def test_rg_fallback_lists_definition_and_callers(self) -> None:
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            (root / "svc.ts").write_text("export function findMasked(id) { return id }\n")
            (root / "ctl.ts").write_text("import { findMasked } from './svc'\nfindMasked('x')\n")
            md = mr_context.build_defs(str(root), ["findMasked"], codegraph=False)
            self.assertIn("## findMasked", md)
            self.assertIn("svc.ts:1", md)
            self.assertIn("ctl.ts:2", md)

    def test_empty_symbols_yields_note(self) -> None:
        md = mr_context.build_defs("/nonexistent", [], codegraph=False)
        self.assertIn("(no symbols", md)


class PackTest(unittest.TestCase):
    FILES = [
        {"path": "apps/grpc-ai/src/persona/persona.service.ts", "tags": ["source"], "hunks": [{"new_start": 10, "new_len": 5}], "added": 5, "removed": 0},
        {"path": "apps/rest-api-gateway/src/persona/persona.controller.ts", "tags": ["controller", "gateway"], "hunks": [{"new_start": 1, "new_len": 3}], "added": 3, "removed": 0},
        {"path": "apps/grpc-ai/src/persona/persona.service.spec.ts", "tags": ["test"], "hunks": [{"new_start": 1, "new_len": 2}], "added": 2, "removed": 0},
        {"path": "docs/notes.md", "tags": ["docs"], "hunks": [{"new_start": 1, "new_len": 1}], "added": 1, "removed": 0},
    ]
    PER_FILE = {f["path"]: f"diff for {f['path']}\n" for f in FILES}

    def test_lens_file_filter(self) -> None:
        pick = lambda lens: [f["path"].split("/")[-1] for f in mr_context.files_for_lens(lens, self.FILES, self.PER_FILE)]
        self.assertEqual(pick("contract"), ["persona.controller.ts"])
        self.assertEqual(pick("tests"), ["persona.service.ts", "persona.controller.ts", "persona.service.spec.ts"])
        self.assertEqual(pick("logic"), ["persona.service.ts", "persona.controller.ts", "persona.service.spec.ts"])
        self.assertEqual(pick("scope"), [f["path"].split("/")[-1] for f in self.FILES])

    def test_keyword_lenses_match_per_file_diff(self) -> None:
        per_file = dict(self.PER_FILE); per_file[self.FILES[0]["path"]] = "+ await this.repository.save(x)\n"
        self.assertEqual([f["path"].split("/")[-1] for f in mr_context.files_for_lens("data", self.FILES, per_file)], ["persona.service.ts"])
        self.assertEqual(mr_context.files_for_lens("data", self.FILES, self.PER_FILE), [])

    def test_shards_split_by_size_cap_along_directory_groups(self) -> None:
        big = {p: "x" * 900 for p in self.PER_FILE}
        shards = mr_context.shard_files(self.FILES, big, cap_bytes=1000)
        self.assertEqual(len(shards), 4)
        self.assertTrue(all(len(s["files"]) == 1 for s in shards))
        two = mr_context.shard_files(self.FILES, big, cap_bytes=1900)
        self.assertEqual([len(s["files"]) for s in two], [2, 2])
        self.assertEqual(two[0]["files"][0]["path"], "apps/grpc-ai/src/persona/persona.service.spec.ts")
        one = mr_context.shard_files(self.FILES, self.PER_FILE, cap_bytes=10_000)
        self.assertEqual(len(one), 1)
        self.assertEqual(one[0]["id"], "all")

    def test_pack_contains_only_its_files_and_defs(self) -> None:
        defs = {"maskGenderAge": {"file": "apps/grpc-ai/src/persona/persona.service.ts", "rows": ["- a:1"]},
                "helper": {"file": "apps/rest-api-gateway/src/persona/persona.controller.ts", "rows": ["- b:2"]}}
        md = mr_context.render_pack({"id": "grpc-ai", "files": [self.FILES[0]]}, ["logic", "data"], {"logic": "LENS TEXT", "data": "DATA TEXT"}, self.PER_FILE, defs,
                                    {"title": "T", "description": "D" * 50, "threads": [], "lessons": [], "rules": []})
        self.assertIn("- `logic`: LENS TEXT", md)
        self.assertIn("- `data`: DATA TEXT", md)
        self.assertIn("diff for apps/grpc-ai/src/persona/persona.service.ts", md)
        self.assertNotIn("persona.controller.ts", md)
        self.assertIn("## maskGenderAge", md)
        self.assertNotIn("## helper", md)
        self.assertIn("hunks(new): 10-14", md)

    def test_bundles_cover_every_lens_once(self) -> None:
        all_lenses = [l for _, ls in mr_context.LENS_BUNDLES for l in ls]
        self.assertEqual(sorted(all_lenses), sorted(mr_context.load_lenses().keys()))
        self.assertEqual(len(all_lenses), len(set(all_lenses)))

    def test_hunk_slug_roundtrip(self) -> None:
        self.assertEqual(mr_context.hunk_slug("apps/a b/c.ts"), "apps__a_b__c.ts")


class ResearchImprovementsTest(unittest.TestCase):
    def test_enclosing_context_walks_up_to_definition(self) -> None:
        src = "import x\n\nexport class Svc {\n  private a = 1\n\n  async find(id: string) {\n    const q = 1\n    return q\n  }\n}\n"
        lines = src.splitlines()
        # hunk starts at line 7 (`const q = 1`); enclosing def is line 6, class is line 3
        ctx = mr_context.enclosing_context(lines, new_start=7, max_up=30)
        self.assertEqual(ctx, [(3, "export class Svc {"), (6, "  async find(id: string) {")])
        self.assertEqual(mr_context.enclosing_context(lines, new_start=1, max_up=30), [])

    def test_refuted_history_match_uses_path_and_tokens_and_exempts_security(self) -> None:
        hist = [{"path": "src/a.ts", "title": "v1 북마크 목록이 order 를 화이트리스트 없이 넘겨 ORDER BY 주입", "category": "bug"},
                {"path": "src/b.ts", "title": "토큰 검증이 빠져 인증 우회", "category": "security"}]
        self.assertTrue(mr_context.matches_refuted(hist, {"path": "src/a.ts", "title": "북마크 목록 order 화이트리스트 우회 ORDER BY 주입 표면", "category": "bug"}))
        self.assertFalse(mr_context.matches_refuted(hist, {"path": "src/c.ts", "title": "북마크 목록 order 화이트리스트 우회 ORDER BY 주입 표면", "category": "bug"}))
        self.assertFalse(mr_context.matches_refuted(hist, {"path": "src/b.ts", "title": "토큰 검증 빠져 인증 우회", "category": "security"}))

    def test_incremental_plan_keeps_unchanged_findings_and_reinspects_changed_shards(self) -> None:
        prev = {"head_sha": "old", "findings": [
            {"path": "src/a.ts", "new_line": 10, "title": "A"}, {"path": "src/b.ts", "new_line": 5, "title": "B"}]}
        changed = {"src/b.ts", "src/new.ts"}
        units = [{"id": "behavior@1-x", "files": ["src/a.ts"]}, {"id": "behavior@2-y", "files": ["src/b.ts", "src/new.ts"]}, {"id": "intent@1-x", "files": ["src/a.ts"]}]
        plan = mr_context.incremental_plan(prev, changed, units)
        self.assertEqual([u["id"] for u in plan["units"]], ["behavior@2-y"])
        self.assertEqual([f["title"] for f in plan["carried"]], ["A"])
        self.assertEqual(plan["dropped"], 1)


if __name__ == "__main__":
    unittest.main()

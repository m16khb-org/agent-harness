from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).parents[1] / "scripts"


def load(name: str):
    spec = importlib.util.spec_from_file_location(name, SCRIPTS / f"{name}.py")
    mod = importlib.util.module_from_spec(spec)
    assert spec.loader
    spec.loader.exec_module(mod)
    return mod


prescreen = load("prescreen")
mr_context = load("mr_context")

HUNKS = {"src/a.ts": [[10, 20]], "src/b.ts": [[5, 8]]}


def cand(**kw):
    base = {"path": "src/a.ts", "new_line": 12, "title": "널 역참조", "what": "user 가 없을 수 있다",
            "why": "id 미존재 시 undefined", "category": "bug", "confidence": 70, "lens": "logic"}
    return {**base, **kw}


class TestPrescreenRule(unittest.TestCase):
    def test_keeps_candidate_on_a_changed_hunk_line(self):
        self.assertIsNone(prescreen.prescreen(cand(), HUNKS, []))

    def test_refuses_a_file_that_is_not_in_the_change(self):
        why = prescreen.prescreen(cand(path="src/untouched.ts"), HUNKS, [])
        self.assertIn("is not a changed file", why or "")

    def test_refuses_a_line_outside_every_hunk(self):
        why = prescreen.prescreen(cand(new_line=99), HUNKS, [])
        self.assertIn("outside every hunk", why or "")

    def test_keeps_an_off_hunk_line_marked_newly_reachable(self):
        self.assertIsNone(prescreen.prescreen(cand(new_line=99, newly_reachable=True), HUNKS, []))

    def test_refuses_a_claim_already_refuted_on_the_same_file(self):
        history = [{"path": "src/a.ts", "title": "널 역참조", "what": "user 가 없을 수 있다"}]
        why = prescreen.prescreen(cand(), HUNKS, history)
        self.assertIn("refuted before", why or "")

    def test_refutation_memory_is_scoped_to_its_own_file(self):
        history = [{"path": "src/b.ts", "title": "널 역참조", "what": "user 가 없을 수 있다"}]
        self.assertIsNone(prescreen.prescreen(cand(), HUNKS, history))

    def test_security_and_data_survive_learned_suppression(self):
        history = [{"path": "src/a.ts", "title": "널 역참조", "what": "user 가 없을 수 있다"}]
        for category in ("security", "data"):
            self.assertIsNone(prescreen.prescreen(cand(category=category), HUNKS, history), category)


class TestDedup(unittest.TestCase):
    def test_same_defect_from_two_lenses_collapses_and_keeps_both_lenses(self):
        out = prescreen.dedup([cand(lens="logic", confidence=60), cand(lens="boundary", confidence=85)])
        self.assertEqual(len(out), 1)
        self.assertEqual(out[0]["lenses"], ["logic", "boundary"])
        self.assertEqual(out[0]["confidence"], 85)

    def test_distinct_defects_are_not_merged(self):
        other = cand(path="src/b.ts", new_line=6, title="트랜잭션 경계 누락",
                     what="쓰기 두 건이 갈라진다", why="부분 실패 시 정합성 깨짐")
        self.assertEqual(len(prescreen.dedup([cand(), other])), 2)

    def test_merged_candidate_keeps_the_security_category_so_it_stays_exempt(self):
        out = prescreen.dedup([cand(lens="logic"), cand(lens="security", category="security")])
        self.assertEqual(len(out), 1)
        self.assertEqual(out[0]["category"], "security")

    def test_output_is_ordered_by_confidence(self):
        out = prescreen.dedup([cand(title="A", why="a", confidence=30),
                               cand(path="src/b.ts", new_line=6, title="B", why="b", confidence=90)])
        self.assertEqual([c["confidence"] for c in out], [90, 30])


class TestCli(unittest.TestCase):
    def test_end_to_end_drops_off_hunk_and_caps_at_max_candidates(self):
        with tempfile.TemporaryDirectory() as d:
            d = Path(d)
            (d / "workflow_args.json").write_text(json.dumps(
                {"level": "high", "hunkRanges": HUNKS, "refutedHistory": [], "maxCandidates": 2}))
            (d / "candidates.json").write_text(json.dumps([
                cand(title="A", why="a", confidence=90),
                cand(path="src/b.ts", new_line=6, title="B", why="b", confidence=80),
                cand(path="src/b.ts", new_line=7, title="C", why="c", category="test", confidence=70),
                cand(path="src/gone.ts", new_line=1, title="D", why="d", confidence=95),
            ]))
            out = subprocess.run(
                [sys.executable, str(SCRIPTS / "prescreen.py"), "--args", str(d / "workflow_args.json"),
                 "--candidates", str(d / "candidates.json"), "--out", str(d / "out.json")],
                capture_output=True, text=True, check=True)
            stats = json.loads(out.stdout)
            result = json.loads((d / "out.json").read_text())
            self.assertEqual(stats["prescreened"], 1)          # src/gone.ts is not in the change
            self.assertEqual(stats["kept"], 2)                 # maxCandidates
            self.assertEqual(stats["over_cap"], 1)
            self.assertEqual([c["title"] for c in result["candidates"]], ["A", "B"])
            self.assertIn("is not a changed file", result["prescreened"][0]["verdicts"][0]["reason"])


class TestLevels(unittest.TestCase):
    def test_only_max_fans_out_and_runs_skeptics(self):
        for name, plan in mr_context.LEVELS.items():
            self.assertEqual(plan["fanout"], name == "max", name)
            self.assertEqual(plan["skeptics"], name == "max", name)

    def test_max_keeps_the_scale_based_candidate_budget(self):
        self.assertIsNone(mr_context.LEVELS["max"]["max_candidates"])

    def test_per_lens_cap_separates_the_inline_levels(self):
        caps = [mr_context.per_lens_cap(mr_context.LEVELS[l]["max_candidates"], 8)
                for l in ("medium", "high", "xhigh")]
        self.assertEqual(caps, sorted(set(caps)), f"levels must widen the search, got {caps}")

    def test_low_narrows_to_the_two_always_on_lenses(self):
        self.assertEqual(mr_context.LEVELS["low"]["lenses"], ["logic", "boundary"])

    def test_default_level_is_an_inline_one(self):
        self.assertIn(mr_context.DEFAULT_LEVEL, mr_context.LEVELS)
        self.assertFalse(mr_context.LEVELS[mr_context.DEFAULT_LEVEL]["fanout"])

    def test_the_level_plan_is_never_rebound_inside_main(self):
        """main() also has a local `plan` from incremental_plan(); the level plan must not share
        a name with it. When it did, --incremental on a moved head overwrote the level plan and
        workflow_args.json / summary.md died on the next key read."""
        import ast
        tree = ast.parse((SCRIPTS / "mr_context.py").read_text())
        main = next(n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == "main")
        binds = [t.id for n in ast.walk(main) if isinstance(n, ast.Assign)
                 for t in n.targets if isinstance(t, ast.Name) and t.id == "level_plan"]
        self.assertEqual(len(binds), 1, "level_plan is assigned more than once in main()")


if __name__ == "__main__":
    unittest.main()

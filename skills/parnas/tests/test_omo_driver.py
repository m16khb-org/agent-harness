from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).parents[1] / "scripts"
spec = importlib.util.spec_from_file_location("omo_driver", SCRIPTS / "omo_driver.py")
omo_driver = importlib.util.module_from_spec(spec)
assert spec.loader
spec.loader.exec_module(omo_driver)

ARGS = {
    "hunkRanges": {"src/a.go": [[10, 20]], "src/other.go": [[10, 20]]},
    "refutedHistory": [{"path": "src/a.go", "title": "order 를 화이트리스트 없이 넘겨 ORDER BY 주입", "what": "", "category": "bug"}],
    "maxCandidates": 24,
    "lenses": ["logic", "boundary", "tests", "rules", "scope", "security"],
}


class ProfileTest(unittest.TestCase):
    def test_omo_flash_profile_raises_budgets_over_args(self) -> None:
        b = omo_driver.resolve_budget(ARGS, "omo-flash")
        self.assertEqual(b["finder_turns"], 24)
        self.assertEqual(b["skeptic_turns"], 18)
        self.assertEqual(b["max_candidates"], 40)          # floor raises 24 → 40
        self.assertGreater(b["per_lens_cap"], 5)           # recomputed from the raised cap
        self.assertEqual(b["workers"]["finder"], 4)          # z.ai concurrency cap
        self.assertEqual(b["thinking"], {"finder": "high", "tracer": "high", "reproducer": "high"})

    def test_standard_profile_keeps_workflow_budgets(self) -> None:
        b = omo_driver.resolve_budget(ARGS, "standard")
        self.assertEqual(b["finder_turns"], 10)
        self.assertEqual(b["skeptic_turns"], 8)
        self.assertEqual(b["max_candidates"], 24)          # no floor applied
        self.assertEqual(b["thinking"]["tracer"], "medium")


class PrescreenTest(unittest.TestCase):
    def test_drops_off_hunk_line_without_newly_reachable(self) -> None:
        c = {"path": "src/a.go", "new_line": 999, "newly_reachable": False, "category": "bug"}
        why = omo_driver.prescreen(ARGS, c)
        self.assertIsNotNone(why)
        self.assertIn("outside every hunk", why)

    def test_keeps_hunk_line_and_newly_reachable(self) -> None:
        self.assertIsNone(omo_driver.prescreen(ARGS, {"path": "src/a.go", "new_line": 15, "category": "bug"}))
        self.assertIsNone(omo_driver.prescreen(ARGS, {"path": "src/a.go", "new_line": 999, "newly_reachable": True, "category": "bug"}))

    def test_newly_reachable_unchanged_file_survives(self) -> None:
        """Consumer regressions on untouched files must reach a skeptic when flagged (dogfood #347)."""
        c = {"path": "src/never-changed.vue", "new_line": 407, "newly_reachable": True, "category": "bug", "title": "t", "what": "w"}
        self.assertIsNone(omo_driver.prescreen(ARGS, c))
        silent = {**c, "newly_reachable": False}
        self.assertIsNotNone(omo_driver.prescreen(ARGS, silent))

    def test_refuted_history_suppresses_same_file_overlap_only(self) -> None:
        c = {"path": "src/a.go", "new_line": 15, "category": "bug",
             "title": "order 화이트리스트 없이 ORDER BY 주입", "what": ""}
        self.assertIsNotNone(omo_driver.prescreen(ARGS, c))
        other = {"path": "src/other.go", "new_line": 15, "category": "bug",
                 "title": "order 화이트리스트 없이 ORDER BY 주입", "what": ""}
        self.assertIsNone(omo_driver.prescreen(ARGS, other))

    def test_security_is_never_suppressed(self) -> None:
        c = {"path": "src/a.go", "new_line": 15, "category": "security",
             "title": "order 를 화이트리스트 없이 넘겨 ORDER BY 주입", "what": ""}
        self.assertIsNone(omo_driver.prescreen(ARGS, c))


class PureHelperTest(unittest.TestCase):
    def test_severity_moves_one_step_only_with_direction(self) -> None:
        self.assertEqual(omo_driver.severity_shift("medium", "raise"), "high")
        self.assertEqual(omo_driver.severity_shift("critical", "raise"), "critical")
        self.assertEqual(omo_driver.severity_shift("low", "lower"), "low")
        self.assertEqual(omo_driver.severity_shift("medium", "keep"), "medium")

    def test_extract_json_takes_outermost_object_from_prose(self) -> None:
        text = '설명입니다\n{"refuted": true, "confidence": 90}\n끝'
        self.assertEqual(omo_driver.extract_json(text), {"refuted": True, "confidence": 90})
        self.assertIsNone(omo_driver.extract_json("JSON 아님"))

    def test_similar_is_symmetric_bigram_coefficient(self) -> None:
        self.assertGreaterEqual(omo_driver.similar("ORDER BY 주입", "ORDER BY 인젝션"), 0.4)
        self.assertEqual(omo_driver.similar("abc", "xyz"), 0.0)

    def test_dedup_merges_same_spot_and_keeps_security_category(self) -> None:
        finders = [
            {"lenses": ["logic"], "candidates": [
                {"path": "a.go", "new_line": 10, "category": "bug", "title": "널 역참조", "confidence": 60, "lens": "logic"}]},
            {"lenses": ["security"], "candidates": [
                {"path": "a.go", "new_line": 11, "category": "security", "title": "널 역참조", "confidence": 80, "lens": "security"}]},
        ]
        merged = omo_driver.dedup_candidates(finders)
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["category"], "security")   # security reading survives dedup
        self.assertEqual(merged[0]["confidence"], 80)
        self.assertEqual(sorted(merged[0]["lenses"]), ["logic", "security"])

    def test_blind_hides_finder_evidence_from_tracer(self) -> None:
        c = {"path": "a.go", "new_line": 1, "severity": "high", "category": "bug", "title": "t",
             "what": "w", "evidence": ["a.go:1"], "upstream": "u", "downstream": "d", "lens": "logic"}
        b = omo_driver.blind(c)
        self.assertNotIn("evidence", b)
        self.assertNotIn("upstream", b)
        self.assertIn("what", b)


class SelfReviewFixTest(unittest.TestCase):
    """PR #498 parnas 자기 리뷰가 발견한 결함의 회귀 테스트 (F1~F6)."""

    def test_dedup_merges_on_why_similarity_not_what(self) -> None:
        """F3: workflow.js 는 마지막 절에 why 유사도를 쓴다 — what 이 아니라."""
        finders = [
            {"lenses": ["logic"], "candidates": [
                {"path": "a.go", "new_line": 10, "category": "bug", "title": "제목 하나",
                 "what": "전혀 다른 서술 alpha", "why": "null 역참조로 프로세스가 크래시", "confidence": 60, "lens": "logic"}]},
            {"lenses": ["boundary"], "candidates": [
                {"path": "b.go", "new_line": 10, "category": "bug", "title": "관계없는 제목 둘",
                 "what": "전혀 다른 서술 gamma", "why": "null 역참조로 프로세스가 크래시", "confidence": 70, "lens": "boundary"}]},
        ]
        merged = omo_driver.dedup_candidates(finders)
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["confidence"], 70)

    def test_dedup_skips_non_dict_candidates(self) -> None:
        """F6: 스키마를 벗어난 finder 응답이 크래시를 일으키지 않는다."""
        finders = [{"lenses": ["logic"], "candidates": ["junk-string",
                                                            {"path": "a.go", "new_line": 1, "category": "bug", "title": "t", "lens": "logic"}]}]
        merged = omo_driver.dedup_candidates(finders)
        self.assertEqual(len(merged), 1)
        self.assertEqual(merged[0]["path"], "a.go")

    def test_phase_find_empty_units_returns_empty_without_agents(self) -> None:
        """F2: units 가 비면 에이전트 없이 빈 구조체를 반환한다 (IndexError 아님)."""
        a = {"units": [], "checkout": "/tmp"}
        found = omo_driver.phase_find(a, runner=None)
        self.assertEqual(found["finders"], [])
        self.assertEqual(found["candidates"], [])
        self.assertEqual(found["prescreened"], [])
        self.assertEqual(found["verified_ok"], [])


if __name__ == "__main__":
    unittest.main()

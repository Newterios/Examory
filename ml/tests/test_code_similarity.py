"""Java code baseline, source offsets and event contract regression tests."""

from __future__ import annotations

import unittest
import uuid
from datetime import datetime
from unittest.mock import patch

from ml.integrity.code.algorithm import (
    JavaLexer,
    exact_token_match,
    overlap_spans,
    prepare,
    similarity,
    winnow,
)
from ml.integrity.code.to_risk_event import compare_submissions


class JavaSimilarityTests(unittest.TestCase):
    def test_tokenization_normalizes_literals_and_preserves_offsets(self):
        text = '/* 🐈 */\r\nint 累計$1 = 0x1_f.fp2f;\r\n//note\rreturn "/* // \\"";'
        tokens = JavaLexer().tokenize(text)
        self.assertEqual(
            [t.value for t in tokens],
            ["int", "@identifier", "=", "@number", ";", "return", "@string", ";"],
        )
        self.assertEqual(
            [text[t.start : t.end] for t in tokens], [t.raw for t in tokens]
        )
        self.assertEqual(tokens[1].start, text.index("累計"))
        literal = '"""\nString inner = "// not a comment";\n"""'
        self.assertEqual(JavaLexer().tokenize(literal)[0].value, "@string")

    def test_rightmost_tie_and_hash_collision(self):
        self.assertEqual(winnow([4, 1, 1, 9], 3), [2])
        self.assertEqual(winnow([7, 7, 7, 7, 7], 3), [2, 3, 4])
        with patch("ml.integrity.code.algorithm._hash", return_value=0):
            a = prepare("if (true) return;")
            b = prepare("class A { int counter; }")
            self.assertEqual(similarity(a, b), 0)
            self.assertEqual(overlap_spans(a, b), [])

    def test_local_overlaps_and_raw_comparator(self):
        target = "/* 🐈 */\r\nint sum(int n) { return n * 123 + 200; }"
        source = "int calc(int z) { /*x*/ return z * 6 + 7; }"
        a, b = prepare(target), prepare(source)
        self.assertEqual(similarity(a, b), 1)
        self.assertFalse(exact_token_match(a, b))
        self.assertTrue(
            exact_token_match(a, prepare(target.replace("sum(", "sum /*x*/ (")))
        )
        self.assertFalse(exact_token_match(prepare("x"), prepare("x")))
        for span in overlap_spans(a, b):
            lhs = JavaLexer().tokenize(target[slice(*span["target"])])
            rhs = JavaLexer().tokenize(source[slice(*span["source"])])
            self.assertEqual([t.value for t in lhs], [t.value for t in rhs])
            self.assertEqual(len(lhs), span["token_count"])

    def test_risk_event_levels_template_and_abstention(self):
        text = "void run(int size) { while (size-- > 0) doWork(size); }"
        context = {"course_id": "c1", "group_id": "g1", "session_id": None}

        def compare(source: str, **options):
            return compare_submissions(
                "s2", text, "s1", source, context, threshold=0.75, **options
            )

        event = compare(text)
        self.assertEqual(
            (event["event_type"], event["version"], event["context"]),
            ("risk.plagiarism", 1, context),
        )
        event_id = uuid.UUID(event["event_id"])
        self.assertEqual(event_id.version, 7)
        self.assertEqual(
            event_id.int >> 80,
            int(datetime.fromisoformat(event["occurred_at"]).timestamp() * 1000),
        )
        payload = event["payload"]
        self.assertEqual(
            set(payload),
            {
                "model",
                "evidence",
                "explanation",
                "risk_id",
                "target_id",
                "target_type",
                "score",
                "level",
            },
        )
        self.assertEqual(
            (
                payload["target_type"],
                payload["target_id"],
                payload["score"],
                payload["level"],
            ),
            ("submission", "s2", 1, "medium"),
        )
        match = payload["evidence"]["matches"][0]
        self.assertEqual((match["submission_id"], match["similarity"]), ("s1", 1))
        self.assertTrue(match["overlap_spans"])
        self.assertEqual(compare(text, high=1)["payload"]["level"], "high")
        self.assertIsNone(compare(text, template=text))
        self.assertIsNone(compare("/* comment */"))


if __name__ == "__main__":
    unittest.main()

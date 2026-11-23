import csv
import json
import tempfile
import unittest
from pathlib import Path

from ml.moodle.convert import convert


class MoodleTests(unittest.TestCase):
    def test_source_rows_stay_private_and_ordered(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            a, h, out = (root / x for x in ("attempts.csv", "steps.csv", "out.csv"))
            a.write_text(
                "attempt_id,user_id,course_id,group_id,quiz_id,variant_id,start,finish,score,max_score,question_count,time_limit_sec,reason,email\n"
                "attempt123,privateUser,course,group,quiz,v,1700000000,1700000020,1.5,2,2,60,submitted,private@example.org\n"
            )
            with h.open("w", newline="") as f:
                w = csv.writer(f)
                w.writerow(
                    [
                        "attempt_id",
                        "question_id",
                        "slot",
                        "sequence",
                        "shown",
                        "saved",
                        "options",
                    ]
                )
                for row in [
                    ("one", 1, 0, "1700000001", 1700000002, ["a"]),
                    ("one", 1, 1, "", 1700000003, ["a"]),
                    ("one", 1, 2, "", 1700000004, ["b"]),
                    ("two", 2, 0, "", 1700000007, ["c"]),
                ]:
                    w.writerow(["attempt123", *row[:-1], json.dumps(row[-1])])
            report = convert(a, h, out, b"k" * 32, "study", 2)
            first = out.read_bytes()
            self.assertEqual(
                report,
                {"attempts": 1, "events": 4, "saves_without_display_observation": 1},
            )
            self.assertNotIn(b"private", first)
            self.assertNotIn(b"attempt123", first)
            with out.open() as f:
                rows = [json.loads(e["payload"]) for e in csv.DictReader(f)]
            self.assertEqual([r["is_change"] for r in rows[1:-1]], [False, True])
            self.assertEqual(rows[2]["time_since_shown_ms"], 3000)
            self.assertEqual(
                (rows[-1]["score"], rows[-1]["max_score"], rows[-1]["answers_changed"]),
                (3, 4, 1),
            )
            convert(a, h, out, b"k" * 32, "study", 2)
            self.assertEqual(out.read_bytes(), first)
            convert(a, h, out, b"x" * 32, "study", 2)
            self.assertNotEqual(out.read_bytes(), first)

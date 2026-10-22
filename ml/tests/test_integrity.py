# ml/tests/test_integrity.py
"""Text baseline checks using fixtures, not PAN benchmark measurements."""
from __future__ import annotations

import argparse
import json
import tempfile
import unittest
from pathlib import Path

from ml.integrity.evaluate import evaluate as evaluate_text
from ml.integrity.minhash import Detector, document, tokenize
from ml.integrity.to_risk_event import risk_event as text_risk_event


class IntegrityTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.temporary = tempfile.TemporaryDirectory(prefix="examory-integrity-")
        cls.root = Path(cls.temporary.name)

    @classmethod
    def tearDownClass(cls) -> None:
        cls.temporary.cleanup()


    def test_minhash_offsets_tail_and_templates(self) -> None:
        text = "Préface\r\n" + " ".join(f"word{i}" for i in range(320))
        detector = Detector({"source": text})
        query = detector.prepare("target", text.upper())
        matches = detector.verify(query, detector.retrieve(query), .8, 50)
        self.assertEqual(len(matches), 1)
        self.assertEqual(matches[0]["similarity"], 1)
        self.assertEqual(matches[0]["overlap_spans"][0]["target"], [0, len(text)])
        self.assertEqual(matches[0]["overlap_spans"][0]["source"], [0, len(text)])
        self.assertEqual(document("tail", " ".join(f"t{i}" for i in range(305))).windows[-1].start, 300)
        self.assertFalse(document("short", "one two three four").windows)
        self.assertEqual(tokenize("Cafe\u0301")[0].value, tokenize("Café")[0].value)
        excluded = Detector({"source": text}, templates=[text])
        self.assertFalse(excluded.prepare("target", text).windows)
        unrelated = detector.prepare("other", " ".join(f"different{i}" for i in range(100)))
        self.assertEqual(detector.verify(unrelated, detector.retrieve(unrelated)), [])


    def test_pan_format_end_to_end(self) -> None:
        corpus = self.root / "pan_fixture"
        corpus.mkdir()
        for number in range(20):
            source = f"source-document{number:05d}.txt"
            target = f"suspicious-document{number:05d}.txt"
            text = " ".join(f"family{number}word{i}" for i in range(110))
            (corpus / source).write_text(text, encoding="utf-8")
            (corpus / target).write_text(text, encoding="utf-8")
            annotation = (f'<document reference="{target}"><feature name="plagiarism" '
                          f'this_offset="0" this_length="{len(text)}" source_reference="{source}" '
                          f'source_offset="0" source_length="{len(text)}" obfuscation="none" /></document>')
            (corpus / target.replace(".txt", ".xml")).write_text(annotation, encoding="utf-8")
        args = argparse.Namespace(corpus=corpus, max_documents=40, seed=42, mode="windows",
                                  num_perm=128, templates=None, artifacts=self.root / "text",
                                  output=self.root / "text.md")
        result = evaluate_text(args)
        self.assertEqual(result["test"][0]["candidate_recall"], 1)
        self.assertEqual(result["test"][0]["f1"], 1)
        self.assertEqual(result["test"][0]["target_spans"]["recall"], 1)
        event = text_risk_event("target", [{"submission_id": "source", "similarity": 1.0,
                                           "overlap_spans": [{"target": [0, 100], "source": [5, 105], "token_count": 20}]}], {}, result)
        self.assertEqual(event["event_type"], "risk.plagiarism")
        self.assertEqual(event["payload"]["target_type"], "submission")
        json.dumps(event, allow_nan=False)


if __name__ == "__main__":
    unittest.main()

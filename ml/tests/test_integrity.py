# ml/tests/test_integrity.py
"""Text baseline checks using fixtures, not PAN benchmark measurements."""
from __future__ import annotations

import argparse
import json
import tempfile
import unittest
from pathlib import Path

from ml.integrity.evaluate import evaluate as evaluate_text
from ml.integrity.evaluate import h3_metrics, h3_pairs, read_pan, span_metrics
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

    def test_h3_counts_gold_passages_and_bootstraps_whole_families(self) -> None:
        text = " ".join(["word"] * 50)
        def span(category: str, length: int) -> dict:
            return {"category": category, "target": [0, length], "source": [0, length]}
        labels = {("a", "s"): [span("literal", len(text))],
                  ("b", "s"): [span("light", len(text))],
                  ("c", "s"): [span("heavy", len(text)), span("light", len(text) - 5)],
                  ("d", "s"): [span("translation", len(text))]}
        positives = h3_pairs(labels, {target: text for target, _ in labels})
        self.assertEqual(positives, {("a", "s"), ("b", "s")})
        negatives = {("a", "n"), ("b", "n")}
        predicted = {("a", "s"), ("a", "n"), ("c", "s"), ("d", "s")}
        families = {"a": "X", "b": "Y", "c": "Z", "d": "W"}
        row = h3_metrics(predicted, set(labels), positives, negatives, families, 42)
        self.assertEqual([row[k] for k in ("tp", "fp", "fn", "tn")], [1, 1, 1, 1])
        self.assertEqual(row["candidate_recall"], 1)
        self.assertEqual(row["bootstrap"]["candidate_recall"], {"defined": 1000, "ci95": [1, 1]})
        self.assertEqual(row["bootstrap"]["recall"]["ci95"], [0, 1])
        self.assertLess(row["bootstrap"]["precision"]["defined"], 1000)
        families["b"] = "X"
        self.assertEqual(h3_metrics(predicted, set(labels), positives, negatives, families, 42)["bootstrap"], {})

    def test_pan_unlocalized_annotation_preserves_positive_pair_and_family(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            corpus = Path(tmp)
            target, source = "suspicious-document001.txt", "source-document005.txt"
            text = " ".join(["word"] * 60)
            for name in (target, source):
                (corpus / name).write_text(text, encoding="utf-8")
            annotation = (f'<document reference="{target}"><feature name="plagiarism" type="simulated" '
                          f'this_offset="7" this_length="0" source_reference="{source}" '
                          'source_offset="0" source_length="256" /></document>')
            (corpus / "suspicious-document001.xml").write_text(annotation, encoding="utf-8")
            _, truth, families = read_pan(corpus)
            pair = (target, source)
            self.assertEqual(set(truth), {pair})
            self.assertEqual(families[target], families[source])
            self.assertEqual(truth[pair][0]["category"], "simulated")
            self.assertIsNone(truth[pair][0]["target"])
            self.assertEqual(h3_pairs(truth, {target: text}), set())
            predictions = {pair: [{"target": [0, 10], "source": [0, 10]}]}
            self.assertIsNone(span_metrics(predictions, truth, "source")["precision"])

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
            annotation = (f'<document reference="{target}"><feature name="plagiarism" type="artificial" '
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
        self.assertEqual(result["manifest"]["test_h3_positive_families"], 4)
        self.assertEqual(result["h3"]["lsh"]["candidate_hits"], 4)
        self.assertEqual(result["h3"]["lsh"]["positives"], 4)
        event = text_risk_event("target", [{"submission_id": "source", "similarity": 1.0,
                                           "overlap_spans": [{"target": [0, 100], "source": [5, 105], "token_count": 20}]}], {}, result)
        self.assertEqual(event["event_type"], "risk.plagiarism")
        self.assertEqual(event["payload"]["target_type"], "submission")
        json.dumps(event, allow_nan=False)


if __name__ == "__main__":
    unittest.main()

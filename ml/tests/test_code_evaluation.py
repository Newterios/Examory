"""Regression checks for clone-type denominators and separation of benchmark folds."""
from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import numpy as np

from ml.integrity.code.evaluate import Pair, load_pairs, summarize, tune


class CodeEvaluationTest(unittest.TestCase):
    def test_validation_tie_and_false_positive_denominator(self) -> None:
        labels = np.array([True, False, False])
        threshold, _ = tune(labels, np.array([.81, .19, .18]))
        self.assertEqual(threshold, .8)
        pairs = [Pair("a", "b", "T2", "test", "group1"),
                 Pair("c", "d", "nonclone", "test", "group1"),
                 Pair("e", "f", "nonclone", "test", "group2")]
        row = summarize(pairs, np.array([True, True, False]), 20, 42)
        self.assertEqual(row["groups"], 2)
        self.assertEqual(row["metrics"]["FPR"], .5)
        self.assertEqual(row["metrics"]["precision"], .5)
        self.assertEqual(row["metrics"]["recall"], 1.)

    def test_duplicate_content_across_splits_rejected_under_distinct_ids(self) -> None:
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            codes = ["same()", "different()", "same()", "another()"]
            (root / "functions.jsonl").write_text(
                "\n".join(json.dumps({"id": str(i), "code": code}) for i, code in enumerate(codes)),
                encoding="utf-8",
            )
            (root / "pairs.csv").write_text(
                "left_id,right_id,clone_type,split,group\n0,1,T2,validation,v1\n2,3,T2,test,t1\n",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(ValueError, "both splits"):
                load_pairs(root)

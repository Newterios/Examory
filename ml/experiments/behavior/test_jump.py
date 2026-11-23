"""Leakage boundaries for a score-gain feature fitted on normal history."""
import unittest

import numpy as np
import pandas as pd

from ml.experiments.behavior.jump import GainRule, history


class ScoreGainTests(unittest.TestCase):
    def test_equal_times_other_variants_and_future_scores_are_excluded(self):
        frame = pd.DataFrame({"user_id": ["u"] * 4, "quiz_id": ["q"] * 4, "variant_id": ["v", "v", "other", "v"],
                              "submitted_at": [10, 10, 12, 15], "score_percent": [50., 70., 100., 80.]})
        result = history(frame)
        self.assertTrue(result.iloc[:3].gain.isna().all())
        self.assertEqual(result.iloc[3].history_n, 2)
        self.assertEqual(result.iloc[3].gain, 20.)

    def test_subtracts_normal_sitting_gain_and_abstains_without_history(self):
        train = pd.DataFrame({"prior": [40.] * 4, "history_n": [2.] * 4, "gain": [5., 10., 10., 15.]})
        rule = GainRule.fit(train, minimum=3)
        test = pd.DataFrame({"prior": [40., 40., np.nan], "history_n": [2., 3., np.nan],
                             "gain": [10., 25., np.nan]})
        scores = rule.score(test)
        np.testing.assert_equal(scores, [0., 3., np.nan])

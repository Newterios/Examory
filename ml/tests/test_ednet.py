# ml/tests/test_ednet.py
"""EdNet pipeline checks on a synthetic KT2-like fixture (no real data needed)."""
from __future__ import annotations

import csv
import json
import random
import tempfile
import unittest
import zipfile
from pathlib import Path

import numpy as np
from scipy.special import expit

from ml.anomaly.features import read_attempts
from ml.ednet.common import (
    change_events,
    find_windows,
    is_calibration_user,
    load_table,
    subset,
)
from ml.ednet.convert import answer_digest, convert, event_identifier
from ml.ednet.extract import extract, load_questions, parse_user
from ml.ednet.models import (
    bias_correct_irt,
    fit_beta_binomial,
    fit_irt,
    fit_lognormal_rt,
    noise_corrected_sd,
)
from ml.ednet.simulate import simulate_windows

HEADER = "timestamp,action_type,item_id,source,user_answer,platform"
N_P5 = 60


def write_contents(path: Path) -> None:
    rows = ["question_id,bundle_id,explanation_id,correct_answer,part,tags,deployed_at"]
    for q in range(1, N_P5 + 1):
        rows.append(f"q{q},b{q},e{q},{'abcd'[q % 4]},5,1;2,1558093217098")
    rows.append("q100,b100,e100,a,5,1,1558093217098")  # multi-question Part-5 bundle -> dropped
    rows.append("q101,b100,e101,b,5,1,1558093217098")
    for q in range(200, 210):
        rows.append(f"q{q},b{q},e{q},a,1,1,1558093217098")
    with zipfile.ZipFile(path, "w") as archive:
        archive.writestr("contents/questions.csv", "\n".join(rows) + "\n")
        archive.writestr("__MACOSX/contents/._questions.csv", "junk")


def user_file(user: int, rng: random.Random) -> str:
    """45 Part-5 bundles in one sitting (+ a long break), with changes, re-taps and noise."""
    lines, ts = [HEADER], 1_550_000_000_000 + user * 10_000_000
    questions = rng.sample(range(1, N_P5 + 1), 45)
    for k, q in enumerate(questions):
        if k == 30:
            ts += 400_000  # gap above the default max gap -> new sitting
        if k == 10:  # an interleaved Part-1 bundle (invisible inside the gap)
            lines += [f"{ts},enter,b200,sprint,,mobile", f"{ts + 3000},respond,q200,sprint,a,mobile",
                      f"{ts + 4000},submit,b200,sprint,,mobile"]
            ts += 5000
        if k == 20:  # a malformed Part-5 bundle: enter without submit
            lines.append(f"{ts},enter,b{questions[-1]},sprint,,mobile")
            ts += 2000
        lines.append(f"{ts},enter,b{q},sprint,,mobile")
        ts += rng.randint(3000, 40000)
        option = rng.choice("abcd")
        lines.append(f"{ts},respond,q{q},sprint,{option},mobile")
        if rng.random() < .3:  # re-tap of the same option, then maybe a real change
            ts += rng.randint(500, 3000)
            lines.append(f"{ts},respond,q{q},sprint,{option},mobile")
            if rng.random() < .5:
                ts += rng.randint(500, 3000)
                lines.append(f"{ts},respond,q{q},sprint,{rng.choice([o for o in 'abcd' if o != option])},mobile")
        ts += rng.randint(300, 4000)
        lines.append(f"{ts},submit,b{q},sprint,,mobile")
        ts += rng.randint(100, 20000)
    return "\n".join(lines) + "\n"


class EdNetPipelineTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.temporary = tempfile.TemporaryDirectory(prefix="examory-ednet-")
        root = cls.root = Path(cls.temporary.name)
        write_contents(root / "contents.zip")
        rng = random.Random(7)
        with zipfile.ZipFile(root / "kt2.zip", "w", zipfile.ZIP_DEFLATED) as archive:
            archive.writestr("KT2/", "")
            archive.writestr("__MACOSX/._KT2", "junk")
            for user in range(1, 61):
                archive.writestr(f"KT2/u{user}.csv", user_file(user, rng))
        cls.table_path = root / "derived" / "table.npz"
        cls.extract_meta = extract(root / "kt2.zip", root / "contents.zip", cls.table_path, workers=2)
        cls.table = load_table(cls.table_path)
        cls.csv = root / "events.csv"
        cls.convert_meta = convert(cls.table_path, cls.csv, users=1000)

    @classmethod
    def tearDownClass(cls) -> None:
        cls.temporary.cleanup()

    def test_extract_counts_and_structure(self) -> None:
        meta = self.extract_meta
        self.assertEqual(meta["user_files_read"], 60)
        self.assertEqual(meta["questions"]["part5_multi_question_bundles_dropped"], 1)
        self.assertEqual(meta["malformed_part5_bundles_dropped"]["missing_submit"], 60)
        self.assertEqual(meta["responses"], 60 * 45)
        t = self.table
        self.assertTrue(np.all(np.diff(t["user"]) >= 0))
        self.assertTrue(np.all(t["first_answer_time_ms"] > 0))
        self.assertTrue(np.all(t["submit_ts"] >= t["final_respond_ts"]))
        questions = load_questions(self.root / "contents.zip")
        keys = np.array(["abcd".index("abcd"[q % 4]) for q in t["question"]])
        np.testing.assert_array_equal(t["first_correct"], t["first_answer"] == keys)
        self.assertNotIn("b100", questions["bundle_to_question"])

    def test_parse_user_malformed(self) -> None:
        text = f"{HEADER}\n1,enter,b1,sprint,,web\n2,respond,q2,sprint,a,web\n3,submit,b1,sprint,,web\n4,enter,b2,sprint,,web\n5,submit,b2,sprint,,web\n6,respond,q2,sprint,a,web\n7,enter,b3,sprint,,web\n9,respond,q3,sprint,a,web\n8,submit,b3,sprint,,web\n10,enter,b4,sprint,,web\n11,respond,q4,sprint,a,web\n12,submit,b4,sprint,,web"
        rows, _, _, bad = parse_user(text, 1, {"b1": 1, "b2": 2, "b3": 3, "b4": 4})
        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0][1], 4)
        self.assertEqual(bad["foreign_respond"], 1)
        self.assertEqual(bad["no_respond"], 1)
        self.assertEqual(bad["non_monotone_ts"], 1)
        self.assertEqual(bad["orphan_respond_line_any_part"], 1)

    def test_converted_csv_passes_validator(self) -> None:
        attempts = read_attempts(self.csv)
        self.assertEqual(len(attempts), self.convert_meta["attempts"])
        self.assertGreater(len(attempts), 5)
        for attempt in attempts:
            self.assertEqual(len(attempt["first_times"]), 20)
            self.assertEqual(attempt["is_anomaly"], 0)
            self.assertEqual(attempt["quiz_id"], "ednet_part5")
        groups: dict[str, str] = {}
        for attempt in attempts:  # a user stays in one cohort
            self.assertEqual(groups.setdefault(attempt["user_id"], attempt["group_id"]), attempt["group_id"])
        users = np.array([int(a["user_id"]) for a in attempts])
        self.assertFalse(is_calibration_user(users).any())

    def test_window_rules(self) -> None:
        t = subset(self.table, ~self.table["is_calibration"])
        windows = find_windows(t, 300_000)
        rows = windows["rows"]
        self.assertTrue(len(rows) > 0)
        gaps = t["enter_ts"][rows[:, 1:]] - t["submit_ts"][rows[:, :-1]]
        self.assertTrue(np.all((gaps >= 0) & (gaps <= 300_000)))
        self.assertTrue(np.all(t["user"][rows] == t["user"][rows[:, :1]]))
        # 30 responses before the long break, 15 after: one window per user at 300 s
        self.assertTrue(np.all(np.bincount(np.unique(windows["user"], return_inverse=True)[1]) == 1))
        # a larger max gap merges the two sittings: 45 responses -> 2 windows
        self.assertTrue(np.all(np.bincount(np.unique(find_windows(t, 500_000)["user"],
                                                     return_inverse=True)[1]) == 2))

    def test_change_semantics(self) -> None:
        table = {"resp_start": np.array([0]), "n_resp": np.array([5]),
                 "resp_ts": np.array([10, 20, 30, 40, 50]), "resp_ans": np.array([0, 0, 1, 1, 2])}
        self.assertEqual(change_events(table, 0), [(10, 0), (30, 1), (50, 2)])
        # extract's n_changes counts the same events, and the CSV's answers_changed agrees
        t = self.table
        for row in range(len(t["user"])):
            self.assertEqual(len(change_events(t, row)) - 1, t["n_changes"][row])
        retaps = (t["n_resp"] > 1) & (t["n_changes"] == 0)
        self.assertTrue(retaps.any())
        with self.csv.open(encoding="utf-8") as handle:
            records = list(csv.DictReader(handle))
        answered = [json.loads(r["payload"]) for r in records if r["event_type"] == "quiz.answered"]
        submitted = [json.loads(r["payload"]) for r in records if r["event_type"] == "quiz.submitted"]
        self.assertEqual(sum(a["is_change"] for a in answered), sum(s["answers_changed"] for s in submitted))
        self.assertEqual(sum(s["answers_changed"] for s in submitted), self.convert_meta["change_events"])
        self.assertTrue({a["answer_hash"] for a in answered} <= {answer_digest(o) for o in range(4)})
        event = event_identifier(1_550_000_000_123, 1, 2, "x")
        self.assertEqual(int(event.replace("-", ""), 16) >> 80, 1_550_000_000_123)

    def test_answer_digest_matches_generator(self) -> None:
        from ml.datasets.generate_quiz_behavior import answer_digest as reference
        for option in range(4):
            self.assertEqual(answer_digest(option), reference(option))

    def test_hash_split(self) -> None:
        users = np.arange(1, 20001)
        flags = is_calibration_user(users)
        self.assertTrue(np.array_equal(flags, is_calibration_user(users)))  # deterministic
        self.assertAlmostEqual(flags.mean(), .5, delta=.02)
        calibration, evaluation = set(users[flags]), set(users[~flags])
        self.assertFalse(calibration & evaluation)
        np.testing.assert_array_equal(self.table["is_calibration"], is_calibration_user(self.table["user"]))


class EstimatorRecoveryTests(unittest.TestCase):
    def test_irt_parameter_recovery(self) -> None:
        rng = np.random.default_rng(3)
        persons, items, c = 2500, 80, .2
        theta = rng.normal(0, 1, persons)
        a = np.exp(rng.normal(0, .25, items))
        b = rng.normal(0, 1, items)
        person = np.repeat(np.arange(persons), 60)
        item = np.concatenate([rng.choice(items, 60, replace=False) for _ in range(persons)])
        y = (rng.random(len(person)) < c + (1 - c) * expit(a[item] * (theta[person] - b[item]))).astype(float)
        fit = fit_irt(person, item, y, c)
        self.assertTrue(fit["converged"])
        self.assertGreater(np.corrcoef(fit["b"], b)[0, 1], .97)
        self.assertGreater(np.corrcoef(fit["a"], a)[0, 1], .7)
        self.assertGreater(np.corrcoef(fit["theta"], theta)[0, 1], .9)
        self.assertLess(np.mean(np.abs(fit["b"] - b)), .25)
        self.assertLess(abs(np.median(fit["a"] / a) - 1), .25)  # raw JML inflates a
        corrected = bias_correct_irt(fit, person, item, rng, reps=4)
        self.assertLess(abs(np.median(corrected["a"] / a) - 1), .07)
        self.assertLess(np.mean(np.abs(corrected["b"] - b)), .2)
        self.assertGreater(np.corrcoef(corrected["b"], b)[0, 1], .97)
        latent_sd = np.sqrt(fit["theta"].var() + np.mean(fit["theta_se"] ** 2))
        self.assertAlmostEqual(latent_sd, 1.0, delta=.03)
        # Profile likelihood prefers the true c over a clearly wrong one.
        wrong = fit_irt(person, item, y, 0.0, init=fit)
        self.assertGreater(fit["log_posterior"], wrong["log_posterior"])

    def test_lognormal_rt_recovery(self) -> None:
        rng = np.random.default_rng(4)
        persons, items = 1500, 50
        tau = rng.normal(0, .35, persons)
        beta = rng.normal(10, .4, items)
        sigma = np.exp(rng.normal(-.5, .15, items))
        person = np.repeat(np.arange(persons), 30)
        item = np.concatenate([rng.choice(items, 30, replace=False) for _ in range(persons)])
        log_t = beta[item] - tau[person] + sigma[item] * rng.standard_normal(len(person))
        fit = fit_lognormal_rt(person, item, log_t)
        shift = np.mean(tau)  # identification: mean(tau_hat) = 0
        self.assertLess(np.max(np.abs(fit["beta"] - (beta - shift))), .08)  # ~4 SE
        self.assertLess(np.max(np.abs(fit["sigma"] / sigma - 1)), .1)
        self.assertGreater(np.corrcoef(fit["tau"], tau)[0, 1], .9)
        self.assertAlmostEqual(noise_corrected_sd(fit["tau"], fit["tau_se"]), .35, delta=.02)

    def test_beta_binomial_recovery(self) -> None:
        rng = np.random.default_rng(5)
        n = rng.integers(20, 200, 20000)
        rate = np.where(rng.random(len(n)) < .6, 0.0, rng.beta(1.2, 12, len(n)))
        k = rng.binomial(n, rate)
        fit = fit_beta_binomial(k, n, zero_inflated=True)
        self.assertAlmostEqual(fit["p_zero"], .6, delta=.03)
        self.assertAlmostEqual(fit["beta_a"] / (fit["beta_a"] + fit["beta_b"]), 1.2 / 13.2, delta=.01)
        self.assertAlmostEqual(fit["beta_a"], 1.2, delta=.25)
        plain = fit_beta_binomial(rng.binomial(n, rng.beta(1.5, 20, len(n))), n)
        self.assertAlmostEqual(plain["beta_a"], 1.5, delta=.2)
        self.assertAlmostEqual(plain["beta_b"], 20, delta=3)
        self.assertEqual(plain["p_zero"], 0.0)

    def test_simulator_smoke(self) -> None:
        rng = np.random.default_rng(6)
        params = json.loads(Path("sim/params/default.json").read_text(encoding="utf-8"))
        items = 30
        model = {"person": params["person"], "a": np.ones(items), "b": np.zeros(items), "c": .2,
                 "beta": np.full(items, 10.0), "sigma": np.full(items, .5),
                 "distractor_weights": np.tile([.5, .3, .2], (items, 1)), "rapid_guess": params["rapid_guess"],
                 "revision": params["revision"], "gap": params["gap"], "review": params["review"],
                 "min_response_ms": params["min_response_ms"]}
        windows = np.stack([rng.choice(items, 20, replace=False) for _ in range(400)])
        out = simulate_windows(model, windows, np.arange(400) // 2, np.arange(400) % 2, rng)
        self.assertEqual(out["first_times"].shape, (400, 20))
        self.assertTrue(np.all(out["first_times"] >= params["min_response_ms"]))
        self.assertTrue(.4 < out["first_correct"].mean() < .75)
        self.assertTrue(np.all(out["duration"] > out["first_times"].sum(axis=1)))
        self.assertTrue(np.all(out["n_changes"][out["rapid"]] == 0))


if __name__ == "__main__":
    unittest.main()

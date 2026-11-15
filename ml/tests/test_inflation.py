"""Tests for the feature-injection inflation harness (ml/experiments/inflation).

Fast: no real datasets. The simulator is replaced by a stub that copies legacy
generator output into the simulator's file contract (honest/fi/bl/truth/manifest).
"""
from __future__ import annotations

import csv
import json
import shutil
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

import numpy as np
from scipy import stats as scipy_stats
from sklearn.metrics import roc_auc_score

from ml.anomaly.evaluate import metrics as frozen_metrics
from ml.anomaly.train import choose_threshold
from ml.experiments.inflation import analyze as analyze_module
from ml.experiments.inflation import review, similarity, supervised
from ml.experiments.inflation import run as run_module
from ml.experiments.inflation.common import (
    BL_TYPES,
    FI_TYPES,
    SLOT_LABEL,
    SLOT_OF_TYPE,
    SLOTS,
    read_json,
    strip_timing,
)
from ml.experiments.inflation.compact import expand_stage, slim_stage
from ml.experiments.inflation.data import IntegrityError
from ml.experiments.inflation.metrics import (
    at_threshold,
    average_precision,
    cohort_bootstrap,
    roc_auc,
    standardized_pauc,
    tpr_on_grid,
)
from ml.experiments.inflation.stats import (
    holm,
    latin_hypercube,
    mean_ci,
    paired_difference,
    scale_unit,
    unpaired_difference,
)

REPO = Path(__file__).resolve().parents[2]

STUB = textwrap.dedent('''\
    #!{python}
    """Test stub of examory-sim: legacy CSV copied into the simulator contract."""
    import argparse, csv, hashlib, json, subprocess, sys
    from pathlib import Path
    FI = ("rapid_response", "excessive_changes", "collusion", "score_jump")
    BL = ("preknowledge", "lookup", "copy_peer", "proxy")
    p = argparse.ArgumentParser()
    p.add_argument("command")
    for name in ("--students", "--attempts", "--questions", "--seed"):
        p.add_argument(name, type=int)
    p.add_argument("--anomaly-rate", type=float)
    for name in ("--params", "--mechanisms", "--item-sampling", "--worlds", "--out-dir"):
        p.add_argument(name)
    p.add_argument("--intensity", type=float)
    p.add_argument("--force", action="store_true")
    a = p.parse_args()
    out = Path(a.out_dir); out.mkdir(parents=True, exist_ok=True)
    legacy = out / "_legacy.csv"
    subprocess.run([sys.executable, "{generator}", "--students", str(a.students), "--attempts",
                    str(a.attempts), "--questions", str(a.questions), "--anomaly-rate", str(a.anomaly_rate),
                    "--seed", str(a.seed), "--output", str(legacy)], check=True, capture_output=True)
    rows = list(csv.DictReader(legacy.open(encoding="utf-8", newline="")))
    header = list(rows[0])
    worlds = a.worlds.split(",")
    truth = []
    for world in worlds:
        with (out / f"{{world}}.csv").open("w", encoding="utf-8", newline="") as h:
            w = csv.DictWriter(h, header); w.writeheader()
            for r in rows:
                r = dict(r)
                if world == "honest":
                    r["is_anomaly"], r["anomaly_type"] = "0", "none"
                elif world == "bl" and r["anomaly_type"] != "none":
                    r["anomaly_type"] = BL[FI.index(r["anomaly_type"])]
                w.writerow(r)
                if r["event_type"] == "quiz.submitted":
                    actor, ctx, pay = (json.loads(r[k]) for k in ("actor", "context", "payload"))
                    mech = r["anomaly_type"]
                    flag = {{"preknowledge": "naive", "lookup": "verify", "copy_peer": "copier"}}.get(mech, "")
                    if mech == "copy_peer" and int(actor["user_id"][-1]) % 2:
                        flag = "source"
                    truth.append({{"world": world, "attempt_id": pay["attempt_id"], "user_id": actor["user_id"],
                                  "group_id": ctx["group_id"], "slot": mech, "mechanism": mech,
                                  "role": flag if mech == "copy_peer" else "", "partner_user_id": "",
                                  "theta": "0", "tau": "0", "mech_flag": flag}})
    with (out / "truth.csv").open("w", encoding="utf-8", newline="") as h:
        w = csv.DictWriter(h, list(truth[0])); w.writeheader(); w.writerows(truth)
    legacy.unlink()
    files = {{f"{{w}}.csv": hashlib.sha256((out / f"{{w}}.csv").read_bytes()).hexdigest() for w in worlds}}
    files["truth.csv"] = hashlib.sha256((out / "truth.csv").read_bytes()).hexdigest()
    (out / "manifest.json").write_text(json.dumps({{"files": [{{"file": k, "sha256": v}} for k, v in files.items()]}}))
''')


class MetricTests(unittest.TestCase):
    def test_threshold_ties_and_counts(self) -> None:
        labels = np.array([0, 0, 1, 1])
        scores = np.array([0.1, 0.5, 0.5, 0.9])
        row = at_threshold(labels, scores, 0.5)  # >= : both tied scores are flagged
        self.assertEqual((row["tp"], row["fp"], row["fn"], row["tn"]), (2, 1, 0, 1))
        self.assertEqual(row["recall"], 1.0)
        self.assertEqual(row["fpr"], 0.5)
        self.assertAlmostEqual(row["precision"], 2 / 3)
        self.assertAlmostEqual(row["f1"], 0.8)
        self.assertEqual(row["alerts_per_100"], 75.0)

    def test_no_positives(self) -> None:
        labels = np.zeros(5, dtype=int)
        scores = np.arange(5.0)
        row = at_threshold(labels, scores, 3)
        self.assertIsNone(row["recall"])
        self.assertIsNone(row["precision"] if row["tp"] + row["fp"] == 0 else None)
        self.assertEqual(row["fpr"], 0.4)
        self.assertIsNone(roc_auc(labels, scores))
        self.assertIsNone(standardized_pauc(labels, scores))
        self.assertIsNone(average_precision(labels, scores))
        self.assertIsNone(tpr_on_grid(labels, scores, np.array([0.0, 0.05])))

    def test_matches_frozen_metric(self) -> None:
        rng = np.random.default_rng(1)
        labels = (rng.random(300) < 0.2).astype(int)
        scores = np.round(rng.random(300) + labels * 0.3, 2)
        ours, theirs = at_threshold(labels, scores, 0.7), frozen_metrics(labels, scores, 0.7)
        for key in ("tp", "fp", "fn", "tn", "precision", "recall", "fpr", "alerts_per_100"):
            self.assertAlmostEqual(ours[key], theirs[key])

    def test_auc_and_pauc_against_sklearn(self) -> None:
        rng = np.random.default_rng(2)
        for _ in range(5):
            labels = (rng.random(400) < 0.3).astype(int)
            scores = np.round(rng.normal(size=400) + labels, 1)  # many ties
            self.assertAlmostEqual(roc_auc(labels, scores), roc_auc_score(labels, scores), places=12)
            for max_fpr in (0.05, 0.2):
                self.assertAlmostEqual(standardized_pauc(labels, scores, max_fpr),
                                       roc_auc_score(labels, scores, max_fpr=max_fpr), places=12)

    def test_pauc_perfect_and_random(self) -> None:
        labels = np.array([0] * 50 + [1] * 10)
        perfect = np.r_[np.zeros(50), np.ones(10)]
        self.assertEqual(standardized_pauc(labels, perfect), 1.0)
        self.assertEqual(roc_auc(labels, perfect), 1.0)
        self.assertEqual(standardized_pauc(labels, np.ones(60)), 0.5)  # all ties: chance diagonal
        self.assertLess(standardized_pauc(labels, -perfect), 0.5)
        rng = np.random.default_rng(3)
        labels = (rng.random(200_000) < 0.5).astype(int)
        self.assertAlmostEqual(standardized_pauc(labels, rng.random(200_000)), 0.5, delta=0.01)
        self.assertEqual(tpr_on_grid(np.array([0, 0, 1]), np.array([0.0, 0.1, 1.0]), np.array([0.0, 0.5])),
                         [1.0, 1.0])

    def test_cohort_bootstrap(self) -> None:
        groups = np.repeat(np.arange(10), 20)
        labels = np.tile([0] * 15 + [1] * 5, 10)
        predicted = np.tile([1] + [0] * 14 + [1, 1, 1, 0, 0], 10).astype(bool)
        result = cohort_bootstrap(groups, labels, predicted, 200, 0)
        self.assertEqual(result["recall"]["low"], 0.6)  # identical cohorts: degenerate interval
        self.assertEqual(result["recall"]["high"], 0.6)
        self.assertIsNone(cohort_bootstrap(np.zeros(5), np.zeros(5), np.zeros(5), 10, 0)["recall"])


class StatsTests(unittest.TestCase):
    def test_paired_difference(self) -> None:
        x = [0.5, 0.6, 0.7, 0.8, 0.9, 0.75]
        y = [0.4, 0.45, 0.72, 0.6, 0.5, 0.7]
        result = paired_difference(x, y, repeats=2000, seed=0)
        diff = np.subtract(x, y)
        self.assertAlmostEqual(result["mean"], diff.mean())
        self.assertLessEqual(result["low"], result["mean"])
        self.assertGreaterEqual(result["high"], result["mean"])
        self.assertAlmostEqual(result["p_wilcoxon"], scipy_stats.wilcoxon(diff).pvalue)
        # exact sign-flip: enumerate all 2^6 patterns by brute force
        import itertools
        sums = [abs(sum(s * d for s, d in zip(signs, diff, strict=True)))
                for signs in itertools.product((-1, 1), repeat=len(diff))]
        self.assertAlmostEqual(result["p"], np.mean(np.array(sums) >= abs(diff.sum()) - 1e-12))
        self.assertEqual(result["test"], "sign-flip")
        self.assertAlmostEqual(result["mcse"], diff.std(ddof=1) / np.sqrt(len(diff)))
        self.assertEqual(paired_difference([0.1, 0.2], [0.1, 0.2])["p"], 1.0)
        self.assertEqual(paired_difference([0.1, None], [0.2, 0.3])["n"], 1)
        self.assertEqual(paired_difference(x, y, seed=5), paired_difference(x, y, seed=5))
        with self.assertRaises(ValueError):
            paired_difference([1.0], [1.0, 2.0])

    def test_unpaired_and_mean(self) -> None:
        result = unpaired_difference([1.0, 2.0, 3.0], [0.0, 0.5])
        self.assertAlmostEqual(result["mean"], 1.75)
        self.assertAlmostEqual(result["p"], scipy_stats.mannwhitneyu([1, 2, 3], [0, 0.5]).pvalue)
        summary = mean_ci([0.2, 0.4, None])
        self.assertEqual(summary["n"], 2)
        self.assertAlmostEqual(summary["mean"], 0.3)
        self.assertIsNone(mean_ci([None])["mean"])

    def test_holm(self) -> None:
        self.assertEqual(holm([0.01, 0.04, 0.03]), [0.03, 0.06, 0.06])
        self.assertEqual(holm([0.5, None, 0.01]), [0.5, None, 0.02])
        self.assertEqual(holm([0.6, 0.7]), [1.0, 1.0])
        self.assertEqual(holm([]), [])

    def test_latin_hypercube(self) -> None:
        unit = latin_hypercube(40, 9, 11)
        self.assertEqual(unit.shape, (40, 9))
        for column in unit.T:
            self.assertEqual(sorted(np.floor(column * 40).astype(int)), list(range(40)))
        np.testing.assert_array_equal(unit, latin_hypercube(40, 9, 11))
        scaled = scale_unit(unit, [(0, 1, False), (0.3, 0.9, False), (10, 90, True)] + [(0, 1, False)] * 6)
        self.assertTrue(np.all((scaled[:, 1] >= 0.3) & (scaled[:, 1] < 0.9)))
        self.assertTrue(np.all((scaled[:, 2] >= 10) & (scaled[:, 2] < 90)))
        configs = run_module.sensitivity_configs(40)
        for name, (low, high, _) in run_module.SENSITIVITY_PARAMETERS:
            values = [c[name] for c in configs]
            self.assertTrue(all(low <= v <= high for v in values), name)

    def test_write_mechanisms(self) -> None:
        values = run_module.sensitivity_configs(3)[0]
        with tempfile.TemporaryDirectory() as directory:
            path = run_module.write_mechanisms(values, REPO / "sim/params/mechanisms.json",
                                               Path(directory) / "m.json")
            written = read_json(path)
        self.assertAlmostEqual(written["preknowledge"]["speedup_low"], 0.4 * values["preknowledge.speedup_high"])
        self.assertAlmostEqual(np.exp(written["lookup"]["duration_log_ms_mean"]) / 1000,
                               values["lookup.duration_median_s"])
        self.assertAlmostEqual(np.exp(written["copy_peer"]["start_lag_log_ms_mean"]) / 1000,
                               values["copy_peer.start_lag_median_s"])
        self.assertEqual(written["fi"], read_json(REPO / "sim/params/mechanisms.json")["fi"])

    def test_slot_mapping(self) -> None:
        expected = {"rapid_response": "preknowledge", "excessive_changes": "lookup",
                    "collusion": "copy_peer", "score_jump": "proxy"}
        for fi, bl in expected.items():
            self.assertEqual(SLOT_OF_TYPE[fi], SLOT_OF_TYPE[bl])
            self.assertEqual(BL_TYPES[FI_TYPES.index(fi)], bl)
        self.assertEqual(SLOTS, ("S1", "S2", "S3", "S4"))
        self.assertEqual(SLOT_LABEL["S3"], "collusion↔copy_peer")


class EndToEndTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.temporary = tempfile.TemporaryDirectory(prefix="examory-inflation-")
        cls.root = Path(cls.temporary.name)
        cls.stub = cls.root / "examory-sim"
        cls.stub.write_text(STUB.format(python=sys.executable, generator=REPO / "ml/datasets/generate_quiz_behavior.py"))
        cls.stub.chmod(0o755)

    @classmethod
    def tearDownClass(cls) -> None:
        cls.temporary.cleanup()

    def stage(self, name: str, exp: Path, stages: Path, *extra: str, seeds=("1", "2")) -> dict:
        argv = [name, "--students", "200", "--seeds", *seeds, "--workers", "1", "--exp-root", str(exp),
                "--output", str(stages / f"{name.replace('-', '_')}.json"), *extra]
        return run_module.main(argv)

    def test_legacy_end_to_end_and_determinism(self) -> None:
        outputs = []
        for repeat in range(2):
            exp, stages, out = self.root / "legacy_exp", self.root / f"stages{repeat}", self.root / f"out{repeat}"
            shutil.rmtree(exp, ignore_errors=True)
            result = self.stage("main", exp, stages, "--conditions", "LEGACY")
            summary = analyze_module.analyze(stages, out, figures=repeat == 0)
            outputs.append((result, summary))
        result, summary = outputs[0]
        self.assertEqual(len(result["runs"]), 2)
        for run in result["runs"]:
            self.assertEqual(run["metrics"]["iforest"]["all"]["n"], run["split_counts"]["test"]["attempts"])
            for detector in ("iforest", "iforest_no_peers", "quantile_rules"):
                self.assertLessEqual(run["operating_points"][detector]["0.05"]["validation_fpr"], 0.05 + 1e-12)
            self.assertTrue(set(run["metrics"]["iforest"]) >= {"all", "final", "slot:S1", "slot:S3"})
        self.assertIn("h1", summary)
        self.assertTrue((self.root / "out0/tables/main.md").exists())
        self.assertTrue((self.root / "out0/figures/F1_recall_by_slot.png").exists())

        def normalize(value):
            value = strip_timing(value)
            value.pop("arguments", None)
            value.pop("stages", None)
            value.pop("tables", None)
            value.pop("figures", None)
            return json.loads(json.dumps(value))
        self.assertEqual(normalize(outputs[0][0]), normalize(outputs[1][0]))
        first = read_json(self.root / "out0/summary.json")
        second = read_json(self.root / "out1/summary.json")
        self.assertEqual(normalize(first), normalize(second))

    def test_simulator_contract_with_stub(self) -> None:
        exp, stages, out = self.root / "sim_exp", self.root / "sim_stages", self.root / "sim_out"
        sim = ("--sim-binary", str(self.stub))
        main = self.stage("main", exp, stages, *sim, seeds=("3",))
        # Generated datasets are deleted once scores are cached (default) ...
        self.assertEqual(list((exp / "data").glob("*/*/*.csv")), [])
        manipulation = next(r for r in main["runs"] if r["condition"] == "SIM-BL")["manipulation"]
        self.assertIn("copy_peer", manipulation)  # stub: cheater == counterfactual, so all deltas are 0
        self.assertEqual(manipulation["copy_peer"]["score_points"], 0.0)
        # ... and kept with --keep-data (used below for the tamper check).
        main = self.stage("main", exp, stages, *sim, "--keep-data", seeds=("3",))
        conditions = {r["condition"] for r in main["runs"]}
        self.assertEqual(conditions, {"LEGACY", "SIM-FI", "SIM-BL"})
        by = {r["condition"]: r for r in main["runs"]}
        # The stub copies legacy data, so SIM-FI equals LEGACY exactly and SIM-BL equals it per slot.
        for slot in SLOTS:
            self.assertEqual(by["SIM-FI"]["metrics"]["iforest"][f"slot:{slot}"]["at_0.05"],
                             by["SIM-BL"]["metrics"]["iforest"][f"slot:{slot}"]["at_0.05"])
        self.assertTrue(any(k.startswith("truth:copy_peer:role=") for k in by["SIM-BL"]["metrics"]["iforest"]))
        transfer = self.stage("transfer", exp, stages, *sim, seeds=("3",))
        pairs = {(r["source"], r["target"]) for r in transfer["runs"]}
        self.assertIn(("LEGACY", "SIM-BL"), pairs)
        diagonal = next(r for r in transfer["runs"] if r["source"] == r["target"] == "SIM-BL")
        self.assertEqual(diagonal["metrics"]["iforest"]["all"]["at_0.05"], by["SIM-BL"]["metrics"]["iforest"]["all"]["at_0.05"])
        robust = self.stage("robust-detectors", exp, stages, *sim, "--conditions", "SIM-BL", seeds=("3",))
        self.assertEqual(set(robust["runs"][0]["metrics"]), {"lof", "lof_no_peers", "ocsvm", "ocsvm_no_peers",
                                                              "mcd", "mcd_no_peers"})
        cached = self.stage("main", exp, self.root / "sim_stages_cache", *sim, "--from-cache", seeds=("3",))
        self.assertEqual(strip_timing(cached["runs"])[0]["metrics"], strip_timing(main["runs"])[0]["metrics"])
        summary = analyze_module.analyze(stages, out, figures=False)
        self.assertIn("SIM-BL", summary["h1"])
        self.assertTrue(summary["inflation"])
        self.assertTrue(summary["manipulation"])
        committed = read_json(stages / "main.json")
        self.assertIn("format", committed)  # compact columnar file
        self.assertIn("roc", committed)
        expanded = expand_stage(committed)["runs"]
        for full, restored in zip(main["runs"], expanded, strict=True):
            for subset in ("all", "slot:S3"):
                for path in (("at_0.05", "recall"), ("at_0.05", "fpr"), ("pauc_std",)):
                    a, b = full["metrics"]["iforest"][subset], restored["metrics"]["iforest"][subset]
                    for key in path:
                        a, b = a[key], b[key]
                    self.assertAlmostEqual(a, b, places=5)
        self.assertEqual(slim_stage({"stage": "ednet", "runs": []}), {"stage": "ednet", "runs": []})
        # Tampering with a generated CSV must be detected on reuse.
        self.stage("main", exp, self.root / "sim_stages_keep", *sim, "--keep-data", seeds=("3",))
        csv_path = Path(by["SIM-BL"]["data"]["csv"])
        csv_path = csv_path if csv_path.is_absolute() else REPO / csv_path
        with csv_path.open(encoding="utf-8", newline="") as handle:
            rows = list(csv.reader(handle))
        with csv_path.open("w", encoding="utf-8", newline="") as handle:
            csv.writer(handle).writerows(rows[:-1])
        with self.assertRaises(IntegrityError):
            self.stage("main", exp, self.root / "sim_stages_bad", *sim, seeds=("3",))


def _attempt(attempt_id: str, user: str, group: str, answers: list[str], score: float | None = None) -> dict:
    hashes = {f"q{k:02d}": a for k, a in enumerate(answers)}
    return {"attempt_id": attempt_id, "user_id": user, "group_id": group, "quiz_id": "quiz_001",
            "variant_id": "variant_001", "final_hashes": hashes,
            "score_percent": 0.0 if score is None else score}


class SimilarityTests(unittest.TestCase):
    def test_null_probability(self) -> None:
        train = [_attempt("t1", "u1", "g", ["a", "a"]), _attempt("t2", "u2", "g", ["b", "a"]),
                 _attempt("t3", "u3", "g", ["a", "a"]), _attempt("t4", "u4", "g", ["c", "a"])]
        p = similarity.null_match_probability(similarity.option_frequencies(train))
        key = lambda q: json.dumps(["quiz_001", "variant_001", q])
        self.assertAlmostEqual(p[key("q00")], 0.5 ** 2 + 0.25 ** 2 + 0.25 ** 2)
        self.assertAlmostEqual(p[key("q01")], 1.0)

    def test_z_statistic_hand_computed(self) -> None:
        # Train frequencies: options a/b at 50 % on each of 10 items -> p_k = 0.5, sum p = 5, sum p(1-p) = 2.5.
        train = [_attempt("t1", "u1", "g1", ["a"] * 10), _attempt("t2", "u2", "g1", ["b"] * 10)]
        null = similarity.null_match_probability(similarity.option_frequencies(train))
        i = _attempt("i", "s1", "g2", ["b"] * 4 + ["a"] * 6)         # wrong (key a) on items 0-3
        j = _attempt("j", "s2", "g2", ["b"] * 4 + ["a"] * 4 + ["b"] * 2)   # matches i on items 0-7
        same_user = _attempt("i2", "s1", "g2", ["b"] * 4 + ["a"] * 6)  # identical, same student: not a partner
        alone = _attempt("k", "s3", "g3", ["a"] * 10)                 # no other student in its cohort
        short = _attempt("m", "s4", "g4", ["a"] * 9)
        short_partner = _attempt("n", "s5", "g4", ["a"] * 9)          # |S| = 9 < 10
        attempts = [i, j, same_user, alone, short, short_partner]
        keys = {json.dumps(["quiz_001", "variant_001", f"q{k:02d}"]): "a" for k in range(10)}
        result = similarity.answer_similarity(attempts, null, keys)
        z = (8 - 5) / np.sqrt(2.5)
        scores = result["scores"]["answersim"]
        self.assertAlmostEqual(scores[0], z)
        self.assertAlmostEqual(scores[1], z)
        self.assertAlmostEqual(scores[2], z)   # i2 pairs with j (different student), never with i
        self.assertEqual(scores[3], similarity.NO_PARTNER_SCORE)
        self.assertEqual(scores[4], similarity.NO_PARTNER_SCORE)
        self.assertEqual(scores[5], similarity.NO_PARTNER_SCORE)
        # Incorrect-only for i: items 0-3, all matched: (4 - 2) / sqrt(1) = 2.
        incorrect = result["scores"]["answersim_incorrect"]
        self.assertAlmostEqual(incorrect[0], 2.0)
        # j is wrong on items 0-3 and 8-9; matches i on 0-3 only: (4 - 3) / sqrt(1.5).
        self.assertAlmostEqual(incorrect[1], 1 / np.sqrt(1.5))
        self.assertEqual(incorrect[3], similarity.NO_PARTNER_SCORE)  # all correct: no wrong items
        self.assertEqual(list(result["eligible_partners"][:4]), [1, 2, 1, 0])
        self.assertAlmostEqual(similarity.pair_z(np.array(list("bbbbaaaaaa")), np.array(list("bbbbaaaabb")),
                                                 np.full(10, 0.5), np.ones(10, dtype=bool)), z)

    def test_infer_keys(self) -> None:
        attempts = [_attempt("a1", "u1", "g", ["a", "b", "c"], 100 * 3 / 3),
                    _attempt("a2", "u2", "g", ["a", "c", "c"], 100 * 2 / 3),
                    _attempt("a3", "u3", "g", ["d", "c", "b"], 0.0),
                    _attempt("a4", "u4", "g", ["a", "b", "b"], 100 * 2 / 3)]
        keys = similarity.infer_keys(attempts)
        self.assertEqual(sorted(keys.values()), ["a", "b", "c"])
        attempts[2]["score_percent"] = 100 / 3
        with self.assertRaises(IntegrityError):
            similarity.infer_keys(attempts)


class SupervisedTests(unittest.TestCase):
    @staticmethod
    def fixture(seed: int = 0):
        import pandas as pd

        from ml.anomaly.features import FEATURE_COLUMNS
        rng = np.random.default_rng(seed)
        n = 600
        frame = pd.DataFrame(rng.lognormal(size=(n, len(FEATURE_COLUMNS))), columns=FEATURE_COLUMNS)
        frame["history_missing"] = (rng.random(n) < 0.3).astype(int)
        frame["is_anomaly"] = (rng.random(n) < 0.15).astype(int)
        frame.loc[frame.is_anomaly == 1, "changes_count"] += 30.0  # signal in an attempt feature
        frame["split"] = np.repeat([0, 1, 2], n // 3)
        frame["truth_is_anomaly"] = frame.is_anomaly.astype(str)
        return frame

    def test_fit_threshold_and_determinism(self) -> None:
        frame = self.fixture()
        first = supervised.fit_supervised(frame, seed=11)
        second = supervised.fit_supervised(frame, seed=11)
        self.assertEqual(set(first), set(supervised.DETECTORS))
        validation = frame[frame.split == 1]
        for name, model in first.items():
            point = model["operating_points"]["0.05"]
            self.assertLessEqual(point["validation_fpr"], 0.05 + 1e-12)
            self.assertEqual(point, choose_threshold(validation.is_anomaly.to_numpy(),
                                                     model["detector"].score(validation), 0.05))
            scores = model["detector"].score(frame)
            self.assertTrue(np.all((scores >= 0) & (scores <= 1)))
            np.testing.assert_array_equal(scores, second[name]["detector"].score(frame))
        self.assertGreater(first["hgb"]["operating_points"]["0.05"]["validation_recall"], 0.5)
        self.assertTrue(first["logreg"]["detector"].info["converged"])
        self.assertEqual(first["hgb_no_peers"]["detector"].columns, supervised.FEATURE_SETS["_no_peers"])

    def test_label_check(self) -> None:
        frame = self.fixture()
        frame.loc[0, "truth_is_anomaly"] = str(1 - frame.is_anomaly[0])
        with self.assertRaises(IntegrityError):
            supervised.training_labels(frame)


class PostHocStageTests(unittest.TestCase):
    """supervised and similarity stages end to end with the stub simulator."""

    def test_stages_with_stub(self) -> None:
        with tempfile.TemporaryDirectory(prefix="examory-posthoc-") as directory:
            root = Path(directory)
            stub = root / "examory-sim"
            stub.write_text(STUB.format(python=sys.executable,
                                        generator=REPO / "ml/datasets/generate_quiz_behavior.py"))
            stub.chmod(0o755)
            exp, stages = root / "exp", root / "stages"
            common = ["--students", "200", "--seeds", "5", "--workers", "1", "--exp-root", str(exp),
                      "--sim-binary", str(stub)]
            run_module.main(["main", *common, "--output", str(stages / "main.json")])
            sup = run_module.main(["supervised", *common, "--output", str(stages / "supervised.json")])
            sim = run_module.main(["similarity", *common, "--output", str(stages / "similarity.json")])
            again = run_module.main(["similarity", *common, "--output", str(root / "again.json")])
            self.assertEqual(list((exp / "data").glob("*/*/*.csv")), [])
            self.assertEqual({(r["source"], r["target"]) for r in sup["runs"]}, set(supervised.PAIRS))
            for run in sup["runs"]:
                self.assertEqual(set(run["metrics"]), set(supervised.DETECTORS))
                for points in run["operating_points"].values():
                    self.assertLessEqual(points["0.05"]["validation_fpr"], 0.05 + 1e-12)
            # The stub's SIM-FI equals LEGACY, so LEGACY->FI and FI->FI coincide.
            by = {(r["source"], r["target"]): r for r in sup["runs"]}
            self.assertEqual(by[("LEGACY", "SIM-FI")]["metrics"]["hgb"]["all"]["at_0.05"],
                             by[("SIM-FI", "SIM-FI")]["metrics"]["hgb"]["all"]["at_0.05"])
            self.assertEqual({r["condition"] for r in sim["runs"]}, {"LEGACY", "SIM-FI", "SIM-BL", "SIM-HONEST"})
            for run in sim["runs"]:
                self.assertTrue(run["eligibility"]["answer_key"]["verified"])
                self.assertEqual(run["eligibility"]["max_shared_items_with_another_student"]["min"], 20)
            self.assertEqual(strip_timing(sim["runs"]), strip_timing(again["runs"]))
            summary = analyze_module.analyze(stages, root / "out", figures=True)
            self.assertTrue(summary["supervised"]["main_pairing"]["all_matched"])
            names = {c["contrast"] for c in summary["supervised"]["contrasts"]}
            self.assertEqual(names, {"FI→FI − FI→BL", "BL→BL − FI→BL", "BL→BL − best unsupervised on BL"})
            self.assertIn("SIM-BL|answersim|slot:S3", summary["similarity"]["cells"])
            for name in ("supervised.md", "similarity.md"):
                self.assertTrue((root / "out/tables" / name).exists())
            self.assertTrue((root / "out/figures/F7_supervised.png").exists())
            committed = expand_stage(read_json(stages / "supervised.json"))
            self.assertAlmostEqual(committed["runs"][0]["metrics"]["hgb"]["all"]["at_0.05"]["recall"],
                                   sup["runs"][0]["metrics"]["hgb"]["all"]["at_0.05"]["recall"], places=5)


class ReviewTests(unittest.TestCase):
    """Stages and statistics added after independent review (15 Nov 2026)."""

    def test_sign_flip_exact(self) -> None:
        from ml.experiments.inflation.stats import sign_flip_p
        self.assertEqual(sign_flip_p([1.0, 1.0, 1.0]), 0.25)          # 2 of 8 patterns reach |3|
        self.assertEqual(sign_flip_p([1.0, -1.0, 2.0]), 0.75)
        self.assertEqual(sign_flip_p([0.5] * 20), 2 / 2 ** 20)        # smallest attainable p for n = 20
        self.assertGreater(sign_flip_p(np.arange(1.0, 26.0)), 0)       # n > 20: Monte Carlo fallback

    def test_prcc(self) -> None:
        from ml.experiments.inflation.stats import prcc
        rng = np.random.default_rng(0)
        x = rng.random((80, 4))
        y = 3 * x[:, 0] - 2 * x[:, 2] + rng.normal(0, 0.2, 80)
        result = prcc(x, list(y))
        self.assertGreater(result[0]["prcc"], 0.8)
        self.assertLess(result[2]["prcc"], -0.7)
        self.assertLess(abs(result[1]["prcc"]), 0.3)
        self.assertEqual(result[0]["df"], 80 - 2 - 3)
        self.assertIsNone(prcc(x[:4], [1.0, 2.0, 3.0, 4.0])[0]["prcc"])  # too few rows

    def test_decorrelated_lhs(self) -> None:
        unit, info = review.decorrelated_lhs(60, 14, 20261115)
        for column in unit.T:
            self.assertEqual(sorted(np.floor(column * 60).astype(int)), list(range(60)))  # still a Latin hypercube
        rho = review.spearman_matrix(unit)
        self.assertLessEqual(np.abs(rho[~np.eye(14, dtype=bool)]).max(), 0.10 + 1e-12)
        self.assertAlmostEqual(info["max_abs_spearman"], np.abs(rho[~np.eye(14, dtype=bool)]).max())
        np.testing.assert_array_equal(unit, review.decorrelated_lhs(60, 14, 20261115)[0])

    def test_mechanism_and_parameter_files(self) -> None:
        base = REPO / "sim/params/mechanisms.json"
        with tempfile.TemporaryDirectory() as directory:
            half = read_json(review.write_fi_dose_mechanisms(0.5, base, Path(directory) / "m.json"))
            one = read_json(review.write_fi_dose_mechanisms(1.0, base, Path(directory) / "one.json"))
            original = read_json(base)
            self.assertEqual(one, original)
            for name in review.FI_DOSE_PARAMETERS:
                self.assertAlmostEqual(half["fi"][name], 0.5 * original["fi"][name])
            self.assertEqual(half["fi"]["collusion_forced_wrong_fraction"], original["fi"]["collusion_forced_wrong_fraction"])
            self.assertEqual({k: v for k, v in half.items() if k != "fi"}, {k: v for k, v in original.items() if k != "fi"})
            values = {**run_module.sensitivity_configs(1)[0], "fi.rapid_item_prob": 0.8,
                      "fi.changes_prob_multiplier": 1.25, "fi.collusion_submit_window_ms": 1234.4,
                      "fi.score_jump_multiplier": 1.5}
            v2 = read_json(review.write_mechanisms_v2(values, base, Path(directory) / "v2.json"))
            self.assertEqual(v2["fi"]["changes_prob_high"], 0.9375)
            self.assertEqual(v2["fi"]["changes_prob_low"], 0.5625)
            self.assertEqual(v2["fi"]["collusion_submit_window_ms"], 1234)
            self.assertAlmostEqual(v2["fi"]["score_jump_high"], 0.48)
            self.assertEqual(v2["copy_peer"]["coverage"], values["copy_peer.coverage"])
            params = REPO / "ml/ednet/calibration.json"
            if params.exists():
                scaled = read_json(review.write_revision_params(0.25, params, Path(directory) / "p.json"))
                self.assertAlmostEqual(scaled["revision"]["change_mult_wrong"],
                                       0.25 * read_json(params)["revision"]["change_mult_wrong"])

    def test_label_free_and_final_only(self) -> None:
        import pandas as pd
        table = pd.DataFrame({"split": [1] * 20 + [2] * 6, "is_anomaly": [0] * 20 + [0, 0, 0, 0, 1, 1],
                              "anomaly_type": ["none"] * 24 + ["lookup", "proxy"],
                              "is_final_attempt": [True] * 20 + [False, False, True, True, True, True],
                              "group_id": ["g"] * 26})
        scores = {"d": np.r_[np.arange(20.0), [100.0, 0.0, 100.0, 0.0, 50.0, 1.0]]}
        points = review.label_free_points(table, scores)
        self.assertAlmostEqual(points["d"]["0.05"]["threshold"], np.quantile(np.arange(20.0), 0.95))
        thresholds = {"d": {"0.05": 18.0, "0.01": 19.0}}
        final = review.final_only_metrics(table, scores, thresholds, 0)["d"]
        self.assertEqual(final["final_only:all"]["at_0.05"]["fpr"], 0.5)        # 1 of the 2 final normals
        self.assertEqual(final["final_only:all"]["at_0.05"]["recall"], 0.5)
        self.assertEqual(final["final_only:slot:S2"]["n_pos"], 1)

    def test_personfit_hand_computed(self) -> None:
        def attempt(times):
            return {"attempt_id": "a", "quiz_id": "q", "variant_id": "v",
                    "first_times": {f"i{k}": t for k, t in enumerate(times)}}
        # Additive log times: ln t = beta_k - tau_i exactly -> residuals 0 except where perturbed.
        betas, taus = np.log([1000.0, 2000.0, 4000.0]), [0.0, 0.5, -0.3, 0.2]
        train = [attempt(np.exp(betas - tau)) for tau in taus]
        train[0]["first_times"]["i0"] *= np.e  # one perturbed cell gives every item a residual spread
        model = similarity.fit_response_time_model(train)
        beta = np.array([model["beta"][json.dumps(["q", "v", f"i{k}"])] for k in range(3)])
        sigma = np.array([model["sigma"][json.dumps(["q", "v", f"i{k}"])] for k in range(3)])
        logs = np.log([1500.0, 2000.0, 9000.0])
        tau = np.mean(beta - logs)
        expected = np.sum(((logs - (beta - tau)) / sigma) ** 2)
        scores, coverage = similarity.response_time_person_fit([attempt(np.exp(logs))], model)
        self.assertAlmostEqual(scores[0], expected)
        self.assertEqual(coverage, 1.0)
        # alternating means reached a fixed point: tau_i = mean(beta - ln t) and beta_k = mean(ln t + tau)
        for a_ in train:
            ln = np.log([a_["first_times"][f"i{k}"] for k in range(3)])
            t = np.mean(beta - ln)
            self.assertLess(abs(np.mean([np.log(b["first_times"]["i0"]) + np.mean(
                beta - np.log([b["first_times"][f"i{k}"] for k in range(3)])) for b in train]) - beta[0]), 1e-9)
            self.assertTrue(np.isfinite(t))

    def test_review_stages_with_stub(self) -> None:
        with tempfile.TemporaryDirectory(prefix="examory-review-") as directory:
            root = Path(directory)
            stub = root / "examory-sim"
            stub.write_text(STUB.format(python=sys.executable,
                                        generator=REPO / "ml/datasets/generate_quiz_behavior.py"))
            stub.chmod(0o755)
            exp, stages = root / "exp", root / "stages"
            common = ["--students", "200", "--seeds", "7", "--workers", "1", "--exp-root", str(exp),
                      "--sim-binary", str(stub)]
            main = run_module.main(["main", *common, "--output", str(stages / "main.json")])
            protocol = run_module.main(["protocol", *common, "--output", str(stages / "protocol.json")])
            dose = run_module.main(["fi-dose", *common, "--lambdas", "0.5", "1.0", "--intensities", "1.0",
                                    "--output", str(stages / "fi_dose.json")])
            self.assertEqual(list((exp / "data").glob("*/*/*.csv")), [])
            by_main = {r["condition"]: r for r in main["runs"]}
            for run in protocol["runs"]:
                if run["variant"] == "standard":
                    self.assertEqual(run["metrics"]["iforest"]["all"]["at_0.05"],
                                     by_main[run["condition"]]["metrics"]["iforest"]["all"]["at_0.05"])
                self.assertIn("final_only:all", run["metrics"]["quantile_rules"])
            self.assertEqual({r["variant"] for r in protocol["runs"]}, set(review.PROTOCOL_VARIANTS))
            # The stub's SIM-BL attempts are the honest ones relabelled: every cheater is event-identical.
            standard_bl = next(r for r in protocol["runs"] if r["condition"] == "SIM-BL" and r["variant"] == "standard")
            self.assertTrue(all(v["event_identical"] == v["n"] for v in standard_bl["unobservable"].values()))
            contaminated = next(r for r in protocol["runs"] if r["condition"] == "LEGACY"
                                and r["variant"] == "contaminated")
            self.assertGreater(contaminated["contaminated_train_positives"], 0)
            self.assertEqual({r.get("lambda") for r in dose["runs"] if r["condition"] == "SIM-FI"}, {0.5, 1.0})
            summary = analyze_module.analyze(stages, root / "out", figures=True)
            self.assertIn("protocol", summary)
            self.assertIn("fi_dose", summary)
            self.assertIn("subpopulations", summary)
            # H1 contrast of the protocol stage: the main-protocol rows reproduce h1.md's computation
            h1_rows = {(r["variant"], r["subset"]): r for r in summary["protocol"]["h1"]}
            self.assertEqual({v for v, _ in h1_rows}, set(review.PROTOCOL_VARIANTS))
            for row in summary["h1"]["SIM-BL"]["rows"]:
                ours = h1_rows[("standard", f"slot:{row['slot']}")]
                self.assertEqual((ours["mean"], ours["low"], ours["high"], ours["p"]),
                                 (row["test"]["mean"], row["test"]["low"], row["test"]["high"], row["test"]["p"]))
            self.assertIn("SIM-BL", summary["protocol"]["train_contamination"])
            self.assertTrue((root / "out/figures/F8_fi_dose.png").exists())
            for name in ("protocol.md", "fi_dose.md", "subpopulations.md", "pvalue_changes.md"):
                self.assertTrue((root / "out/tables" / name).exists(), name)


if __name__ == "__main__":
    unittest.main()

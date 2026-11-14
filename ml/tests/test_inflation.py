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
from ml.experiments.inflation import analyze as analyze_module
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
        self.assertAlmostEqual(result["p"], scipy_stats.wilcoxon(diff).pvalue)
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


if __name__ == "__main__":
    unittest.main()

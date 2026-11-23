# False positives and score gains in simulator cohorts

Use the repository's Go simulator and the Python environment in `ml/service/requirements.txt`. The commands below reproduce the checked-in E-16 and E-17 tables. They create one calibration-derived SIM-BL world with a fixed roster. Keep the raw world, model pickle and split manifest under the ignored raw directory. Run from the repository root.

```sh
world=ml/datasets/raw/e16-seed42
(cd sim && go build -o /tmp/examory-sim ./cmd/examory-sim)
/tmp/examory-sim generate --students 1200 --attempts 3 --questions 20 --seed 42 \
  --anomaly-rate 0.12 --params ml/ednet/calibration.json --mechanisms sim/params/mechanisms.json \
  --item-sampling fixed --worlds bl --out-dir "$world"
python -m ml.experiments.behavior.study --world-dir "$world" --output ml/experiments/behavior/results.md
```

The quartiles are fitted on unique training-cohort students observed in normal attempts. The held-out FPR table reports stratum sizes and the positive alert count. [Subgroup results](results.md)

The same study also compares a historical gain rule with the forest and quantile baselines. Run `python -m unittest ml.experiments.behavior.test_jump -v` to check time ordering and practice-gain adjustment. [Score-gain results](results.md)

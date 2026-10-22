<!-- ml/anomaly/results.md -->
# Behavior baseline results — pending execution

No measurements are claimed in this template.

```bash
python -m ml.anomaly.evaluate --csv ml/datasets/raw/quiz.csv --model ml/anomaly/artifacts/model.pkl --bootstrap 1000 --output ml/anomaly/results.md
```

| Model | Subset | N | Precision | Recall | F1 | FPR | PR-AUC |
|---|---|---:|---:|---:|---:|---:|---:|
| Isolation Forest with peers | all / final / each type | pending | pending | pending | pending | pending | pending |
| Isolation Forest without peers | all / final / each type | pending | pending | pending | pending | pending | pending |
| Training quantile rules | all / final / each type | pending | pending | pending | pending | pending | pending |

The command also writes confusion counts, validation-selected 1%/5% operating
points, cohort bootstrap intervals, PR curves in artifacts/evaluation.json,
runtime and Python-tracked peak memory. No test threshold is selected.

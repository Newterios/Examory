# ML modules

Owner: Akhanov Yerdaulet.

| Research task | Implementation and results |
|---|---|
| Detect text reuse with MinHash | [Run the PAN-PC-11 evaluation](integrity/README.md) and inspect the [pair and span results](integrity/results.md). |
| Score quiz attempts | The [behaviour baseline](anomaly/README.md) documents feature extraction, training and evaluation. The [baseline report](anomaly/results.md) covers the quiz generator. |
| Test H1 on a separate simulator | The [inflation experiment](experiments/inflation/README.md) compares detectors on simulated behaviour. Its [H1 table](experiments/inflation/results/tables/h1.md) reports the comparison with quantile rules. |
| Calibrate response times using EdNet | See the [EdNet pipeline](ednet/README.md) and its [calibration report](ednet/calibration_report.md). |

The [dataset notes](notebooks/datasets.md) and [baseline plan](notebooks/baseline_plan.md) describe data selection and evaluation splits. Download or generate raw inputs locally with the scripts under `datasets/`. Run the ML tests from the repository root with `python -m unittest discover -s ml/tests -v`.

# Java code similarity on a BigCloneBench subset

The prepared subset contains 763 functions, 256 validation pairs and 256 held-out test pairs. All metrics refer to supplied pairs. Repository-wide candidate retrieval was outside this experiment.

Winnowing used 5-token grams and minima over 4 grams. Validation selected a Jaccard threshold of **0.20** by F1, then precision, then the higher threshold. The exact-token comparator uses full lexical token equality. Both methods were frozen before scoring test pairs.

| Method | Test subset | TP | FP | FN | TN | Precision | Recall | F1 | FPR |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Winnowing | All | 190 | 4 | 2 | 60 | 0.9794 | 0.9896 | 0.9845 | 0.0625 |
| Winnowing | T1 | 64 | 4 | 0 | 60 | 0.9412 | 1.0000 | 0.9697 | 0.0625 |
| Winnowing | T2 | 64 | 4 | 0 | 60 | 0.9412 | 1.0000 | 0.9697 | 0.0625 |
| Winnowing | T3 | 62 | 4 | 2 | 60 | 0.9394 | 0.9688 | 0.9538 | 0.0625 |
| Exact tokens | All | 64 | 0 | 128 | 64 | 1.0000 | 0.3333 | 0.5000 | 0.0000 |
| Exact tokens | T1 | 64 | 0 | 0 | 64 | 1.0000 | 1.0000 | 1.0000 | 0.0000 |
| Exact tokens | T2 | 0 | 0 | 64 | 64 | NA | 0.0000 | 0.0000 | 0.0000 |
| Exact tokens | T3 | 0 | 0 | 64 | 64 | NA | 0.0000 | 0.0000 | 0.0000 |

Each type-specific row compares positives of that type with the same labelled nonclones. The negative denominator and FPR are consequently shared. Precision reflects this sampling ratio. An undefined denominator is marked NA.

## Uncertainty

Percentile 95% intervals use 1000 draws of test sampling groups with replacement (seed 42). Each draw retains the selected groups' paired labels and scores. Only draws with a defined metric contribute to its interval.

| Method | Subset | Groups | Precision | Recall | F1 | FPR |
|---|---|---:|---|---|---|---|
| Winnowing | All | 30 | [0.937, 1.000] | [0.962, 1.000] | [0.958, 0.997] | [0.000, 0.353] |
| Winnowing | T1 | 16 | [0.839, 1.000] | [1.000, 1.000] | [0.912, 1.000] | [0.000, 0.250] |
| Winnowing | T2 | 13 | [0.684, 1.000] | [1.000, 1.000] | [0.812, 1.000] | [0.000, 0.261] |
| Winnowing | T3 | 28 | [0.851, 1.000] | [0.896, 1.000] | [0.893, 0.993] | [0.000, 0.294] |
| Exact tokens | All | 30 | [1.000, 1.000] | [0.227, 0.464] | [0.370, 0.634] | [0.000, 0.000] |
| Exact tokens | T1 | 16 | [1.000, 1.000] | [1.000, 1.000] | [1.000, 1.000] | [0.000, 0.000] |
| Exact tokens | T2 | 13 | NA | [0.000, 0.000] | [0.000, 0.000] | [0.000, 0.000] |
| Exact tokens | T3 | 28 | NA | [0.000, 0.000] | [0.000, 0.000] | [0.000, 0.000] |

Winnowing flagged 4 of the 64 labelled nonclone pairs. The overall FPR interval is wide because related methods share sampling groups. The T3 stratum covers strongly and moderately similar clones. The release combines weak T3 with T4, so this evaluation gives no separate estimate for weak T3. A benchmark clone label describes similarity between methods. A teacher still needs the assignment and source context to assess a submission.

## Reproduction

See the [evaluation instructions](README.md) for dataset provenance, selection rules and the commands. Local input checksums (SHA-256) identify this sample.

- `functions.jsonl`: `7d17bf6d793ebf57d3b4126b1df1c92dea806c4d67b1b1aae9de5d2f5140f57c`
- `pairs.csv`: `4df94643ff7854fa0587caddd6a6c909b6c7014295732b20f1cf10618ce255cf`
- `provenance.json`: `c71150116b2ffd9ccd12a67399ab8f3ed40b6f0c4f722d79e035b29d43c5c314`

<!-- ml/integrity/results.md -->
# Text reuse results — pending PAN execution

No benchmark measurements are claimed in this template. After downloading and
extracting PAN-PC-11, run from the repository root:

```bash
python -m ml.integrity.evaluate --corpus ml/datasets/raw/pan-pc-11/extracted --max-documents 120 --seed 42 --output ml/integrity/results.md
```

| Method | Candidate recall | Pair precision | Pair recall | Pair F1 | FPR |
|---|---:|---:|---:|---:|---:|
| MinHash 128, LSH 32×4, exact Jaccard | pending | pending | pending | pending | pending |
| Exhaustive Jaccard, independently tuned | pending | pending | pending | pending | pending |
| Exhaustive Jaccard, frozen LSH settings | pending | pending | pending | pending | pending |
| TF-IDF cosine | pending | pending | pending | pending | pending |

The evaluator adds literal/light/heavy/translated breakdowns, character-span
metrics, coverage, validation settings, runtime and a full artifact manifest.

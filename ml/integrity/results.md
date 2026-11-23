<!-- ml/integrity/results.md -->
# PAN-PC-11 evaluation of text reuse

## Interpretation

Windowed MinHash missed the H3 recall targets on all five 360-document splits (seeds 42–46). Candidate recall ranged from 0.0% to 33.3%. No pair passed exact verification, so pair recall was zero and precision was undefined.
H3 sets pair precision at 0.90, pair recall at 0.85, and candidate recall at 0.95. A positive H3 pair has a valid literal or lightly edited target passage of at least 50 normalized tokens. Known positive pairs outside this definition are omitted from the H3 comparison. Every negative pair stays in its denominator. Thresholds and minimum evidence lengths come from validation and are frozen for test.

The source/copy graph has 22,186 documents in 9,389 connected families. Selection and splitting keep each connected family together. The larger budget was chosen from annotation counts. The five test sets can share documents across seeds, so each has its own estimates and intervals. The results describe these closed subsets and the mix of reuse types they contain.

## H3 outcomes for MinHash

| Run | Documents | Oversized families excluded | Positive test families | H3-positive test families | Candidate hits / eligible positive pairs | Validation Jaccard | Minimum evidence tokens | Pair precision | Pair recall | Candidate recall | Quality targets |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 120-42 | 120 | 3 | 3 | 1 | 0/1 | 0.8 | 50 | NA | 0.0000 | 0.0000 | Missed recall, candidate_recall |
| 360-42 | 360 | 3 | 7 | 4 | 1/4 | 0.8 | 50 | NA | 0.0000 | 0.2500 | Missed recall, candidate_recall |
| 360-43 | 360 | 3 | 5 | 3 | 0/4 | 0.8 | 50 | NA | 0.0000 | 0.0000 | Missed recall, candidate_recall |
| 360-44 | 360 | 3 | 7 | 3 | 1/3 | 0.8 | 50 | NA | 0.0000 | 0.3333 | Missed recall, candidate_recall |
| 360-45 | 360 | 3 | 5 | 2 | 0/2 | 0.8 | 50 | NA | 0.0000 | 0.0000 | Missed recall, candidate_recall |
| 360-46 | 360 | 3 | 8 | 1 | 0/1 | 0.8 | 50 | NA | 0.0000 | 0.0000 | Missed recall, candidate_recall |
| 360-42-whole | 360 | 3 | 7 | 4 | 0/4 | 0.8 | 50 | NA | 0.0000 | 0.0000 | Missed recall, candidate_recall |
| 360-42-256 | 360 | 3 | 7 | 4 | 2/4 | 0.8 | 50 | NA | 0.0000 | 0.5000 | Missed recall, candidate_recall |

A pair hit identifies the source document. The appendix reports span localization separately. The two seed-42 ablations use exactly the same documents and family split as the larger seed-42 baseline. Whole-document signatures retrieved 0 of 4 qualifying pairs, windowed signatures with 128 permutations retrieved 1, and windowed signatures with 256 permutations retrieved 2.

## Family bootstrap intervals

The 95% percentile intervals resample query families with replacement within each test split against its fixed source bank. Each cell gives the interval and the fraction of bootstrap draws with a defined denominator.

| Run | Eligible query families | Pair precision | Pair recall | Candidate recall | F1 |
| --- | --- | --- | --- | --- | --- |
| 120-42 | 13 | NA | [0.0000, 0.0000] (655/1000) | [0.0000, 0.0000] (655/1000) | [0.0000, 0.0000] (655/1000) |
| 360-42 | 38 | NA | [0.0000, 0.0000] (980/1000) | [0.0000, 1.0000] (980/1000) | [0.0000, 0.0000] (980/1000) |
| 360-43 | 43 | NA | [0.0000, 0.0000] (952/1000) | [0.0000, 0.0000] (952/1000) | [0.0000, 0.0000] (952/1000) |
| 360-44 | 40 | NA | [0.0000, 0.0000] (955/1000) | [0.0000, 1.0000] (955/1000) | [0.0000, 0.0000] (955/1000) |
| 360-45 | 41 | NA | [0.0000, 0.0000] (863/1000) | [0.0000, 0.0000] (863/1000) | [0.0000, 0.0000] (863/1000) |
| 360-46 | 41 | NA | [0.0000, 0.0000] (623/1000) | [0.0000, 0.0000] (623/1000) | [0.0000, 0.0000] (623/1000) |
| 360-42-whole | 38 | NA | [0.0000, 0.0000] (980/1000) | [0.0000, 0.0000] (980/1000) | [0.0000, 0.0000] (980/1000) |
| 360-42-256 | 38 | NA | [0.0000, 0.0000] (980/1000) | [0.0000, 1.0000] (980/1000) | [0.0000, 0.0000] (980/1000) |

## Test retrieval time

After common reference indexing and query preparation, the two measured passes run retrieval and exact window checks. Timings in seconds exclude validation tuning and final evidence merging. The ratio is LSH seconds divided by exhaustive seconds. Values below 1 indicate faster retrieval.

| Run | Shared preparation (s) | LSH (s) | Exhaustive (s) | LSH / exhaustive |
| --- | --- | --- | --- | --- |
| 120-42 | 11.2004 | 0.1506 | 1.3445 | 0.1120 |
| 360-42 | 16.3964 | 0.5612 | 8.0805 | 0.0695 |
| 360-43 | 18.3235 | 0.6360 | 10.3835 | 0.0612 |
| 360-44 | 17.4580 | 0.3066 | 11.0163 | 0.0278 |
| 360-45 | 12.5001 | 0.3290 | 9.8453 | 0.0334 |
| 360-46 | 16.3779 | 0.4522 | 10.1140 | 0.0447 |
| 360-42-whole | 8.8999 | 0.0012 | 5.1227 | 0.0002 |
| 360-42-256 | 14.8552 | 0.7886 | 6.7080 | 0.1176 |

## Comparators on the H3 subset

The exhaustive comparator has its own validation settings. A second exhaustive pass uses the settings of the LSH run to isolate retrieval loss. TF-IDF fits the vectorizer on training text and tunes cosine on validation.

| Run | Method | Eligible positives | Precision | Recall | F1 | Candidate recall |
| --- | --- | --- | --- | --- | --- | --- |
| 360-42 | lsh | 4 | NA | 0.0000 | 0.0000 | 0.2500 |
| 360-42 | exhaustive | 4 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-42 | exhaustive_lsh_settings | 4 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-42 | tfidf_cosine | 4 | 1.0000 | 0.2500 | 0.4000 | 1.0000 |
| 360-43 | lsh | 4 | NA | 0.0000 | 0.0000 | 0.0000 |
| 360-43 | exhaustive | 4 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-43 | exhaustive_lsh_settings | 4 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-43 | tfidf_cosine | 4 | 0.0000 | 0.0000 | 0.0000 | 1.0000 |
| 360-44 | lsh | 3 | NA | 0.0000 | 0.0000 | 0.3333 |
| 360-44 | exhaustive | 3 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-44 | exhaustive_lsh_settings | 3 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-44 | tfidf_cosine | 3 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-45 | lsh | 2 | NA | 0.0000 | 0.0000 | 0.0000 |
| 360-45 | exhaustive | 2 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-45 | exhaustive_lsh_settings | 2 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-45 | tfidf_cosine | 2 | 0.0114 | 0.5000 | 0.0222 | 1.0000 |
| 360-46 | lsh | 1 | NA | 0.0000 | 0.0000 | 0.0000 |
| 360-46 | exhaustive | 1 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-46 | exhaustive_lsh_settings | 1 | NA | 0.0000 | 0.0000 | 1.0000 |
| 360-46 | tfidf_cosine | 1 | 0.0000 | 0.0000 | 0.0000 | 1.0000 |

## Reuse types

These pair counts include annotated passages of any length. A pair can belong to more than one type. Each category is evaluated against the same negative pairs for that split. NA marks an undefined denominator.

| Run | Category | Gold pairs | Detected pairs | Precision | Recall | F1 |
| --- | --- | --- | --- | --- | --- | --- |
| 360-42 | literal | 0 | 0 | NA | NA | NA |
| 360-42 | light | 5 | 0 | NA | 0.0000 | 0.0000 |
| 360-42 | heavy | 2 | 0 | NA | 0.0000 | 0.0000 |
| 360-42 | translation | 1 | 0 | NA | 0.0000 | 0.0000 |
| 360-42 | simulated | 0 | 0 | NA | NA | NA |
| 360-42 | other_or_unknown | 0 | 0 | NA | NA | NA |
| 360-43 | literal | 0 | 0 | NA | NA | NA |
| 360-43 | light | 4 | 0 | NA | 0.0000 | 0.0000 |
| 360-43 | heavy | 4 | 0 | NA | 0.0000 | 0.0000 |
| 360-43 | translation | 2 | 0 | NA | 0.0000 | 0.0000 |
| 360-43 | simulated | 0 | 0 | NA | NA | NA |
| 360-43 | other_or_unknown | 0 | 0 | NA | NA | NA |
| 360-44 | literal | 1 | 0 | NA | 0.0000 | 0.0000 |
| 360-44 | light | 3 | 0 | NA | 0.0000 | 0.0000 |
| 360-44 | heavy | 3 | 0 | NA | 0.0000 | 0.0000 |
| 360-44 | translation | 1 | 0 | NA | 0.0000 | 0.0000 |
| 360-44 | simulated | 0 | 0 | NA | NA | NA |
| 360-44 | other_or_unknown | 0 | 0 | NA | NA | NA |
| 360-45 | literal | 0 | 0 | NA | NA | NA |
| 360-45 | light | 2 | 0 | NA | 0.0000 | 0.0000 |
| 360-45 | heavy | 8 | 0 | NA | 0.0000 | 0.0000 |
| 360-45 | translation | 0 | 0 | NA | NA | NA |
| 360-45 | simulated | 0 | 0 | NA | NA | NA |
| 360-45 | other_or_unknown | 0 | 0 | NA | NA | NA |
| 360-46 | literal | 0 | 0 | NA | NA | NA |
| 360-46 | light | 1 | 0 | NA | 0.0000 | 0.0000 |
| 360-46 | heavy | 10 | 0 | NA | 0.0000 | 0.0000 |
| 360-46 | translation | 0 | 0 | NA | NA | NA |
| 360-46 | simulated | 0 | 0 | NA | NA | NA |
| 360-46 | other_or_unknown | 0 | 0 | NA | NA | NA |

Recall for heavy pairs was 0.0000 (5 splits with positive examples).

Recall for light pairs was 0.0000 (5 splits with positive examples).

Recall for literal pairs was 0.0000 (one split with positive examples).

No other or unknown pairs occurred in the larger test splits.

No simulated pairs occurred in the larger test splits.

Recall for translation pairs was 0.0000 (3 splits with positive examples).

Small numbers of qualifying families limit precision of the estimates. Oversized connected components fall outside the selected population. The evaluation measures recovery of annotated text reuse. A review of academic misconduct also requires context such as attribution and assignment rules.

## Reproduction

Repository base [`42d825a8a40b`](https://github.com/Newterios/Examory/commit/42d825a8a40bd68a34001ba12e91278de9586f65). The evaluator records SHA-256 digests of its Python sources. These identify the code used in addition to the base revision.

Data: [official PAN-PC-11 release](https://doi.org/10.5281/zenodo.3250095). Expected archive MD5s are `b2930f859497dd48ba5bb606d3f4a4f3` (part 1) and `b23d86c17a47d2bfbdc4c314ea5810df` (part 2). The [integrity README](README.md) documents the downloader, archive extraction, and reference selection.

Runs: 120-42, 360-42, 360-43, 360-44, 360-45, 360-46, 360-42-whole, 360-42-256. Python 3.12.10. datasketch 1.6.5, numpy 2.3.5, pandas 3.0.1, scikit-learn 1.7.2.

| Python source | SHA-256 |
| --- | --- |
| `evaluate.py` | `0f9d441a987ee242ea54852b5ef6f678c32e391ca77c6097b1107c99b8049539` |
| `minhash.py` | `102da76325dc6e1e1a6d7c4cb7557c5df2d3a28c11a2fe21ea55a57739a1b83b` |
| `to_risk_event.py` | `5f464f3dc93b97858f622c4add0e767a9a7dce12ad017f624ea34db2145cdd32` |

Run from the repository root with Python 3.12 and 7-Zip available.

```sh
python -m pip install -r ml/requirements.txt
python ml/datasets/download_pan.py --download --extract
CORPUS=ml/datasets/raw/pan-pc-11/extracted
OUT=ml/integrity/artifacts/stage2
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 120 --seed 42 --mode windows --num-perm 128 --no-tracemalloc --artifacts "$OUT/120-42" --output "$OUT/120-42/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 42 --mode windows --num-perm 128 --no-tracemalloc --artifacts "$OUT/360-42" --output "$OUT/360-42/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 43 --mode windows --num-perm 128 --no-tracemalloc --artifacts "$OUT/360-43" --output "$OUT/360-43/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 44 --mode windows --num-perm 128 --no-tracemalloc --artifacts "$OUT/360-44" --output "$OUT/360-44/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 45 --mode windows --num-perm 128 --no-tracemalloc --artifacts "$OUT/360-45" --output "$OUT/360-45/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 46 --mode windows --num-perm 128 --no-tracemalloc --artifacts "$OUT/360-46" --output "$OUT/360-46/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 42 --mode whole --num-perm 128 --no-tracemalloc --artifacts "$OUT/360-42-whole" --output "$OUT/360-42-whole/results.md"
python -m ml.integrity.evaluate --corpus "$CORPUS" --max-documents 360 --seed 42 --mode windows --num-perm 256 --no-tracemalloc --artifacts "$OUT/360-42-256" --output "$OUT/360-42-256/results.md"
```

## Appendix: initial 120-document run at seed 42


Closed families, positive-family stratification; oversized families excluded. Not full-corpus performance.
Selected 120 of 22186 documents; seed 42.

| Method | Pairs | Positives | TP | FP | FN | TN | Candidate recall | Precision | Recall | F1 | FPR |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| lsh | 140 | 5 | 0 | 0 | 5 | 135 | 0.0000 | NA | 0.0000 | 0.0000 | 0.0000 |
| exhaustive | 140 | 5 | 0 | 0 | 5 | 135 | 1.0000 | NA | 0.0000 | 0.0000 | 0.0000 |
| exhaustive_lsh_settings | 140 | 5 | 0 | 0 | 5 | 135 | 1.0000 | NA | 0.0000 | 0.0000 | 0.0000 |
| tfidf_cosine | 140 | 5 | 0 | 16 | 5 | 119 | 1.0000 | 0.0000 | 0.0000 | 0.0000 | 0.1185 |

Corpus annotations with a negative offset or nonpositive length: 1. Additional out-of-bounds annotations in the selected documents: 0. Test pairs excluded from localization metrics: 0. A known source link still counts as a positive pair even if its passage cannot be localized. Only spans valid on both sides qualify a pair for H3.

### H3: literal or lightly edited reuse of at least 50 tokens

A positive pair contains at least one literal or light PAN target span with 50 or more normalized tokens. Other annotated positive pairs are excluded from this comparison. All held-out negative pairs remain. A hit is a correct document pair. Span localization is reported separately.

Test positive families: 3. Families with qualifying H3 pairs: 1. Oversized families excluded before selection: 3.

| Method | TP | FP | FN | TN | Candidates retrieved / gold | Candidate recall | Precision | Recall | F1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| lsh | 0 | 0 | 1 | 135 | 0/1 | 0.0000 | NA | 0.0000 | 0.0000 |
| exhaustive | 0 | 0 | 1 | 135 | 1/1 | 1.0000 | NA | 0.0000 | 0.0000 |
| exhaustive_lsh_settings | 0 | 0 | 1 | 135 | 1/1 | 1.0000 | NA | 0.0000 | 0.0000 |
| tfidf_cosine | 0 | 16 | 1 | 119 | 1/1 | 1.0000 | 0.0000 | 0.0000 | 0.0000 |

Percentile 95% intervals resample whole query families against the fixed test source bank, using the frozen validation settings. Parentheses show defined replicates out of 1,000. Fewer than two eligible query families gives no interval.

| Method | Candidate recall | Precision | Recall | F1 |
|---|---|---|---|---|
| lsh | [0.0000, 0.0000] (655/1000) | NA | [0.0000, 0.0000] (655/1000) | [0.0000, 0.0000] (655/1000) |
| exhaustive | [1.0000, 1.0000] (655/1000) | NA | [0.0000, 0.0000] (655/1000) | [0.0000, 0.0000] (655/1000) |
| exhaustive_lsh_settings | [1.0000, 1.0000] (655/1000) | NA | [0.0000, 0.0000] (655/1000) | [0.0000, 0.0000] (655/1000) |
| tfidf_cosine | [1.0000, 1.0000] (655/1000) | [0.0000, 0.0000] (998/1000) | [0.0000, 0.0000] (655/1000) | [0.0000, 0.0000] (1000/1000) |

Test reference indexing and query preparation: 11.200 s. On the prepared documents, LSH retrieval and exact window checks took 0.151 s. Exhaustive retrieval and exact window checks took 1.345 s. Both timings exclude validation tuning and final evidence merging.

### Validation-selected settings

```json
{
  "lsh": {
    "threshold": 0.8,
    "min_span": 50
  },
  "exhaustive": {
    "threshold": 0.8,
    "min_span": 50
  },
  "tfidf_cosine": {
    "threshold": 0.30000000000000004
  }
}
```

### Annotation categories (pairs can belong to multiple categories)

| Method | Category | Positive pairs | Precision | Recall | F1 |
|---|---|---:|---:|---:|---:|
| lsh | literal | 0 | NA | NA | NA |
| exhaustive | literal | 0 | NA | NA | NA |
| exhaustive_lsh_settings | literal | 0 | NA | NA | NA |
| tfidf_cosine | literal | 0 | 0.0000 | NA | 0.0000 |
| lsh | light | 1 | NA | 0.0000 | 0.0000 |
| exhaustive | light | 1 | NA | 0.0000 | 0.0000 |
| exhaustive_lsh_settings | light | 1 | NA | 0.0000 | 0.0000 |
| tfidf_cosine | light | 1 | 0.0000 | 0.0000 | 0.0000 |
| lsh | heavy | 4 | NA | 0.0000 | 0.0000 |
| exhaustive | heavy | 4 | NA | 0.0000 | 0.0000 |
| exhaustive_lsh_settings | heavy | 4 | NA | 0.0000 | 0.0000 |
| tfidf_cosine | heavy | 4 | 0.0000 | 0.0000 | 0.0000 |
| lsh | translation | 0 | NA | NA | NA |
| exhaustive | translation | 0 | NA | NA | NA |
| exhaustive_lsh_settings | translation | 0 | NA | NA | NA |
| tfidf_cosine | translation | 0 | 0.0000 | NA | 0.0000 |
| lsh | simulated | 0 | NA | NA | NA |
| exhaustive | simulated | 0 | NA | NA | NA |
| exhaustive_lsh_settings | simulated | 0 | NA | NA | NA |
| tfidf_cosine | simulated | 0 | 0.0000 | NA | 0.0000 |
| lsh | other_or_unknown | 0 | NA | NA | NA |
| exhaustive | other_or_unknown | 0 | NA | NA | NA |
| exhaustive_lsh_settings | other_or_unknown | 0 | NA | NA | NA |
| tfidf_cosine | other_or_unknown | 0 | 0.0000 | NA | 0.0000 |

### Pair-conditioned character-span micro metrics

| Method | Side | Precision | Recall |
|---|---|---:|---:|
| lsh | target | NA | 0.0000 |
| lsh | source | NA | 0.0000 |
| exhaustive | target | NA | 0.0000 |
| exhaustive | source | NA | 0.0000 |

### Constructed hard negatives, separate from PAN

| Case | Pairs | Flagged | False positives under construction assumption |
|---|---:|---:|---:|
| attributed_quotation | 1 | 0 | 0 |
| shared_teacher_template_without_allowlist | 1 | 1 | 1 |
| shared_teacher_template_with_allowlist | 1 | 0 | 0 |

Query coverage (>=5 non-template tokens): 1.0000.
Total wall time: 81.11 s. Python allocation peak was not measured.
TF-IDF vocabulary/IDF fit only on train; exhaustive comparisons use identical held-out pairs.
Span metrics are not official PAN plagdet/granularity. Missed candidates count as false negatives.
The three constructed stress cases assume attributed quotation/provided templates are legitimate. They are not independently adjudicated or PAN truth.
Detailed grids, hashes, manifests and retrieval timings: ml/integrity/artifacts/stage2/120-42/experiment.json.

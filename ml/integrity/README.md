<!-- ml/integrity/README.md -->
# LMS-17: text reuse baseline

Python 3.12; run commands from the repository root. The four direct Python
dependencies are pinned in ml/requirements.txt. No pretrained weights are used.

```bash
python -m pip install -r ml/requirements.txt
python ml/datasets/download_pan.py
python ml/datasets/download_pan.py --download --extract
python -m ml.integrity.evaluate --corpus ml/datasets/raw/pan-pc-11/extracted --max-documents 120 --seed 42
```

The [official PAN-PC-11 release](https://zenodo.org/records/3250095) comprises two
multipart RAR volumes, about 1.7 GB total. Research use is permitted by its
description; no standard license identifier is stated. The first command prints
metadata only; `--download` transfers both volumes and validates their pinned
MD5 checksums. Install [7-Zip](https://www.7-zip.org/) for `--extract`, or unpack
part1.rar manually while both parts are together. `--extractor /path/to/7z`
selects the executable. Extraction requires a system tool because the allowed
Python packages and standard library do not decode multipart RAR. Interrupted
partial downloads are restarted. Archives, extracted text and manifests stay
under ignored ml/datasets/raw/.

The loader locates external-detection-corpus, parses PAN XML and validates source
references and span bounds. Every source and all connected suspicious copies
form one indivisible family, computed before subset selection. The experiment
uses a small closed-family subset for tractable exhaustive comparison: at least
five positive families, maximum 120 documents by default, 60/20/20 family splits
stratified by whether a family contains reuse. Families larger than one fifth of
the budget are excluded and their count is reported. Increase --max-documents
if there are too few positive families; giant connected components are never
silently split. This selection is biased toward small families and must not be
described as full-corpus PAN performance. No text truncation is performed.
Source documents within each held-out partition are the retrieval reference bank.
No source/copy family crosses a partition. The artifact records source hashes,
the subset, seed, thresholds and all validation grid results.

Tokenization groups Unicode letters, marks, numbers and underscore; applies NFKC
and casefold; and retains original code-point offsets. Expansion of a single
compatibility word can map several tokens to the same original interval.
Five-token shingle sets feed 128 seeded permutations and exactly 32×4 LSH bands.
The detector indexes 200-token windows every 100 tokens, keeping the short tail;
fewer than five usable tokens abstains. Candidate probability at similarity s is
1-(1-s^4)^32, not a deterministic threshold. Exact Jaccard verifies retrieved
windows. Matching shingle positions reconstruct contiguous normalized-token
runs; overlapping hits on the same alignment are merged. Pair score is the
maximum accepted local-window Jaccard, not whole-document overlap.

Validation searches Jaccard {0.50,0.65,0.80} and minimum contiguous evidence
length {20,50} tokens. It maximizes pair F1, then precision, then conservative
threshold/span length. Exhaustive Jaccard gets its own validation tuning and an
additional run at the frozen LSH settings to isolate candidate loss. TF-IDF fits
vocabulary/IDF only on train; cosine threshold searches 0.1 through 0.9 on
validation. All methods use the same test pairs. Missed candidates are false
negatives. Category rows use PAN obfuscation/type metadata and retain unknown
categories. A pair with several categories can appear in several rows.
Character-span metrics union overlaps per document pair; these are not the
official PAN plagdet/granularity measures. Undefined denominators are NA.

Whole-document and larger-signature ablations use identical subset seeds:

```bash
python -m ml.integrity.evaluate --corpus ml/datasets/raw/pan-pc-11/extracted --mode whole --artifacts ml/integrity/artifacts/whole --output ml/integrity/artifacts/whole/results.md
python -m ml.integrity.evaluate --corpus ml/datasets/raw/pan-pc-11/extracted --num-perm 256 --artifacts ml/integrity/artifacts/256 --output ml/integrity/artifacts/256/results.md
```

Exhaustive window comparison can be expensive for long books: start with the
small subset. `--templates path.json` accepts a JSON list of explicit
teacher-provided template strings and excludes their five-token shingles from
both retrieval and verification. Citations and quotations are not automatically
removed. Legitimate quotations/template examples need separate human labels;
they are not assumed negative merely because the text contains quotation marks.
The report separately includes three constructed stress cases: an attributed
quotation and shared teacher instructions with/without an explicit allowlist.
Legitimacy is an explicit construction assumption, not an adjudicated label;
these cases never enter PAN metrics or validation tuning.
Paraphrase, translation and code similarity remain limitations/separate work.

Standalone comparison (use input text files, no PAN required):

```bash
python -m ml.integrity.minhash --query ml/datasets/raw/query.txt --sources ml/datasets/raw/sources --output ml/integrity/artifacts/matches.json
```

Convert a resolved submission.created event and source submission texts using
the validation-selected experiment:

```bash
python -m ml.integrity.to_risk_event --query ml/datasets/raw/query.txt --sources ml/datasets/raw/sources --source-id-map ml/datasets/raw/source_ids.json --submission-event ml/datasets/raw/submission.json --experiment ml/integrity/artifacts/experiment.json --output ml/integrity/artifacts/risk.json
```

source_ids.json maps relative filenames to actual submission IDs, for example
`{"source_a.txt":"sub_377"}`. Supply sources from the applicable assignment and
resolve text_ref to the local query file beforehand. The converter excludes the
target's own ID. risk.plagiarism uses the complete v1 envelope and payload with
evidence.matches containing submission_id, similarity and overlap_spans.
Each overlap span is `{"target":[start,end],"source":[start,end],"token_count":N}`.
The interval ends are exclusive and refer to original Unicode code points.
The shared draft does not specify overlap_spans internals; this representation
is the documented baseline convention pending schema alignment. Preserve
newlines when resolving extracted text; offsets must refer to the same string.

Medium starts at the selected validation Jaccard threshold; high starts at the
smallest eligible validation grid threshold with precision>=0.90 and the chosen
minimum span. If none qualifies, high is disabled, not invented. These labels
are relative review priorities; similarity does not establish plagiarism.

Raw inputs and artifacts are excluded by ml/.gitignore. results.md is a template
until a real corpus run overwrites it; smoke fixtures are not PAN measurements.
Run `python -m unittest discover -s ml/tests -v` for the regression suite. It
creates disposable synthetic events and a PAN-format fixture outside the repository.

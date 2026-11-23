# Java code similarity

This offline baseline compares two Java submissions and locates matching code
in both originals. Install the existing pinned dependencies from
[ml/requirements.txt](../../requirements.txt) with Python 3.12. The lexer and
fingerprint algorithm use the standard library. The command below reuses the
existing integrity event builder.

## Prepare the Java benchmark

The evaluation uses the authors' [Toma distribution of BigCloneBench](https://github.com/CGCL-codes/Toma/tree/15f74517668318369d567680d2089d8d2a417cdc/dataset).
Its Java function archive and CSVs come from the artifact for Feng, Suo, Wu,
Zou, Liu and Jin, [ICSE 2024](https://doi.org/10.1145/3597503.3639114).
For the original benchmark, see Svajlenko et al., *Towards a Big Data Curated
Benchmark of Inter-Project Code Clones* (ICSME 2014), and Svajlenko and Roy,
*Evaluating Clone Detection Tools with BigCloneBench* (ICSME 2015).
The [benchmark maintainers](https://github.com/clonebench/BigCloneBench#license)
license the database under CC BY-NC 4.0. Java source files retain their
individual licenses. Keep the downloads, generated snippets and pair lists
under the ignored raw-data directory or in a private external cache.

From the repository root, run:

```sh
python -m ml.integrity.code.prepare_data --download --cache ml/datasets/raw/toma/downloads --output ml/datasets/raw/toma/corpus --seed 42 --per-class 64
```

The preparer checks SHA-256 for seven files pinned to Git revision
`15f74517668318369d567680d2089d8d2a417cdc`. It reads Java members directly
from the ZIP, preserves their original line endings and uses the authors'
function IDs. The [release description](https://github.com/CGCL-codes/Toma/tree/15f74517668318369d567680d2089d8d2a417cdc#dataset)
maps its five numbered CSVs to T1, T2, strong T3, moderate T3 and weak T3/T4.
The T3 evaluation stratum draws half its quota from strong T3 and the rest
from moderate T3. Weak T3/T4 has an ambiguous subtype and contributes only
to grouping. Negative examples come exclusively from `nonclone.csv`.

The split is determined before pair sampling. Connect functions that share
identical source bytes or any published positive edge, including weak T3/T4.
Each resulting component gets its smallest numeric ID as its key. An even
first byte of SHA-256 of the ASCII string `seed:key` assigns the component
to validation, an odd byte to test. Keep a pair only when both endpoints
land in the same split. Within each stratum, sample pairs in a seeded
round-robin over the component of the smaller endpoint ID. This favors
coverage of different components and imposes a synthetic class prevalence.
The measured precision applies to this sample.

The generated `functions.jsonl` stores `id` and `code`. In `pairs.csv`,
`clone_type` identifies T1, T2, T3 or nonclone and `source_type` preserves
the original subtype. For bootstrap inference the `group` column joins
source components linked by a selected negative pair *after* splitting.
All observed dependencies from shared function IDs, identical sources and
published clone links therefore stay within one bootstrap group. Project
identities and undisclosed clone links can still introduce dependence.
`provenance.json` records the release checksums, eligible and selected
pair counts, seed and output hashes.

### Run the comparison

From the repository root, run the evaluator on the prepared sample:

```sh
python -m ml.integrity.code.evaluate --corpus ml/datasets/raw/toma/corpus --bootstrap 1000 --seed 42
```

The evaluator checks for duplicate pairs and cross-partition group or function-text
overlap. Validation selects the threshold from 0.10 to 1.00 in increments of
0.10. Test metrics in the [benchmark report](results.md) include all labelled
pairs, including short inputs for which the detector finds no fingerprint.
The validation grid, frozen cutoff and counts of defined bootstrap draws are
written to `ml/integrity/artifacts/code/experiment.json`.

## Algorithm

The lexer discards whitespace and comments, preserves Java keywords and
operators, and replaces identifiers, numbers, strings, characters and booleans
with their respective categories. Each token retains its spelling and its
half-open offsets in the original text. For another language, implement
`Lexer.tokenize(text)` and pass the lexer to `prepare`. The Java lexer accepts
method fragments, quoted literals and text blocks. It does not implement the
compiler's preprocessing of Unicode escape sequences or resolve contextual
keywords such as `record` against a syntax tree. An unterminated block comment
or literal raises a lexical error. Include that limitation when reporting
coverage on new corpora.

Winnowing forms five-token sequences and chooses the rightmost minimum 64-bit
BLAKE2b hash among each four consecutive sequences. Repeated minima at the
same position yield one fingerprint. Fragments with five to seven tokens use
one shorter window. Shorter fragments abstain. A contiguous match of eight
normalized tokens has a shared fingerprint under the default settings before
template filtering. An explicit instructor template suppresses every selected
five-token sequence also found in that template.

For scoring, the detector groups selected fingerprints into 200-token windows
spaced 100 tokens apart. The final window ends at the last token. It compares
distinct original token tuples behind the hashes and takes the highest local
Jaccard score. Hash collisions therefore cannot create overlap evidence. In
the best window pair, equal token sequences at a constant offset are merged
when they overlap or touch. Ties use the first best window pair in target and
source order. Repeated passages can have several valid alignments. Both span
endpoints count Unicode code points including the original line endings.

The comparator `exact_token_match` tests equality of complete token sequences
using original spelling after removing comments and whitespace. Renaming an
identifier can defeat this comparator. For both methods, independently written
solutions to a short exercise can share the same structure. Method choice and
thresholds should be interpreted alongside the exercise and approved template.

## Generate a review event

Provide two UTF-8 Java files matching the submissions' resolved `text_ref`
strings. Use the respective submission IDs, the target event's `context` JSON,
and a cutoff chosen on validation. The caller selects pairs from the same
assignment and supplies the later submission as the target. Copy the
validation-selected cutoff from the experiment report into `CUTOFF`.

```bash
python -m ml.integrity.code.to_risk_event \
  --target target.java --source source.java \
  --target-id sub_501 --source-id sub_377 --context context.json \
  --threshold "$CUTOFF" --output risk.json
```

The output follows [the event contract](../../../docs/03_architecture/events.md).
At or above the frozen cutoff the event carries level `medium` and the source
ID, verified similarity and aligned spans in `evidence.matches`. A separately
validated `--high-threshold` enables level `high`. Below the medium cutoff the
pair has low priority and the command writes JSON `null`, indicating no event.
It also writes `null` if the pair has no usable shared evidence. Supply
`--template instructor.java` to exclude instructor-provided code. The score
describes relative similarity for teacher review.

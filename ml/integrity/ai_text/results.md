# Offline English AI-text indicators

Each normalized HC3 prompt belongs to one deterministic 60/20/20 partition. Identical answer texts occur once, and answers with conflicting labels are discarded. A character trigram model estimates perplexity under generated training answers. The second model fits logistic regression on eleven length, vocabulary, sentence and punctuation features. Each method maximizes validation recall subject to a 5% human FPR budget. Thresholds are frozen before test.

| Partition | Human | Generated |
|---|---:|---:|
| train | 31788 | 15682 |
| validation | 10875 | 5394 |
| test | 10423 | 5168 |
| external | 91 | 0 |

| Method | Frozen rule | Test human FP / total (FPR, 95% CI) | Test AI recall | TOEFL human FP / total (FPR, 95% CI) |
|---|---|---|---:|---|
| Perplexity | PPL ≤ 7.34419 | 510/10423 (4.9%, 4.5%–5.3%) | 81.2% | 41/91 (45.1%, 34.4%–55.6%) |
| Stylometry | score ≥ 0.60642 | 513/10423 (4.9%, 4.5%–5.3%) | 70.7% | 1/91 (1.1%, 0.0%–3.3%) |

Intervals use 1,000 bootstrap samples of prompts for HC3 and essays for TOEFL. HC3 answers have no native-language labels. The external set contains human TOEFL essays from the authors’ bias study. Its topic, genre and length differ from HC3 answers, so a difference between the two FPRs cannot isolate the effect of language background. The experiment covers English only and produces no platform event.

[HC3 English, Guo et al.](https://huggingface.co/datasets/Hello-SimpleAI/HC3/tree/4d0ff18143b5a7e1b1e79beb540c04549d1e59d3) · [TOEFL human essays, Liang et al.](https://github.com/Weixin-Liang/ChatGPT-Detector-Bias/tree/1d05ed8242d4694a0ed7b73a79c52be8864470f9/Data_and_Results/Human_Data/TOEFL_real_91)

Reproduce with the original English `all.jsonl` and the TOEFL directory’s `data.json`. Raw answers and essays stay outside Git.

```sh
python -m ml.integrity.ai_text.evaluate --hc3 /path/to/all.jsonl --learners /path/to/data.json --out ml/integrity/ai_text/results.md
```

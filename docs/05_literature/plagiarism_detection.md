<!-- docs/05_literature/plagiarism_detection.md -->
# Literature review: plagiarism and AI-generated text indicators

## Problem statement

Examory must distinguish three related tasks. **Near-duplicate detection** identifies copied passages with minor edits; **semantic similarity** identifies related meaning despite changed wording. Neither establishes plagiarism: quotations, shared instructions, standard terminology and independent solutions can legitimately overlap. Source-code similarity additionally depends on programming language, compulsory templates and the small number of natural solutions to introductory exercises. **AI-text detection** estimates whether writing resembles a generator's outputs; it does not identify a copied source or establish prohibited assistance.

The scientific objective is therefore evidence-supported retrieval and risk ranking, with low false-positive rates. Evaluation must separate verbatim copying, partial copying, paraphrasing, legitimate similarity and machine generation. English-language results cannot be assumed to transfer to Kazakh or Russian. The decisions below are proposed project choices, not reported experimental results.

## Methods

### Shingling, MinHash and locality-sensitive hashing

A document becomes a set of overlapping token sequences, or shingles. For sets A and B, Jaccard similarity is |A ∩ B| / |A ∪ B|. MinHash estimates this quantity from compact signatures: under a random permutation, the probability of matching minima equals Jaccard similarity. Broder also distinguishes resemblance from directional containment, which matters when a short passage is copied into a much longer document. [1](https://doi.org/10.1109/SEQUEN.1997.666900)

Locality-sensitive hashing (LSH) retrieves likely similar signatures without comparing every pair; retrieval is approximate, so missed candidates remain possible. [2](https://www.vldb.org/conf/1999/P49.pdf) For the proposed banded MinHash implementation, use b bands of r signature values, giving idealized candidate probability 1 − (1 − s^r)^b at similarity s. This is a probability curve, not a guaranteed threshold. Verify candidate pairs with actual shingles and reconstruct spans using original offsets. Chunk-level matching should accompany document-level matching because low whole-document Jaccard can hide partial borrowing. Substantial paraphrasing destroys lexical shingles.

### TF-IDF and cosine similarity

TF-IDF reduces the influence of common terms and represents documents as sparse weighted vectors. Cosine similarity compares their direction rather than raw document length. It is a useful inexpensive baseline, especially with word and character n-grams, but shared assignment vocabulary can dominate the result. Reordered words may remain similar while synonym substitution and translation remain difficult. Fit vocabulary and inverse document frequencies on training/reference data only; threshold selection on test data would leak information. [3](https://doi.org/10.1016/0306-4573(88)90021-0)

### Sentence embeddings: SBERT and related encoders

Sentence-BERT (SBERT) uses siamese/triplet training to produce vectors comparable by cosine; the same work includes Sentence-RoBERTa. Such bi-encoders permit cached embeddings and efficient retrieval, unlike running a cross-encoder on every pair. Their semantic generalization can retrieve paraphrases missed by shingles, but semantic similarity is not evidence of copying. Short answers to the same question may correctly converge on the same meaning. [4](https://aclanthology.org/D19-1410/)

For this project, multilingual or distilled sentence encoders are later experimental alternatives. Evaluate each selected checkpoint separately by language and task, compare passages rather than truncating long submissions, and retain retrieved text as evidence. A high cosine score alone cannot justify a plagiarism allegation.

### Code: winnowing/MOSS, tokens, ASTs and CodeBERT

**Winnowing** selects representative hashes from sliding windows of k-grams. With window size w, matching normalized substrings of at least k + w − 1 units share a selected fingerprint, subject to consistent preprocessing and hash handling. The original paper describes its use in MOSS and explains how to suppress instructor-provided boilerplate. Fingerprints support localized overlap evidence, but do not establish common authorship. [5](https://theory.stanford.edu/~aiken/publications/papers/sigmod03.pdf)

**Token-based comparison** removes formatting and can normalize identifiers while retaining language constructs. JPlag compares token sequences using Greedy String Tiling; it demonstrates the value of structural token matches for student programs. Normalization improves resilience to cosmetic changes but can also merge independently written simple solutions. Preserve source mappings and measure false positives on template-heavy exercises. [6](https://www.jucs.org/jucs_8_11/finding_plagiarisms_among_a/Prechelt_L.html)

**AST-based methods** compare abstract syntax trees or subtrees, reducing sensitivity to surface spelling and formatting. Baxter et al. use subtree-based clone detection; extensions address near-matches. AST methods require a language-specific parser and normalization rules, and structural resemblance still does not prove equivalent behavior or copying. Arbitrary tree comparison is not interchangeable with fast subtree hashing. [7](https://leodemoura.github.io/files/ICSM98.pdf)

**CodeBERT** learns joint representations of code and natural language. Its original evaluation addresses code search and documentation generation, so its paper does not establish an off-the-shelf plagiarism detector. Applying embeddings to clone retrieval requires a defined pooling/fine-tuning method and clone-specific evaluation. Code transformations, long functions, unsupported languages and assignment leakage must be tested. [8](https://aclanthology.org/2020.findings-emnlp.139/)

### AI-text detection: statistical features, classifiers, watermarking and stylometry

**Perplexity** measures predictability under a specified reference language model: exp(−mean token log-probability). Low perplexity can occur in both generated and human writing. GLTR examines token likelihood/rank patterns as assistance to human judgment. [9](https://aclanthology.org/P19-3019/) “Burstiness” has no universal detector definition: the project should explicitly define it, for example as variation of sentence-level negative log-likelihood, and record tokenizer and model versions. It must not be marketed as a reliable human/AI boundary.

DetectGPT uses changes in log-probability under perturbed passages to estimate a curvature-related signal. It requires scoring access to an appropriate model and repeated perturbation/scoring operations; it is not simply a perplexity threshold. Its experimental results are task/model dependent. [10](https://proceedings.mlr.press/v202/mitchell23a.html)

**Supervised classifiers** learn from human/generated examples using lexical features or transformer representations. HC3 provides English/Chinese human–ChatGPT comparisons and evaluates statistical and neural detection systems. Its question-answer distributions and historical generator outputs do not directly represent AITU essays. Hold out prompts, authors and generator families when possible; assess new generators and human-edited outputs separately. [11](https://arxiv.org/abs/2301.07597)

**Watermarking** changes token selection during generation so a detector can test for a statistical pattern. Kirchenbauer et al. introduce a green-list watermark scheme. Detection depends on a compatible watermark and its parameters/key; arbitrary external model outputs may have none. Absence of a watermark is therefore not evidence of human authorship, and editing can weaken the signal. [12](https://proceedings.mlr.press/v202/kirchenbauer23a.html)

**Stylometry** examines character n-grams, function words, punctuation and syntactic habits. Authorship-attribution research offers useful features, but a writing-style change is not specific to AI use: genre, topic, language proficiency, revision and assistance can all change style. Reliable within-author comparison also needs sufficient consensually collected historical writing. [13](https://doi.org/10.1002/asi.21001)

**False positives for non-native writers:** Liang et al. tested seven detectors on 91 TOEFL essays and 88 US eighth-grade essays and reported a mean false-positive rate of 61.3% on the TOEFL set. This is evidence about those detectors and samples, not a universal rate for all present-day tools. Their findings make a single global perplexity threshold unsuitable for punitive use. Examory should audit errors by language and available consented language-background groups, report uncertainty, and abstain on insufficient text. [14](https://doi.org/10.1016/j.patter.2023.100779)

## Comparison table

Here n and m are token/character counts of two inputs; h is signature length; N is reference-document count; c is retrieved candidate count; z is sparse-vector nonzero count; d is embedding dimension; T(n) is one transformer forward-pass cost; K is perturbation count. Costs are indicative for the stated implementation, excluding training unless specified. Dense self-attention contributes quadratic sequence-length cost to T; indexing does not guarantee sublinear worst-case retrieval.

| Method | Input | Complexity / main cost | Robustness to paraphrasing or code rewriting | Suitability for MVP |
|---|---|---|---|---|
| Shingles + MinHash/LSH | Text or normalized code tokens | Basic signature O(nh); h-value band hashing O(h), candidate handling O(c), plus exact verification; c can reach N | Low for paraphrasing; handles small edits | Primary text baseline |
| TF-IDF + cosine | Sparse word/character n-grams | Feature counting O(n); one sorted sparse comparison O(zA + zB); exhaustive retrieval scales with N | Low–moderate for lexical edits | Cheap comparison baseline |
| SBERT / Sentence-RoBERTa | Text chunks | Encode O(T(n)); exact vector search O(Nd); approximate index adds recall tradeoff | Better semantic recall; topic-related false positives | Later ablation |
| Winnowing / MOSS approach | Normalized k-grams | O(n) with rolling hashes and deque window minima; matching cost depends on shared fingerprints | Resistant to formatting; weak against semantic rewrites | Local code fingerprint experiment |
| Token matching / JPlag approach | Language-aware token sequences | Tokenization O(n); Greedy String Tiling can be O((n+m)^3) in worst case; hashing improves typical behavior | Good for renaming/formatting; limited semantic equivalence | Small-cohort code baseline |
| AST subtree matching | Parsed code | Parsing plus O(n) bottom-up hashing under usual assumptions; candidate verification extra; general tree edit methods cost more | Moderate structural robustness | Defer parser/normalization burden |
| CodeBERT representations | Code chunks, optionally natural language | O(T(n)) encoding; O(Nd) exact vector search | Potentially useful; must validate on clones | Research extension |
| Perplexity / burstiness / DetectGPT | Text and reference model | One scoring pass approximately T(n); DetectGPT roughly K scoring and perturbation passes | Fragile under editing and domain shift | Offline research only |
| Supervised AI-text classifier | Text and labeled examples | Linear features can be O(n); transformer inference O(T(n)); training separate | Depends strongly on generator/domain coverage | Offline research only |
| Watermark detector | Text from compatible watermarked generator | Often O(n) token checks with keyed pseudorandom operations | Depends on editing, length and scheme | No general coverage |
| Stylometric classifier | Text and appropriate reference writing | Basic feature counting O(n), plus classifier cost | Sensitive to style and topic shifts | Exploratory only |

## Decision for MVP

Use **word shingles + MinHash/LSH followed by exact overlap verification** for text, with document and chunk retrieval. Begin with five-token shingles and a 128-value signature as tunable engineering defaults, not literature-derived optimum settings. Compare against exact search on a manageable cohort to measure candidate recall. Evaluate token normalization and local winnowing for code separately; exclude supplied templates and retain matched source spans. TF-IDF provides a cheap benchmark. Add embeddings only after measuring a clear benefit on held-out paraphrases and acceptable false positives.

Inputs follow `submission.created`: `submission_id`, `assignment_id`, `kind`, `language`, `text_ref`, `files`. A reviewable result uses `risk.plagiarism` with `evidence.matches[{submission_id, similarity, overlap_spans}]`. Keep the required `risk_id`, `target_type`, `target_id`, `score`, `level`, `model{name, version}`, `explanation`, and `evidence`. Similarity and normalized risk `score` are not calibrated probabilities of misconduct.

AI-origin experiments remain offline during the MVP. Any later `risk.ai_generated` signal must include the defined `evidence.features{perplexity, burstiness,...}` and methodological limitations. Teachers resolve cases through `risk.reviewed` (`confirmed` / `dismissed`); an unreviewed case is not a negative label. Record dismissals, independently audit non-alerted work, and use these outcomes to estimate false positives without treating detector output as ground truth.

## References

1. **`broder1997resemblance`** — Andrei Z. Broder (1997). “On the resemblance and containment of documents.” *Compression and Complexity of SEQUENCES 1997*, pp. 21–29. [DOI](https://doi.org/10.1109/SEQUEN.1997.666900).
2. **`gionis1999similarity`** — Aristides Gionis, Piotr Indyk, Rajeev Motwani (1999). “Similarity Search in High Dimensions via Hashing.” *Proceedings of the 25th International Conference on Very Large Data Bases*, pp. 518–529. [Publisher PDF](https://www.vldb.org/conf/1999/P49.pdf).
3. **`salton1988term`** — Gerard Salton, Christopher Buckley (1988). “Term-weighting approaches in automatic text retrieval.” *Information Processing & Management*, 24(5), 513–523. [DOI](https://doi.org/10.1016/0306-4573(88)90021-0).
4. **`reimers2019sentence`** — Nils Reimers, Iryna Gurevych (2019). “Sentence-BERT: Sentence Embeddings using Siamese BERT-Networks.” *EMNLP-IJCNLP*, pp. 3982–3992. [DOI](https://doi.org/10.18653/v1/D19-1410).
5. **`schleimer2003winnowing`** — Saul Schleimer, Daniel S. Wilkerson, Alex Aiken (2003). “Winnowing: Local Algorithms for Document Fingerprinting.” *ACM SIGMOD International Conference on Management of Data*, pp. 76–85. [DOI](https://doi.org/10.1145/872757.872770).
6. **`prechelt2002jplag`** — Lutz Prechelt, Guido Malpohl, Michael Philippsen (2002). “Finding Plagiarisms among a Set of Programs with JPlag.” *Journal of Universal Computer Science*, 8(11), 1016–1038. [DOI](https://doi.org/10.3217/jucs-008-11-1016).
7. **`baxter1998clone`** — Ira D. Baxter, Andrew Yahin, Leonardo Moura, Marcelo Sant'Anna, Lorraine Bier (1998). “Clone Detection Using Abstract Syntax Trees.” *International Conference on Software Maintenance*, pp. 368–377. [DOI](https://doi.org/10.1109/ICSM.1998.738528).
8. **`feng2020codebert`** — Zhangyin Feng, Daya Guo, Duyu Tang, Nan Duan, Xiaocheng Feng, Ming Gong, Linjun Shou, Bing Qin, Ting Liu, Daxin Jiang, Ming Zhou (2020). “CodeBERT: A Pre-Trained Model for Programming and Natural Languages.” *Findings of EMNLP 2020*, pp. 1536–1547. [DOI](https://doi.org/10.18653/v1/2020.findings-emnlp.139).
9. **`gehrmann2019gltr`** — Sebastian Gehrmann, Hendrik Strobelt, Alexander Rush (2019). “GLTR: Statistical Detection and Visualization of Generated Text.” *ACL: System Demonstrations*, pp. 111–116. [DOI](https://doi.org/10.18653/v1/P19-3019).
10. **`mitchell2023detectgpt`** — Eric Mitchell, Yoonho Lee, Alexander Khazatsky, Christopher D. Manning, Chelsea Finn (2023). “DetectGPT: Zero-Shot Machine-Generated Text Detection using Probability Curvature.” *ICML*, PMLR 202, 24950–24962. [Proceedings](https://proceedings.mlr.press/v202/mitchell23a.html).
11. **`guo2023hc3`** — Biyang Guo, Xin Zhang, Ziyuan Wang, Minqi Jiang, Jinran Nie, Yuxuan Ding, Jianwei Yue, Yupeng Wu (2023). “How Close is ChatGPT to Human Experts? Comparison Corpus, Evaluation, and Detection.” *arXiv preprint*, arXiv:2301.07597. [DOI](https://doi.org/10.48550/arXiv.2301.07597).
12. **`kirchenbauer2023watermark`** — John Kirchenbauer, Jonas Geiping, Yuxin Wen, Jonathan Katz, Ian Miers, Tom Goldstein (2023). “A Watermark for Large Language Models.” *ICML*, PMLR 202, 17061–17084. [Proceedings](https://proceedings.mlr.press/v202/kirchenbauer23a.html).
13. **`stamatatos2009survey`** — Efstathios Stamatatos (2009). “A Survey of Modern Authorship Attribution Methods.” *Journal of the American Society for Information Science and Technology*, 60(3), 538–556. [DOI](https://doi.org/10.1002/asi.21001).
14. **`liang2023bias`** — Weixin Liang, Mert Yuksekgonul, Yining Mao, Eric Wu, James Zou (2023). “GPT detectors are biased against non-native English writers.” *Patterns*, 4(7), 100779. [DOI](https://doi.org/10.1016/j.patter.2023.100779).

<!-- docs/05_literature/anomaly_detection.md -->
# Behavioral Anomaly Detection and Online Proctoring

## Problem statement and behavioral evidence

Examory should identify unusual assessment activity for teacher review. Statistical rarity is not evidence of intent: rapid answers can reflect expertise, long pauses can reflect accessibility needs, and simultaneous submissions can follow a shared deadline. The target is a reviewable `risk.behavior_anomaly` or `risk.proctoring` signal, never an automatic misconduct finding.

The shared `docs/03_architecture/events.md` contract (draft v0.1, checked against main commit `e60b7ac` on 20 October 2026) provides `quiz.answered` with `attempt_id`, `question_id`, `question_index`, `time_since_shown_ms`, `time_since_attempt_start_ms`, `is_change`, and `answer_hash`. The `quiz.submitted` payload contains `attempt_id`, `quiz_id`, `variant_id`, `score`, `max_score`, `duration_ms`, and `answers_changed`. Group events by `payload.attempt_id`; use `actor.user_id` and `occurred_at` for student histories. `context.session_id` denotes session context, not the attempt join key. Deduplicate by `event_id`; exclude invalid durations and distinguish missing observations from zeros. Compare scores after normalizing by `max_score`, and align items using question identifiers and assessment variants.

Four feature families are relevant:

- **Time per question:** first-response mean, median, dispersion, and proportion of unusually short responses. Response times are positively skewed; van der Linden models log-times using person speed and item time intensity [1]. Compare similar assessment forms and avoid pooling easy and difficult items without adjustment. `time_since_shown_ms` on an answer change is elapsed time from display under the generator convention, so changes must not be added as independent question durations.
- **Answer-change patterns:** `answers_changed`, the proportion of `is_change=true` events, and late change bursts. Revision is normal learning behavior; unusually dense changes need contextual corroboration.
- **Synchronous submissions:** small differences in `quiz.submitted.occurred_at`, reinforced by matching answer sequences when item alignment is known. Synchrony alone is weak evidence because deadlines and network batching produce clusters.
- **Score jumps:** change from the student's earlier comparable `score` values, interpreted with duration and revision history. Standardize only comparable score scales and forms. With insufficient history, mark the feature missing instead of treating a first attempt as suspicious.

Counts of `client.focus_lost`, `client.tab_switched`, and `client.paste` are possible supplementary signals from the contract, but the proposed quiz-only synthetic baseline does not generate them. They cannot be claimed as tested features.

## Detection methods

**Isolation Forest.** Random feature splits isolate observations; unusually short paths yield high anomaly scores [2]. Subsampling makes it economical for attempt-level numeric features. It needs neither known attack categories nor a distance metric. However, it can flag legitimate minority profiles and miss coordinated groups that resemble ordinary clusters. Its score measures atypicality, not cheating probability.

**One-Class SVM.** A kernel boundary estimates a normal-data region [3]. It can capture nonlinear combinations but needs feature scaling and careful `nu` and kernel-width selection. Contaminated training data, small cohorts, and quadratic kernel storage make it less convenient as the first baseline. The parameter `nu` does not provide a real-world misconduct prevalence estimate.

**Local Outlier Factor (LOF).** LOF compares each observation's local density with its neighbors [4]. It can accommodate normal groups with different densities, but neighbor count and scaling affect results. A colluding cluster may be locally dense and receive a low anomaly score. Evaluate held-out attempts using a novelty-detection configuration fitted only on training data.

**Autoencoders.** An encoder compresses normal feature vectors and a decoder reconstructs them; reconstruction error supplies an anomaly score [5]. Sequence models could later use timing and revision order. Additional capacity needs enough representative normal data and regularization. Good reconstruction does not imply legitimate behavior, and anomalies present during training may also be reconstructed well. A small synthetic study cannot establish superiority over simpler models.

**Collusion detection.** Wollack's answer-copying statistic evaluates unexpected matching responses while accounting for response probabilities [6]. This motivates combining answer agreement, uncommon shared wrong responses, and temporal proximity. Examory's `answer_hash` supports equality, not correctness; shared wrong answers require authorized item/answer-key information not present in the listed fields. Compare only aligned items with a consistent hash scheme. For the MVP, use time-window candidates and an explicit pairwise rule; label it an engineering heuristic, not an implementation of Wollack's statistic. Calibrate pairwise thresholds against same-exam null data and account for multiple comparisons.

## Online proctoring

### Face presence and gaze with MediaPipe

MediaPipe provides reusable perception components [7]. The original Face Mesh research predicts 468 facial vertices [8]; the current Face Landmarker bundle documents 478 landmarks, confidence thresholds, and video/live-stream modes [9]. Version and model choice must be recorded.

Face presence can support sustained no-face or multiple-face intervals. Smooth brief failures and separate unusable video from detected absence. Head orientation and eye-region landmarks can support coarse gaze proxies after calibration, but a face mesh is not a validated attention or cheating detector. Looking away may reflect thinking, reading permitted material, disability, camera position, or glasses. Evaluate face-presence errors under varied lighting, skin tones, occlusion, and hardware; avoid inferring emotion or intent.

### Virtual cameras

OBS's official documentation demonstrates that a virtual camera presents a composed scene as a webcam input [10]. Device/driver metadata and the brief's `exam.agent_signal` value `virtual_camera` can therefore support source review. Such metadata is neither unspoofable nor proof of deception; legitimate tools also use virtual cameras. RGB frames alone cannot reliably distinguish every physical camera from a virtual device delivering the same images. Treat available source signals and visual anomalies as separate observations, with uncertainty retained.

### Deepfake detection in video

FaceForensics++ studies learned facial-manipulation detection under compression [11]; DFDC broadens training and evaluation with varied face-swap videos [12]. A frame classifier may be aggregated across short windows, while temporal models can assess inconsistent motion. Temporal aggregation is a proposed extension here, not a result established by these two datasets.

Evaluation must split by identity and source video, never randomly distribute neighboring frames across train and test. Test held-out manipulation methods and webcam-like compression, dropped frames, resizing, and low illumination. A successful benchmark result does not guarantee detection of newer generators or replay attacks. Real-time use also requires measured frame-processing latency and coverage. Defer this capability until an independent consented evaluation supports a useful false-positive rate.

### Ethical and legal constraints

Coghlan, Miller, and Paterson identify privacy, fairness, autonomy, transparency, and trust concerns in automated proctoring [13]. Provide understandable evidence, teacher review, an appeal process, and reasonable assessment accommodations. Retain the minimum material needed for a stated purpose, control access, and evaluate error rates across relevant conditions. These are operational requirements, not evidence that a proposed model is fair.

For AITU, Kazakhstan's Law No. 94-V is the relevant starting point: assess consent or another lawful basis, confidentiality, purpose limitation, local storage under Article 12, and cross-border conditions under Article 16 [14]. Research use and automated processing also require checking the current provisions; student identifiers and video must not be published as an open dataset. Institutional review must establish the applicable requirements before collecting real student data.

The EU AI Act is a comparative reference, not automatically Kazakhstan law. If Article 2 brings a deployment within scope, Article 5(1)(f) restricts emotion inference in education, with specified exceptions; Annex III includes systems for detecting prohibited student behavior during tests [15]. A coarse geometric gaze estimate must not be presented as validated emotion inference.

## Comparison table

Notation: `n` attempts, `d` features, `T` trees, `psi` samples/tree, `q` aligned questions, `E` epochs, `P` dense-network parameters, and `F` processed frames. Costs are approximate and depend on implementation.

| Method | Input | Computational cost | Main limitation | MVP suitability |
|---|---|---|---|---|
| Isolation Forest | Attempt feature vectors | Fit `O(T psi log psi)`; score `O(T log psi)` per vector for depth-limited trees | Minority profiles; dense anomaly groups | Primary baseline |
| RBF One-Class SVM | Scaled vectors | Kernel construction `O(n²d)`; solver can approach cubic time | Scaling, tuning, memory | Comparator on small data |
| LOF | Scaled vectors | Brute-force neighbors `O(n²d)` during fit | Neighborhood sensitivity; dense collusion | Secondary comparator |
| Autoencoder | Vectors or sequences | Dense fit approximately `O(E n P)`; score `O(P)` | Data demand and reconstruction of anomalies | Defer |
| Pairwise collusion rule | Aligned answers and times | Exhaustive `O(n²q)`; time-window filtering reduces candidates | Common answers, deadlines, multiplicity | Interpretable supplement |
| MediaPipe face presence/gaze proxy | Video frames | `O(F C_face)` for fixed model cost `C_face` | Occlusion and calibration; gaze ambiguity | Presence pilot; gaze exploratory |
| Virtual-camera signal checks | Device evidence and `exam.agent_signal` | Linear in inspected signals/devices | Spoofable metadata; legitimate uses | Review-only supplement |
| Deepfake classifier | Frames or temporal windows | Frame baseline `O(F C_model)`; temporal cost model-dependent | Unseen manipulations and domain shift | Defer |

## Decision for MVP

Use Isolation Forest on timing, change-pattern, duration, and comparable score-history features, with a separate time-window collusion heuristic. Train on normal synthetic profiles, stress-test contamination, and validate using unseen users, sessions, and anomaly parameters. Fit normalization and thresholds using training/validation data only. Synthetic success establishes pipeline behavior, not real misconduct detection.

Publish `risk.behavior_anomaly` using `risk_id`, `target_type`, `target_id`, `score`, `level`, `model{name, version}`, `explanation`, and `evidence`. Set `target_type="attempt"` and `target_id` to the attempt identifier. Preserve `evidence.features.mean_answer_time_ms`, `z_score`, and `synchronous_with[]`, whose entries are peer `attempt_id` values. For consistency with the baseline plan, define `z_score` as standardized normalized-score delta against a training reference, with a variance floor and null for missing history. Map anomaly scores to `[0,1]` using the empirical training-normal score CDF, and choose alert thresholds on validation. These are relative risk rankings, not calibrated misconduct probabilities. Do not expose synthetic `is_anomaly` or `anomaly_type` as predictive features.

Pilot sustained face-presence checks separately, recording `risk.proctoring.evidence.segments[{from_ms, to_ms, kind}]` with timings relative to the recording. Defer automated gaze judgments, virtual-camera guilt decisions, and deepfake conclusions.

Report precision, recall, F1, false-positive rate, review workload, and uncertainty intervals at a threshold fixed before testing. Join `risk.reviewed` by `risk_id` and use its `decision` values `confirmed` and `dismissed` as teacher adjudications, retaining `reviewed_by` and `comment`; preserve disagreements and missing reviews. Reviewing only flagged attempts estimates alert precision, not population recall or false-positive rate. Randomly audit unflagged attempts to measure missed cases and selection bias.

## References

1. `vanderlinden2006lognormal` — Wim J. van der Linden. 2006. “A Lognormal Model for Response Times on Test Items.” *Journal of Educational and Behavioral Statistics*, 31(2), 181–204. [DOI](https://doi.org/10.3102/10769986031002181).
2. `liu2008isolation` — Fei Tony Liu, Kai Ming Ting, and Zhi-Hua Zhou. 2008. “Isolation Forest.” *IEEE ICDM*, 413–422. [DOI](https://doi.org/10.1109/ICDM.2008.17).
3. `scholkopf2001support` — Bernhard Schölkopf, John C. Platt, John Shawe-Taylor, Alexander J. Smola, and Robert C. Williamson. 2001. “Estimating the Support of a High-Dimensional Distribution.” *Neural Computation*, 13(7), 1443–1471. [DOI](https://doi.org/10.1162/089976601750264965).
4. `breunig2000lof` — Markus M. Breunig, Hans-Peter Kriegel, Raymond T. Ng, and Jörg Sander. 2000. “LOF: Identifying Density-Based Local Outliers.” *ACM SIGMOD*, 93–104. [DOI](https://doi.org/10.1145/342009.335388).
5. `sakurada2014autoencoders` — Mayu Sakurada and Takehisa Yairi. 2014. “Anomaly Detection Using Autoencoders with Nonlinear Dimensionality Reduction.” *MLSDA 2014: 2nd Workshop on Machine Learning for Sensory Data Analysis*, 4–11. [DOI](https://doi.org/10.1145/2689746.2689747).
6. `wollack1997copying` — James A. Wollack. 1997. “A Nominal Response Model Approach for Detecting Answer Copying.” *Applied Psychological Measurement*, 21(4), 307–320. [DOI](https://doi.org/10.1177/01466216970214002).
7. `lugaresi2019mediapipe` — Camillo Lugaresi, Jiuqiang Tang, Hadon Nash, Chris McClanahan, Esha Uboweja, Michael Hays, Fan Zhang, Chuo-Ling Chang, Ming Guang Yong, Juhyun Lee, Wan-Teh Chang, Wei Hua, Manfred Georg, and Matthias Grundmann. 2019. “MediaPipe: A Framework for Building Perception Pipelines.” *arXiv preprint*. [arXiv:1906.08172](https://arxiv.org/abs/1906.08172).
8. `kartynnik2019geometry` — Yury Kartynnik, Artsiom Ablavatski, Ivan Grishchenko, and Matthias Grundmann. 2019. “Real-time Facial Surface Geometry from Monocular Video on Mobile GPUs.” *arXiv preprint*. [arXiv:1907.06724](https://arxiv.org/abs/1907.06724).
9. `google2026facelandmarker` — Google. 2026. “Face Landmark Detection Guide.” *Google AI Edge documentation*, updated 9 October 2026. [Official documentation](https://developers.google.com/edge/mediapipe/solutions/vision/face_landmarker).
10. `obs2022virtualcamera` — OBS Project. 2022. “Virtual Camera Guide.” *OBS Knowledge Base*, 31 August. [Official guide](https://obsproject.com/kb/virtual-camera-guide).
11. `rossler2019faceforensics` — Andreas Rössler, Davide Cozzolino, Luisa Verdoliva, Christian Riess, Justus Thies, and Matthias Nießner. 2019. “FaceForensics++: Learning to Detect Manipulated Facial Images.” *IEEE/CVF ICCV*, 1–11. [DOI](https://doi.org/10.1109/ICCV.2019.00009).
12. `dolhansky2020dfdc` — Brian Dolhansky, Joanna Bitton, Ben Pflaum, Jikuo Lu, Russ Howes, Menglin Wang, and Cristian Canton Ferrer. 2020. “The DeepFake Detection Challenge (DFDC) Dataset.” *arXiv preprint*. [arXiv:2006.07397](https://arxiv.org/abs/2006.07397).
13. `coghlan2021proctor` — Simon Coghlan, Tim Miller, and Jeannie Paterson. 2021. “Good Proctor or ‘Big Brother’? Ethics of Online Exam Supervision Technologies.” *Philosophy & Technology*, 34, 1581–1606. [DOI](https://doi.org/10.1007/s13347-021-00476-1).
14. `kazakhstan2013personal` — Republic of Kazakhstan. 2013, as amended. “On Personal Data and their Protection.” *Law No. 94-V of 21 May 2013; Adilet Legal Information System*. [Official text](https://old.adilet.zan.kz/eng/docs/Z1300000094).
15. `european2024aiact` — European Parliament and Council of the European Union. 2024. “Regulation (EU) 2024/1689 (Artificial Intelligence Act).” *Official Journal of the European Union*, L, 12 July 2024. [Official text](https://eur-lex.europa.eu/legal-content/EN/TXT/PDF/?uri=OJ%3AL_202401689).

Online documentation and legal texts checked on 20 October 2026.

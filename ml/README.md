# ML modules

Owner: Akhanov Yerdaulet.

Planned layout:

```
ml/
  datasets/      download scripts and synthetic data generators (raw data is not committed)
  notebooks/     experiments
  integrity/     plagiarism and AI-text detection service (Python)
  anomaly/       behavioral anomaly detection service (Python)
  proctoring/    webcam analysis (OpenCV, MediaPipe)
  inference/     ONNX Runtime inference service (Rust, Axum)
```

Every model is evaluated with precision, recall and false-positive rate; results go to `ml/notebooks/results.md`.

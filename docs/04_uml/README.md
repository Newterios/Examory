# UML models

Owner: Shadiyarbek Syrym.

Store each diagram as PlantUML source (`*.puml`) next to its exported PNG. Render locally with:

```bash
docker run --rm -v "$PWD":/work -w /work plantuml/plantuml -tpng *.puml
```

Planned diagrams: use case (student, teacher, admin), domain data model, sequence diagrams for assignment submission with integrity check, quiz attempt with anomaly detection, exam session with proctoring.


## Diagram set (stage 1)

| Diagram | Source | Export |
|---|---|---|
| System context (C4) | `context.puml` | `context.png` |
| Component diagram (draft, C4) | `component.puml` | `component.png` |
| Use case | `use_case.puml` | `use_case.png` |
| Conceptual data model | `data_model.puml` | `data_model.png` |
| Sequence: submission with integrity review | `seq_submission_integrity.puml` | `seq_submission_integrity.png` |
| Sequence: quiz attempt with anomaly review | `seq_quiz_anomaly.puml` | `seq_quiz_anomaly.png` |
| Sequence: consented exam with proctoring | `seq_exam_proctoring.puml` | `seq_exam_proctoring.png` |

C4 diagrams use the PlantUML standard library (`!include <C4/C4_Context>`), no network access needed to render.

# UML models

Owner: Shadiyarbek Syrym.

Store each diagram as PlantUML source (`*.puml`) next to its exported PNG. Render locally with:

```bash
docker run --rm -v "$PWD":/work -w /work plantuml/plantuml -tpng *.puml
```

Planned diagrams: use case (student, teacher, admin), domain data model, sequence diagrams for assignment submission with integrity check, quiz attempt with anomaly detection, exam session with proctoring.

# Examory UX mockups (LMS-23)

The [editable Figma file](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X) contains four desktop mockups for the key Examory scenarios. The screens use synthetic names and results and follow [the dashboard specification](dashboards_spec.md) and the [v0.2 event contract](../../03_architecture/events.md).

| Screen | Figma frame | PNG export | Scenario |
|---|---|---|---|
| Student dashboard | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-2) | [student_dashboard.png](student_dashboard.png) | Review progress, weak topics, recommendations, deadlines, grades, and exam readiness. |
| Teacher dashboard | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-3) | [teacher_dashboard.png](teacher_dashboard.png) | See group performance, learning-support needs, pending risk reviews, rubric suggestions, and variant comparison. |
| Risk review | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-4) | [risk_review.png](risk_review.png) | Inspect evidence and enter a rationale before confirming or dismissing a signal. The decision is a human review, not a grade or sanction. |
| Proctoring consent | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-5) | [proctoring_consent.png](proctoring_consent.png) | Read the monitoring disclosure, make separate webcam and client-agent choices, or request an alternative. |

The PNGs are 1440 × 900 exports of the Figma frames. They are static review artifacts; controls illustrate intended interactions and states. The consent screen shows both options unchecked and the continue action disabled. The risk review screen labels the score as a relative ranking, and its buttons correspond to `risk.reviewed` decisions `confirmed` and `dismissed`.

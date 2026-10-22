# Examory UX mockups (LMS-23)

The [Figma file](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X) contains four editable desktop screens, a student mobile layout and an interaction-state sheet. The student Summary focuses on the next lesson, deadlines and weak topics. The teacher, submission review and consent screens share Geist typography, neutral surfaces, navy accents and white navigation. All names and learning data are fictional examples for **Algorithms and Data Structures**. Link access is set to **Anyone can view**, verified on 23 October 2026.

## Latest student revision

The PNGs below were exported from the Figma editor. The Figma file contains editable native layers; the accompanying SVG exports are vector references and may outline text. Use the Figma frames to edit the design.

| Artifact | Figma frame | PNG | SVG reference | Dimensions |
|---|---|---|---|---|
| Student desktop Summary | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-2) | [Preview](student_dashboard.png) | [SVG](student_dashboard.svg) | 1440 × 1024 |
| Student mobile Summary | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=38-559) | [Preview](student_dashboard_mobile.png) | [SVG](student_dashboard_mobile.svg) | 375 × 1702 |
| Default, hover, focus-visible and disabled states | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=38-657) | [Preview](student_interaction_states.png) | [SVG](student_interaction_states.svg) | 1160 × 1047 |

See [student design tokens](student_design_tokens.md) for colors, type, spacing, radii, grid and interaction specifications. The Grades and Exam tabs indicate separate views; Summary presents Next up, Deadlines and Needs attention. Dates use local Kazakhstan time without a location or timezone selector. These static mockups specify intended interaction; they do not demonstrate working navigation, keyboard behavior or accessibility compliance.

## Other desktop screens

All four desktop frames use the same 1440 × 1024 px canvas. Each desktop sidebar places the account at the bottom; sample group labels use SE-2301. All four desktop screens include matching editable Lucide sidebar icons, with the active section highlighted. The student screen also includes thin content dividers. The teacher overview uses three filters and three primary metrics, with compact support, review, rubric and variant summaries; detailed criteria and evidence are accessed through their actions. Submission review keeps the matched excerpt and required decision controls prominent, with a compact linked-attempt summary. Their PNGs were exported from the Figma editor. Use [dashboards_spec.md](dashboards_spec.md) for functional requirements and [events.md](../../03_architecture/events.md) for event semantics.

| Screen | Figma frame | PNG | Dimensions |
|---|---|---|---|
| Teacher dashboard | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-3) | [teacher_dashboard.png](teacher_dashboard.png) | 1440 × 1024 |
| Submission review | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-4) | [submission_review.png](submission_review.png) | 1440 × 1024 |
| Proctoring consent | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-5) | [proctoring_consent.png](proctoring_consent.png) | 1440 × 1024 |

[risk_review.png](risk_review.png) is retained as an alias of `submission_review.png` for existing references. Risk review remains a human decision, separate from grading or sanctions; buttons correspond to `risk.reviewed` decisions `confirmed` and `dismissed`. The consent mockup presents separate webcam and agent choices and fixed timezone metadata without a selector. Icons use the [Lucide license](assets/Lucide-LICENSE.txt).

## Review checks

| Step | Verified content |
|---|---|
| 1. Student Summary | Next lesson, 18/24 material completion (75%), two November deadlines, three weak topics and a recommended reading. The mobile layout preserves the same reading order. Grades and Exam are separate tab destinations; their detail views are outside these four mockups. |
| 2. Teacher dashboard | Course, group and period filters; group performance with denominators; two learning-support examples; pending work; an advisory rubric suggestion; variant rates, cohort sizes and comparison windows. Detailed evidence and criteria are reached through the displayed actions. |
| 3. Submission review | Highlighted matching passages, source context, explanation, a linked behavior-anomaly attempt, required decision reason and Confirm signal / Dismiss signal actions. Pending review, Confirmed and Dismissed variants are available in the Figma risk-badge components. |
| 4. Proctoring consent | Separate unchecked webcam and client-agent choices, collection purpose, storage/access notice, withdrawal explanation, disabled Agree until both permissions are selected, and Decline with an alternative assessment route. |

All six PNG exports were checked against a fresh Figma export; desktop dimensions, SVG syntax and local documentation links were also checked. The current overview density follows the later simplification requests. Static frames do not verify working tabs, form validation, keyboard navigation, assistive technology behavior or service authorization; these require implementation testing. The approved retention duration belongs in the institutional privacy notice linked from the consent screen.

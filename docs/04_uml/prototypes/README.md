# Examory UX mockups (LMS-23)

The [Figma file](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X) contains the earlier four desktop screens. The latest student revision below follows the 23 October 2026 brief: a restrained course Summary focused on the next lesson, deadlines and weak topics, with a 375 px mobile layout. It uses fictional learning data for **Algorithms and Data Structures**.

## Latest student revision

These PNGs were rendered locally from the accompanying SVGs. **They are not Figma exports and are not yet synced to the Figma file**: MCP tool access is currently quota-limited, and the available browser account has view access. The SVGs retain text and vector shapes; use Geist when editing them. Native Figma components and Auto Layout still require a Figma editing session.

| Artifact | PNG | Editable SVG | Dimensions |
|---|---|---|---|
| Student desktop Summary | [Preview](student_dashboard.png) | [SVG](student_dashboard.svg) | 1440 × 900 |
| Student mobile Summary | [Preview](student_dashboard_mobile.png) | [SVG](student_dashboard_mobile.svg) | 375 × 1512 |
| Hover, focus-visible and disabled states | [Preview](student_interaction_states.png) | [SVG](student_interaction_states.svg) | 1040 × 568 |

See [student design tokens](student_design_tokens.md) for colors, type, spacing, radii, grid and interaction specifications. Grades and exam details are reached through tabs; Summary presents Next up, Deadlines and Needs attention. Dates use local Kazakhstan time without a location selector. Static mockups specify intended interaction; they do not demonstrate working accessibility behavior.

## Earlier screens

These assets remain from the earlier iteration and have not received the latest student visual treatment. Use [dashboards_spec.md](dashboards_spec.md) for functional requirements and [events.md](../../03_architecture/events.md) for event semantics.

| Screen | Figma frame | Earlier PNG |
|---|---|---|
| Student dashboard before this revision | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-2) | Replaced by the local Summary preview above |
| Teacher dashboard | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-3) | [teacher_dashboard.png](teacher_dashboard.png) |
| Risk review | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-4) | [risk_review.png](risk_review.png) |
| Proctoring consent | [Open frame](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X?node-id=3-5) | [proctoring_consent.png](proctoring_consent.png) |

The three earlier PNGs are 1440 × 900. Risk review remains a human decision, separate from grading or sanctions; buttons correspond to `risk.reviewed` decisions `confirmed` and `dismissed`. The consent mockup presents separate webcam and agent choices.

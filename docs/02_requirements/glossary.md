<!-- docs/02_requirements/glossary.md -->
# Examory Glossary

| Term | Definition |
|---|---|
| Examory | The AI-powered LMS for academic integrity, personalized learning analytics, and anomaly detection described by this project. |
| LMS | Learning management system supporting courses, materials, assignments, quizzes, grades, and learning activity. |
| User | An account with one or more student, teacher, or admin roles and explicitly scoped access permissions. |
| Group | A set of students used for course enrollment, permission scoping, and aggregated teaching analytics. |
| Course | An educational offering containing modules, learning materials, assignments, and quizzes. |
| Module | An ordered instructional unit within a Course. |
| Material | A learning resource in a Module; its viewing and completion activity is represented by `material.viewed`. |
| Assignment | A task with instructions, submission rules, a deadline, and optional assessment Rubric. |
| Submission | A student's submitted work for an Assignment, including file references and the submission attempt number. |
| Quiz | An assessment containing questions, assigned variants, scoring rules, and a time limit. |
| Question | An individual assessment item with a topic mapping used to interpret learning outcomes. |
| Question bank | A teacher-managed collection of Questions available for quiz construction and reuse. |
| Variant | A specific set or arrangement of Questions assigned for a Quiz and identified by `variant_id`. |
| Attempt | One student's execution of a Quiz, from `quiz.started` to `quiz.submitted`. |
| Answer | A student's response to a Question within an Attempt; answer activity is represented by `quiz.answered`. |
| Rubric | Assessment criteria and score limits used to explain and structure grading. |
| Grade | The recorded assessment result, including criterion scores when a Rubric applies; publication produces `grade.assigned`. |
| RiskSignal | A reviewable concern with a target, score, level, explanation, and evidence; it is not a misconduct verdict. |
| Academic integrity | The assessment of possible text or code borrowing and AI-text indicators, with final interpretation by a teacher. |
| Human-in-the-loop | The rule that a teacher reviews automated signals and makes the final academic decision. |
| ExamSession | A controlled examination period managed by `exam-sessions`, with consent, lifecycle state, and associated signals. |
| Client agent | The Windows examination client that supplies disclosed session telemetry through `exam.agent_signal`. |
| Proctoring | Consented observation of an ExamSession using webcam and client agent inputs to produce reviewable `risk.proctoring` signals. |
| Consent | A student's recorded agreement to the disclosed webcam and Windows client agent processing before an ExamSession starts. |
| Event | A versioned record of an action with `event_id`, `event_type`, `version`, `occurred_at`, `producer`, `actor`, `context`, and `payload`. |
| Kafka topic | A named Apache Kafka stream; the analytics service consumes `lms.quizzes`, `lms.submissions`, `lms.grades`, and `lms.courses`. |
| Personalized analytics | Student-specific progress, weak-topic findings, and recommendations derived from authorized learning activity. |
| Group risk | A learning-support indication of students who may need assistance, published as `analytics.group_risk`; it is distinct from misconduct risk. |
| Grade suggestion | An explained rubric-based proposal published as `analytics.grade_suggestion`; teacher confirmation is required to assign a Grade. |
| Variant leak | A possible exposure of a quiz Variant, indicated by a change in its pass-rate pattern and published as `risk.variant_leak`. |
| Anomaly detection | Identification of unusual timing, synchronized submissions, or result changes, producing `risk.behavior_anomaly` for review. |
| Retention | A documented limit on how long an identified data category remains stored before deletion or approved anonymization. |

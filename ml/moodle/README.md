# Moodle research extract

Run `python -m ml.moodle.convert --attempts attempts.csv --steps responses.csv --output events.csv --key-file /private/study.key --namespace school-pilot` from the repository root. Generate a random binary key of at least 32 bytes outside the repository and keep it with the school custodian. Run the converter inside the school’s data boundary. Only the converted CSV and aggregate report are intended for analysis.

## What to request

Request two UTF-8 CSV extracts joined on the local attempt key. Moodle's Grades and Responses reports are starting points. A normal download can lack stable attempt/option IDs, group membership for the assessment, step history or first-display timing. The custodian must export the missing join fields from attempt metadata, question-engine history and any instrumented display log. Localized headings should be mapped once by the custodian. [Moodle quiz reports](https://docs.moodle.org/en/Quiz_reports)

| Extract | Required columns and interpretation |
|---|---|
| Finished attempts | `attempt_id,user_id,course_id,group_id,quiz_id,variant_id,start,finish,score,max_score,question_count,time_limit_sec,reason`. Use the group at assessment time and a stable form/variant identifier. Timestamps are Unix seconds or timezone-qualified ISO 8601. A zero time limit means unlimited. Supply the actual completion reason, `submitted` or `timed_out`. |
| Saved choices | `attempt_id,question_id,slot,sequence,shown,saved,options`. Use a one-based variant slot and the question-engine step sequence within that slot. Supply stable option IDs as a JSON array and the first observed display time in `shown`. A blank display time is allowed. Supply all actual saved choices including revisions. A final-only export cannot recover revision counts. |

The input is restricted to choice questions and finished attempts with complete saved-choice history. Repeated identical saves collapse. When no display observation exists for a question, its answer events are omitted and counted. The real submission still records the transitions in the saved-choice history. Unlimited or unrepresentable time limits omit the start event. Fractional scores require an explicit integer scale such as `--score-scale 100`, applied equally to score and maximum. Downstream v1 models that require one point on every item may abstain.

The output replaces every subject, group and content identifier with a type-separated HMAC, shifts the whole study by the same keyed number of days, and derives stable time-consistent UUIDv7 event IDs. A shared shift preserves synchronous submissions. Equal choices have comparable hashes inside the same study. Do not request names, emails, contact details, IP addresses, device identifiers, free-text answers, question wording, answer keys or browser logs. The key and identifiable exports stay private. Pseudonymized event sequences can still disclose rare behavior and should receive restricted access.

Verification uses a synthetic fixture with an extra email column, an identical save, a revision, a missing display observation and fractional points. Run `python -m unittest ml.moodle.test_convert`.

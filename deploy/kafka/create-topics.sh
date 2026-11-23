#!/bin/sh
# Creates platform topics with the retention from events.md §2 (idempotent).
set -eu
BS="${KAFKA_BOOTSTRAP:-kafka:19092}"
T=/opt/kafka/bin/kafka-topics.sh
day=86400000
mk() {
  $T --bootstrap-server "$BS" --create --if-not-exists --topic "$1" \
     --partitions "$2" --replication-factor 1 --config retention.ms="$3"
}
mk lms.users 3 $((30 * day))
mk lms.courses 3 $((90 * day))
mk lms.submissions 3 $((365 * day))
mk lms.quizzes 6 $((365 * day))
mk lms.grades 3 $((365 * day))
mk lms.exams 3 $((90 * day))
mk ai.risk 3 $((365 * day))
mk ai.analytics 3 $((90 * day))
echo "topics ready"

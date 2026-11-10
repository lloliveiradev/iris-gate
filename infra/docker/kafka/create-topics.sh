#!/bin/sh
# Creates every topic listed in topics.txt. Idempotent (--if-not-exists), so a
# second `make up` is safe. Run by the one-shot `kafka-init` Compose service.
set -eu

BROKER="${KAFKA_BOOTSTRAP:-kafka:9092}"
TOPICS_FILE="${TOPICS_FILE:-$(dirname "$0")/topics.txt}"

grep -v '^[[:space:]]*#' "$TOPICS_FILE" | while read -r topic partitions; do
  [ -z "$topic" ] && continue
  /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server "$BROKER" \
    --create --if-not-exists \
    --topic "$topic" \
    --partitions "${partitions:-3}" \
    --replication-factor 1
done

echo "kafka-init: all topics provisioned"

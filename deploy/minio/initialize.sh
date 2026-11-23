#!/bin/sh
# Development bootstrap. Service accounts receive only their designated prefix.
set -eu
mc=/usr/local/bin/app
umask 077
export MC_CONFIG_DIR=/tmp/mc
for i in $(seq 1 60); do
  if "$mc" alias set local http://minio:9000 examory "$MINIO_ROOT_PASSWORD" >/dev/null 2>&1; then break; fi
  sleep 1
done
"$mc" mb --ignore-existing local/submissions
"$mc" anonymous set none local/submissions
for role in reader writer; do
  case "$role" in
    reader) user=integrity_reader; password=$INTEGRITY_S3_SECRET ;;
    writer) user=files_writer; password=$FILES_S3_SECRET ;;
  esac
  "$mc" admin policy create local "submissions-$role" "/policies/$role.json"
  "$mc" admin user add local "$user" "$password" >/dev/null
  "$mc" admin policy attach local "submissions-$role" --user "$user"
done

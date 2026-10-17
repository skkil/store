#!/bin/sh
set -eu
for file in $(ls /migrations/V*__*.sql | sort -V); do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$file"
done

#!/usr/bin/env bash
# Migration files must be named <14-digit timestamp>_<snake_name>.sql, be unique and non-empty.
set -euo pipefail
dir="supabase/migrations"; rc=0; prev=""
shopt -s nullglob
for f in "$dir"/*; do
  b=$(basename "$f"); [ "$b" = ".gitkeep" ] && continue
  if ! [[ "$b" =~ ^[0-9]{14}_[a-z0-9_]+\.sql$ ]]; then echo "bad name: $b"; rc=1; continue; fi
  [ -s "$f" ] || { echo "empty: $b"; rc=1; }
  v=${b%%_*}; [ "$v" = "$prev" ] && { echo "duplicate version: $v"; rc=1; }; prev=$v
done
exit $rc

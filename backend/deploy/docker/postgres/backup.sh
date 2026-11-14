#!/bin/sh
# Daily dump of the whole database into /backups, one file per weekday
# (fin-1.dump is Monday), so the last 7 days stay and nothing needs deleting.
# With BACKUP_COPY_DIR set, each dump is also written to /copy (another disk,
# a NAS, a folder a cloud client syncs), encrypted when BACKUP_PASSPHRASE is set.
# Once a week the dump is restored into a scratch database to prove it works.
# Every step is logged in sys.backup_run for the data health screen.
set -u
umask 077
conn="-h postgres -p $POSTGRES_PORT -U $POSTGRES_USER"
check_db=fin_restore_check

sql() { psql $conn -d "$1" -qtAX -v ON_ERROR_STOP=1 -c "$2"; }

# log kind ok detail; details are fixed text and numbers, never quotes.
log() {
  sql "$POSTGRES_BASE" "INSERT INTO sys.backup_run (kind, ok, detail) VALUES ('$1', $2, '$3')" >/dev/null 2>&1 ||
    echo "backup: could not log $1" >&2
}

copy() {
  if [ -n "${BACKUP_PASSPHRASE:-}" ]; then
    out="/copy/$2.enc"
    openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass env:BACKUP_PASSPHRASE -in "$1" -out "$out.tmp" &&
      mv -f "$out.tmp" "$out" && { log copy true "$2.enc, encrypted"; return; }
  else
    out="/copy/$2"
    cp "$1" "$out.tmp" && mv -f "$out.tmp" "$out" && { log copy true "$2, not encrypted"; return; }
  fi
  rm -f "$out.tmp"
  log copy false "could not write to the copy folder"
}

restore_check() {
  dropdb $conn --if-exists "$check_db" 2>/dev/null
  if ! createdb $conn "$check_db"; then
    log restore_check false "could not create the scratch database"
    return
  fi
  # Extensions such as pg_cron warn in a scratch database; the count below is the proof.
  pg_restore $conn -d "$check_db" --no-owner --no-privileges "$1" >/dev/null 2>&1
  n=$(sql "$check_db" "SELECT count(*) FROM app.fin_transaction" 2>/dev/null)
  dropdb $conn --if-exists "$check_db"
  if [ -n "$n" ]; then log restore_check true "$n transactions restored"; else log restore_check false "the dump did not restore"; fi
}

while true; do
  # Started together with the database: wait until it accepts connections.
  until pg_isready $conn -d "$POSTGRES_BASE" -q; do sleep 5; done
  day=$(date +%u)
  f="/backups/fin-$day.dump"
  if pg_dump $conn -d "$POSTGRES_BASE" -Fc -f "$f.tmp" && mv -f "$f.tmp" "$f"; then
    log dump true "fin-$day.dump, $(du -k "$f" | cut -f1) KB"
    [ -n "${BACKUP_COPY_DIR:-}" ] && copy "$f" "fin-$day.dump"
    due=$(sql "$POSTGRES_BASE" "SELECT NOT EXISTS (SELECT 1 FROM sys.backup_run WHERE kind = 'restore_check' AND ok AND created_at > now() - interval '6 days')" 2>/dev/null)
    [ "$due" != f ] && restore_check "$f"
  else
    rm -f "$f.tmp"
    log dump false "pg_dump failed, trying again in 1 hour"
    sleep 3600
    continue
  fi
  sleep 86400
done

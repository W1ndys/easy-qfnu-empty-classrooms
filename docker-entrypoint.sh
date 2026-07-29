#!/bin/sh
set -eu

mkdir -p /app/logs
chown -R app:app /app/logs

if [ "$#" -gt 0 ]; then
    exec su-exec app:app "$@"
fi

exec su-exec app:app /app/easy-qfnu-kjs

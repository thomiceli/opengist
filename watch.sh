#!/bin/bash
set -euo pipefail

if [ "$1" == "task" ]; then
    task watch_frontend &
    task watch_backend &
else
    make watch_frontend &
    make watch_backend &
fi

trap 'kill $(jobs -p)' EXIT
wait

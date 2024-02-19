#!/bin/sh
set -euo pipefail

make watch_frontend &
make watch_backend &

trap 'kill $(jobs -p)' EXIT
wait

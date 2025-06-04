#!/bin/sh
set -euo pipefail

# Start background processes
make watch_frontend &
FRONTEND_PID=$!

make watch_backend &
BACKEND_PID=$!

# Function for graceful shutdown
cleanup() {
    echo "Shutting down gracefully..."
    kill -TERM $FRONTEND_PID $BACKEND_PID 2>/dev/null || true
    wait $FRONTEND_PID $BACKEND_PID 2>/dev/null || true
    echo "Shutdown complete"
}

# Set up trap for graceful shutdown
trap cleanup EXIT INT TERM

# Wait for background processes
wait
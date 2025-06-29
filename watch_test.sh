#!/usr/bin/env bash

echo "🧪 Test watcher for Genie"
echo "Watching for .go file changes to run tests..."
echo "Press Ctrl+C to stop"

# Global variables to track processes
FSWATCH_PID=""

# Cleanup function
cleanup() {
    echo ""
    echo "🛑 Shutting down test watcher..."
    
    # Kill fswatch if running
    if [ -n "$FSWATCH_PID" ] && kill -0 "$FSWATCH_PID" 2>/dev/null; then
        echo "🔄 Stopping file watcher (PID: $FSWATCH_PID)..."
        kill "$FSWATCH_PID" 2>/dev/null || true
    fi
    
    echo "✅ Cleanup complete"
    exit 0
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM

# Function to run tests
run_tests() {
    echo "🧪 Running tests..."
    if make test; then
        echo "✅ All tests passed"
    else
        echo "❌ Tests failed"
    fi
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# Initial test run
run_tests

# Watch for changes with debouncing
LAST_RUN=0
echo "👀 Starting file watcher..."

# Start fswatch and handle signals properly
{
    fswatch -o --exclude='build/' --exclude='\.git' --exclude='\.genie' --exclude='tmp/' --include='.*\.go$' . | while read -r f; do
        NOW=$(date +%s)
        # Only run tests if more than 2 seconds have passed since last run
        if [ $((NOW - LAST_RUN)) -gt 2 ]; then
            echo "🔄 Go file change detected, running tests..."
            run_tests
            LAST_RUN=$NOW
        else
            echo "⏳ Change detected but debouncing..."
        fi
    done
} &
FSWATCH_PID=$!

# Wait for the watcher to finish (or be interrupted)
wait "$FSWATCH_PID"
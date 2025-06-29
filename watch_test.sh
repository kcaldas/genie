#!/usr/bin/env bash

echo "🧪 Test watcher for Genie"
echo "Watching for .go file changes to run tests..."

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
fswatch -o --exclude='build/' --exclude='\.git' --exclude='\.genie' --exclude='tmp/' --include='.*\.go$' . | while read f; do
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
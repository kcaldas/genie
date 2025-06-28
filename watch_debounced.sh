#!/usr/bin/env bash

echo "🔥 Hot reload for Genie TUI (debounced)"
echo "Watching for .go file changes..."

# Kill any existing processes
pkill -f 'build/genie' 2>/dev/null || true

# Function to build and run
build_and_run() {
    echo "📦 Building..."
    if go build -o build/genie ./cmd; then
        echo "✅ Build successful"
        
        # Kill old process
        pkill -f 'build/genie' 2>/dev/null || true
        sleep 0.5
        
        # Start new process in background
        echo "🚀 Starting TUI..."
        ./build/genie --tui gocui &
        APP_PID=$!
        echo "Started with PID: $APP_PID"
    else
        echo "❌ Build failed"
    fi
}

# Initial build and run
build_and_run

# Watch for changes with debouncing
LAST_BUILD=0
fswatch -o --exclude='build/' --exclude='\.git' --exclude='\.genie'  . | while read f; do
    NOW=$(date +%s)
    # Only build if more than 2 seconds have passed since last build
    if [ $((NOW - LAST_BUILD)) -gt 2 ]; then
        echo "🔄 Change detected, rebuilding..."
        build_and_run
        LAST_BUILD=$NOW
    else
        echo "⏳ Change detected but debouncing..."
    fi
done

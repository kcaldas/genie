#!/usr/bin/env bash

echo "🔥 Hot reload for Genie TUI"
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

# Watch for changes (requires fswatch: brew install fswatch)
fswatch -o --exclude='build/' --exclude='.git/' . | while read f; do
    echo "🔄 Change detected, rebuilding..."
    build_and_run
done

#!/usr/bin/env bash

echo "🔥 Hot reload for Genie TUI"

# Kill any existing processes
pkill -f 'build/genie' 2>/dev/null || true

# Function to build and run
build_and_run() {
    echo "📦 Building..."
    if go build -o build/genie ./cmd; then
        echo "✅ Build successful"
        pkill -f 'build/genie' 2>/dev/null || true
        sleep 0.2
        echo "🚀 Starting TUI..."
        ./build/genie --tui gocui &
    else
        echo "❌ Build failed"
    fi
}

# Initial build
build_and_run

# Watch only .go files with 1-second delay between events
echo "👀 Watching .go files..."
fswatch -1 -l 1 --include='.*\.go$' --exclude='build/' . | while read f; do
    echo "🔄 $f changed"
    build_and_run
done

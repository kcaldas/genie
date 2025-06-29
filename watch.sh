#!/usr/bin/env bash

echo "🔥 Hot reload for Genie TUI (debounced)"
echo "Watching for .go file changes..."
echo "Press Ctrl+C to stop"

# Global variables to track processes
APP_PID=""
FSWATCH_PID=""

# Cleanup function
cleanup() {
    echo ""
    echo "🛑 Shutting down watcher..."
    
    # Kill the genie app if running
    if [ -n "$APP_PID" ] && kill -0 "$APP_PID" 2>/dev/null; then
        echo "🔄 Stopping Genie app (PID: $APP_PID)..."
        kill "$APP_PID" 2>/dev/null || true
        wait "$APP_PID" 2>/dev/null || true
    fi
    
    # Kill any remaining genie processes
    pkill -f 'build/genie' 2>/dev/null || true
    
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

# Kill any existing processes
pkill -f 'build/genie' 2>/dev/null || true

# Function to build and run
build_and_run() {
    echo "📦 Building..."
    if go build -o build/genie ./cmd; then
        echo "✅ Build successful"
        
        # Kill old process if running
        if [ -n "$APP_PID" ] && kill -0 "$APP_PID" 2>/dev/null; then
            echo "🔄 Stopping previous instance..."
            kill "$APP_PID" 2>/dev/null || true
            wait "$APP_PID" 2>/dev/null || true
        fi
        
        # Kill any remaining genie processes
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
echo "👀 Starting file watcher..."

# Start fswatch and handle signals properly
{
    fswatch -o --exclude='build/' --exclude='\.git' --exclude='\.genie' . | while read -r f; do
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
} &
FSWATCH_PID=$!

# Wait for the watcher to finish (or be interrupted)
wait "$FSWATCH_PID"

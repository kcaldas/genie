#!/bin/bash

# Docker run script for Genie
# This script makes it easy to run Genie in a Docker container with proper volume mounts

set -e

# Default image
IMAGE="ghcr.io/kcaldas/genie:latest"

# Help function
show_help() {
    cat << EOF
Usage: ./docker-run.sh [OPTIONS] [GENIE_ARGS...]

Run Genie in a Docker container with proper volume mounts.

OPTIONS:
    -h, --help          Show this help message
    -i, --image IMAGE   Use specific Docker image (default: $IMAGE)
    -v, --version       Show Genie version
    --build-local       Build and use local Docker image

EXAMPLES:
    ./docker-run.sh                    # Run interactive TUI
    ./docker-run.sh ask "hello"        # Run CLI command
    ./docker-run.sh --build-local      # Build local image first
    ./docker-run.sh -i my-genie:dev    # Use custom image

VOLUME MOUNTS:
    - Current directory mounted to /workspace (read-only)
    - ~/.genie mounted to /home/genie/.genie (persistent config)

EOF
}

# Parse options
BUILD_LOCAL=false
GENIE_ARGS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -i|--image)
            IMAGE="$2"
            shift 2
            ;;
        -v|--version)
            GENIE_ARGS+=("--version")
            shift
            ;;
        --build-local)
            BUILD_LOCAL=true
            shift
            ;;
        *)
            GENIE_ARGS+=("$1")
            shift
            ;;
    esac
done

# Build local image if requested
if [[ "$BUILD_LOCAL" == "true" ]]; then
    echo "Building local Genie Docker image..."
    docker build -f Dockerfile.local -t genie:local .
    IMAGE="genie:local"
fi

# Create .genie directory if it doesn't exist
mkdir -p ~/.genie

# Determine if we need TTY (for interactive mode)
TTY_FLAG=""
if [[ ${#GENIE_ARGS[@]} -eq 0 ]] || [[ "${GENIE_ARGS[0]}" != "ask" ]]; then
    TTY_FLAG="-it"
fi

# Run Genie in Docker
echo "Running Genie in Docker container..."
echo "Image: $IMAGE"
echo "Arguments: ${GENIE_ARGS[*]}"
echo "Working directory: $(pwd)"
echo ""

docker run --rm $TTY_FLAG \
    -v "$(pwd):/workspace:ro" \
    -v "$HOME/.genie:/home/genie/.genie" \
    -w /workspace \
    -e "GEMINI_API_KEY=${GEMINI_API_KEY:-}" \
    -e "OPENAI_API_KEY=${OPENAI_API_KEY:-}" \
    -e "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}" \
    -e "GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS:-}" \
    "$IMAGE" "${GENIE_ARGS[@]}"
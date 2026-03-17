#!/bin/bash
set -e

VERSION="${1:-dev}"
MODULE="optimus/framework/cli/cmd"
ENTRY="./framework/cli"
OUTPUT="build"

mkdir -p "$OUTPUT"

echo "Building Optimus v${VERSION}..."

# Build for current platform
go build -ldflags "-X ${MODULE}.Version=${VERSION}" -o "${OUTPUT}/optimus" "$ENTRY"

echo "Built: ${OUTPUT}/optimus"
echo ""

# Cross-compile if --all flag is passed
if [ "$2" == "--all" ]; then
    PLATFORMS=(
        "darwin/amd64"
        "darwin/arm64"
        "linux/amd64"
        "linux/arm64"
        "windows/amd64"
    )

    for PLATFORM in "${PLATFORMS[@]}"; do
        GOOS="${PLATFORM%/*}"
        GOARCH="${PLATFORM#*/}"
        BINARY="${OUTPUT}/optimus-${GOOS}-${GOARCH}"
        if [ "$GOOS" == "windows" ]; then
            BINARY="${BINARY}.exe"
        fi

        echo "Building ${GOOS}/${GOARCH}..."
        GOOS=$GOOS GOARCH=$GOARCH go build \
            -ldflags "-X ${MODULE}.Version=${VERSION}" \
            -o "$BINARY" "$ENTRY"
    done

    echo ""
    echo "All platforms built in ${OUTPUT}/"
fi

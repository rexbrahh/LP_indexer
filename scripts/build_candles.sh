#!/bin/bash
set -e

# Build script for candle C++ module
# Usage: ./scripts/build_candles.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CANDLE_DIR="$PROJECT_ROOT/state/candle_cpp"
BUILD_DIR="$CANDLE_DIR/build"

echo "=================================================="
echo "Building Candle C++ Module"
echo "=================================================="

# Create build directory
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

# Configure with CMake
echo ""
echo "Configuring CMake..."
cmake ..

# Build
echo ""
echo "Building..."
cmake --build .

echo ""
echo "=================================================="
echo "Build complete!"
echo "=================================================="
echo ""
echo "Test binaries:"
echo "  - $BUILD_DIR/tests/fixed_point_test"
echo "  - $BUILD_DIR/tests/candle_finalize_test"
echo ""
echo "Run tests with:"
echo "  cd $BUILD_DIR && ctest --output-on-failure"
echo ""

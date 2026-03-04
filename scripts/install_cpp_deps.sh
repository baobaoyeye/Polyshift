#!/bin/bash
set -e

# Define dependencies directory
PROJECT_ROOT=$(pwd)
DEPS_DIR="$PROJECT_ROOT/sdk/cpp/deps"
mkdir -p "$DEPS_DIR"

echo "Installing C++ SDK dependencies to $DEPS_DIR..."

# Check for cmake
if ! command -v cmake &> /dev/null; then
    echo "cmake could not be found, please install it."
    exit 1
fi

# Install opentelemetry-cpp
if [ ! -d "opentelemetry-cpp" ]; then
    echo "Cloning opentelemetry-cpp..."
    git clone --depth 1 --branch v1.14.2 https://github.com/open-telemetry/opentelemetry-cpp.git
fi

cd opentelemetry-cpp
mkdir -p build
cd build

echo "Building opentelemetry-cpp..."
cmake .. \
    -DCMAKE_CXX_STANDARD=17 \
    -DWITH_OTLP_GRPC=ON \
    -DWITH_ABSEIL=ON \
    -DBUILD_TESTING=OFF \
    -DBUILD_EXAMPLES=OFF \
    -DCMAKE_INSTALL_PREFIX="$DEPS_DIR" \
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON

make -j$(nproc 2>/dev/null || sysctl -n hw.ncpu)
make install

echo "----------------------------------------------------------------"
echo "C++ dependencies installed successfully to $DEPS_DIR"
echo "To use them, configure your build with:"
echo "cmake -DCMAKE_PREFIX_PATH=$DEPS_DIR .."
echo "----------------------------------------------------------------"

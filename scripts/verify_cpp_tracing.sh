#!/bin/bash

# Build C++ SDK
echo "Building C++ SDK..."
cd sdk/cpp || exit 1
mkdir -p build
cd build || exit 1
if [ -d "$(pwd)/../deps" ]; then
    cmake -DCMAKE_PREFIX_PATH="$(pwd)/../deps" ..
else
    cmake ..
fi
make -j4
if [ $? -ne 0 ]; then
    echo "Build failed."
    exit 1
fi

# Run Unit Tests
echo "Running C++ Unit Tests..."
ctest --output-on-failure
if [ $? -ne 0 ]; then
    echo "Unit tests failed."
    exit 1
fi

# Run plugin
export OTEL_TRACES_EXPORTER=console
export OTEL_SERVICE_NAME=cpp-plugin
echo "Starting C++ plugin..."
./polyshift-cpp-plugin > plugin_cpp.log 2>&1 &
PID=$!

# Wait for plugin to start
sleep 2

# Extract port
PORT=$(grep "PLUGIN_ADDR" plugin_cpp.log | awk -F'[:|]' '{print $4}')
if [ -z "$PORT" ]; then
    echo "Failed to get plugin port."
    cat plugin_cpp.log
    kill $PID
    exit 1
fi

echo "Plugin listening on port $PORT"

# Send request
echo "Sending request..."
cd ../../../ || exit 1
python3 scripts/send_grpc_request.py $PORT

# Check logs for traces
echo "Checking logs for traces..."
# Allow flush
sleep 5
if grep -iE "trace_id|traceId|span|Trace ID" sdk/cpp/build/plugin_cpp.log; then
    echo "SUCCESS: Trace info found in logs."
    grep -iE "trace_id|traceId|span|Trace ID" sdk/cpp/build/plugin_cpp.log | head -n 5
else
    echo "FAILURE: No trace info found in logs."
    cat sdk/cpp/build/plugin_cpp.log
    kill $PID
    exit 1
fi

kill $PID
echo "Done."

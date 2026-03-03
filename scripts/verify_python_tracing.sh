#!/bin/bash
set -e

# Ensure we are in project root
cd "$(dirname "$0")/.."

echo "Starting Python Tracing Verification..."

# Install dependencies if possible (ignoring errors if pip fails in restricted env)
pip install -r sdk/python/requirements.txt || echo "Warning: pip install failed, assuming deps exist"

# Start Plugin in background
export OTEL_TRACES_EXPORTER=console
export OTEL_SERVICE_NAME=python-hello-service
export PYTHONPATH=$(pwd)/sdk/python

# Clean previous log
rm -f plugin.log

# Cleanup trap
cleanup() {
    echo "Stopping plugin..."
    if [ ! -z "$PID" ]; then
        kill $PID || true
    fi
}
trap cleanup EXIT

echo "Launching plugin..."
python3 examples/py-hello/main.py > plugin.log 2>&1 &
PID=$!

echo "Plugin started with PID $PID"

# Wait for port
echo "Waiting for plugin to initialize..."
for i in {1..10}; do
    if grep -q "|PLUGIN_ADDR|" plugin.log; then
        break
    fi
    sleep 1
done

PORT=$(grep "|PLUGIN_ADDR|" plugin.log | tail -n 1 | awk -F'|' '{print $3}' | awk -F':' '{print $2}')

if [ -z "$PORT" ]; then
    echo "Failed to get port from logs:"
    cat plugin.log
    exit 1
fi

echo "Plugin listening on port $PORT"

# Send request
echo "Sending HealthCheck request..."
python3 scripts/send_grpc_request.py $PORT

# Give some time for logs to flush
sleep 2

# Verify Trace ID in logs
echo "Checking logs for traces..."
if grep -q "trace_id" plugin.log || grep -q "Span" plugin.log; then
    echo "SUCCESS: Trace ID found in logs."
    echo "Sample log output:"
    grep -A 5 "Span" plugin.log | head -n 20
else
    echo "FAILURE: No Trace ID found in logs."
    echo "Full Log:"
    cat plugin.log
    exit 1
fi

#!/bin/bash
set -e

# Ensure we are in project root
cd "$(dirname "$0")/.."

echo "Starting Java Tracing Verification..."

# Install SDK to local maven repo (skip tests for speed/reliability of this script)
echo "Installing Java SDK..."
mvn clean install -f sdk/java/pom.xml -DskipTests

# Build Example Plugin
echo "Building Java Example Plugin..."
mvn clean package -f examples/java-hello/pom.xml

# Start Plugin in background
export OTEL_TRACES_EXPORTER=console
export OTEL_SERVICE_NAME=java-hello-service

# Clean previous log
rm -f plugin_java.log

echo "Launching plugin..."
java -jar examples/java-hello/target/java-hello-1.0.0.jar > plugin_java.log 2>&1 &
PID=$!

echo "Plugin started with PID $PID"

# Cleanup trap
cleanup() {
    echo "Stopping plugin..."
    kill $PID || true
}
trap cleanup EXIT

# Wait for port
echo "Waiting for plugin to initialize..."
for i in {1..20}; do
    if grep -q "|PLUGIN_ADDR|" plugin_java.log; then
        break
    fi
    sleep 1
done

PORT=$(grep "|PLUGIN_ADDR|" plugin_java.log | tail -n 1 | awk -F'|' '{print $3}' | awk -F':' '{print $2}')

if [ -z "$PORT" ]; then
    echo "Failed to get port from logs:"
    cat plugin_java.log
    exit 1
fi

echo "Plugin listening on port $PORT"

# Send request
echo "Sending HealthCheck request..."
# Reuse the python script if available, or use grpcurl if installed, 
# or just rely on the fact that we have python env setup from previous step
if [ -f "scripts/send_grpc_request.py" ]; then
    python3 scripts/send_grpc_request.py $PORT
else
    echo "Error: scripts/send_grpc_request.py not found"
    exit 1
fi

# Give some time for logs to flush
sleep 2

# Verify Trace ID in logs
echo "Checking logs for traces..."
if grep -q "trace_id" plugin_java.log || grep -q "Span" plugin_java.log || grep -q "OTEL" plugin_java.log; then
    echo "SUCCESS: Trace info found in logs."
    echo "Sample log output:"
    grep -A 5 "Span" plugin_java.log | head -n 20 || grep "trace_id" plugin_java.log | head -n 20
else
    echo "FAILURE: No Trace info found in logs."
    echo "Full Log:"
    cat plugin_java.log
    exit 1
fi

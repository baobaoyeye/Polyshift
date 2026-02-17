#!/bin/bash
set -e

CONFIG="test/e2e/config_chaos.yaml"
CORE_BIN="bin/polyshift-core"
PORT=8081

# Cleanup function
cleanup() {
  echo "Cleaning up..."
  kill $CORE_PID 2>/dev/null || true
  pkill -f "flaky_plugin" 2>/dev/null || true
}
trap cleanup EXIT

# Start Core
echo "Starting Core..."
$CORE_BIN -config $CONFIG &
CORE_PID=$!
sleep 3

# 1. Verify Health
echo "Verifying health..."
curl -f http://localhost:$PORT/health
echo "Health OK"

# 2. Verify Plugin
echo "Verifying plugin..."
curl -f http://localhost:$PORT/api/chaos
echo "Plugin OK"

# 3. Kill Plugin
echo "Killing plugin process..."
# Find PID of the plugin started by this core
# We assume only one instance of flaky_plugin is running
PLUGIN_PID=$(pgrep -f "flaky_plugin" | head -n 1)
if [ -z "$PLUGIN_PID" ]; then
  echo "Plugin PID not found"
  exit 1
fi
echo "Killing PID $PLUGIN_PID"
kill -9 $PLUGIN_PID

# 4. Verify Failure (Immediate)
echo "Verifying failure..."
# Expect 503 or 500
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/api/chaos)
if [ "$HTTP_CODE" == "200" ]; then
  echo "Error: Plugin still responding after kill (Code: $HTTP_CODE)"
  exit 1
else
  echo "Plugin failed as expected (Code: $HTTP_CODE)"
fi

# 5. Wait for Watchdog Restart (Interval 2s + Backoff)
echo "Waiting for Watchdog restart (5s)..."
sleep 5

# 6. Verify Recovery
echo "Verifying recovery..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/api/chaos)
if [ "$HTTP_CODE" == "200" ]; then
  echo "Plugin recovered successfully!"
else
  echo "Error: Plugin failed to recover (Code: $HTTP_CODE)"
  exit 1
fi

echo "Test Passed!"

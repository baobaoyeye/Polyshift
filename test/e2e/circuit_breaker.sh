#!/bin/bash
set -e

CONFIG="test/e2e/config_cb.yaml"
CORE_BIN="bin/polyshift-core"
PORT=8082

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

# 2. Verify Plugin Success
echo "Verifying plugin success..."
curl -f http://localhost:$PORT/api/cb
echo "Plugin OK"

# 3. Trigger Failures (Circuit Breaker Config: 3 requests to trip)
echo "Triggering failures..."
# Send 3 requests with X-Fail-Once: true
for i in {1..3}; do
  echo "Failure request $i"
  # We expect 500
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Fail-Once: true" http://localhost:$PORT/api/cb)
  if [ "$HTTP_CODE" != "500" ]; then
    echo "Expected 500, got $HTTP_CODE"
    exit 1
  fi
done

# 4. Verify Circuit Breaker Open (Fail Fast)
echo "Verifying Circuit Breaker Open..."
# Now send a NORMAL request. If CB is open, it should fail immediately (Core returns error).
# If CB is closed, it should SUCCEED (because we don't send X-Fail-Once).

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/api/cb)

if [ "$HTTP_CODE" == "500" ]; then
  echo "Circuit Breaker is OPEN (blocked request)"
elif [ "$HTTP_CODE" == "200" ]; then
  echo "Error: Circuit Breaker failed to open (request succeeded)"
  exit 1
else
  echo "Unexpected code: $HTTP_CODE"
  exit 1
fi

# 5. Wait for Half-Open (Timeout 5s)
echo "Waiting for Half-Open (6s)..."
sleep 6

# 6. Verify Recovery
echo "Verifying recovery..."
# Next request allows through (Half-Open). Since it's normal request, it succeeds.
# Then CB closes.

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/api/cb)
if [ "$HTTP_CODE" == "200" ]; then
  echo "Circuit Breaker recovered (Half-Open -> Closed)"
else
  echo "Error: Circuit Breaker failed to recover (Code: $HTTP_CODE)"
  exit 1
fi

echo "Test Passed!"

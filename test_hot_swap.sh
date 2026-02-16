#!/bin/bash

BASE_URL="http://localhost:8080"

echo "=== Testing Plugin Endpoints ==="
echo "1. Go Plugin:"
curl -s "$BASE_URL/api/hello"
echo -e "\n"

echo "2. Python Plugin:"
curl -s "$BASE_URL/api/python"
echo -e "\n"

echo "3. Node Plugin:"
curl -s "$BASE_URL/api/node"
echo -e "\n"

echo "4. Java Plugin:"
curl -s "$BASE_URL/api/java"
echo -e "\n"

echo "=== Testing Hot Swapping ==="
echo "5. Reloading go-hello..."
curl -s -X PUT "$BASE_URL/admin/plugins/go-hello/reload"
echo -e "\n"
sleep 1
echo "Verifying Go Plugin after reload:"
curl -s "$BASE_URL/api/hello"
echo -e "\n"

echo "6. Stopping py-hello..."
curl -s -X DELETE "$BASE_URL/admin/plugins/py-hello"
echo -e "\n"
sleep 1
echo "Verifying Python Plugin is gone (should be 503):"
curl -v "$BASE_URL/api/python"
echo -e "\n"

echo "7. Starting py-hello again..."
curl -s -X POST "$BASE_URL/admin/plugins" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "py-hello",
    "version": "1.0.0",
    "runtime": "python",
    "entrypoint": "examples/py-hello/main.py",
    "params": {
      "greeting": "Hola Reloaded"
    },
    "routes": [
      {
        "path": "/api/python",
        "method": "GET",
        "handler": "python"
      }
    ]
  }'
echo -e "\n"
sleep 2
echo "Verifying Python Plugin is back with new config:"
curl -s "$BASE_URL/api/python"
echo -e "\n"

echo "8. Adding a NEW dynamic plugin (py-hello-dynamic)..."
curl -s -X POST "$BASE_URL/admin/plugins" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "py-hello-dynamic",
    "version": "1.0.0",
    "runtime": "python",
    "entrypoint": "examples/py-hello/main.py",
    "params": {
      "greeting": "Dynamic Hola"
    },
    "routes": [
      {
        "path": "/api/python-dynamic",
        "method": "GET",
        "handler": "python"
      }
    ]
  }'
echo -e "\n"
sleep 2
echo "Verifying dynamic route /api/python-dynamic:"
curl -s "$BASE_URL/api/python-dynamic"
echo -e "\n"

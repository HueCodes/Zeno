#!/bin/bash
set -e

echo "Starting integration test suite..."

# Start services
echo "Starting Docker Compose stack..."
docker-compose -f docker/docker-compose.yml up -d

# Wait for controller to be healthy
echo "Waiting for controller to be healthy..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
  if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo "Controller is healthy!"
    break
  fi
  attempt=$((attempt + 1))
  echo "Attempt $attempt/$max_attempts..."
  sleep 2
done

if [ $attempt -eq $max_attempts ]; then
  echo "Controller failed to become healthy"
  docker-compose -f docker/docker-compose.yml logs controller
  docker-compose -f docker/docker-compose.yml down
  exit 1
fi

# Run API tests
echo "Running API tests..."

# Test health endpoint
echo "Testing /health..."
response=$(curl -sf http://localhost:8080/health)
if echo "$response" | grep -q "healthy"; then
  echo "✓ /health passed"
else
  echo "✗ /health failed"
  exit 1
fi

# Test metrics endpoint
echo "Testing /api/v1/metrics..."
response=$(curl -sf http://localhost:8080/api/v1/metrics)
if [ -n "$response" ]; then
  echo "✓ /api/v1/metrics passed"
else
  echo "✗ /api/v1/metrics failed"
  exit 1
fi

# Test runners endpoint
echo "Testing /api/v1/runners..."
response=$(curl -sf http://localhost:8080/api/v1/runners)
if echo "$response" | grep -q "count"; then
  echo "✓ /api/v1/runners passed"
else
  echo "✗ /api/v1/runners failed"
  exit 1
fi

# Test history endpoint
echo "Testing /api/v1/history..."
response=$(curl -sf http://localhost:8080/api/v1/history)
if [ -n "$response" ]; then
  echo "✓ /api/v1/history passed"
else
  echo "✗ /api/v1/history failed"
  exit 1
fi

echo ""
echo "All integration tests passed! ✓"

# Cleanup
echo "Cleaning up..."
docker-compose -f docker/docker-compose.yml down

exit 0

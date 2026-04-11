#!/bin/bash
# CampusRec Test Runner
# NOTE: Do NOT use `set -e` here. Test commands may return non-zero exit codes
# which must be captured into variables for the final summary.

echo "=== CampusRec Test Runner ==="
echo ""

# 1. Cleanup previous environment
echo "--- Cleaning up previous environment ---"
docker compose --profile test down --remove-orphans -v 2>/dev/null || true

# 2. Build all services (fail fast if build fails)
echo ""
echo "--- Building services ---"
if ! docker compose --profile test build; then
    echo "FAIL: docker compose build failed"
    exit 1
fi

# 3. Start infrastructure services
echo ""
echo "--- Starting database ---"
docker compose up -d db
echo "Waiting for database to be ready..."
RETRIES=30
until docker compose exec -T db pg_isready -U campusrec -d campusrec 2>/dev/null; do
    RETRIES=$((RETRIES - 1))
    if [ $RETRIES -le 0 ]; then
        echo "FAIL: Database did not become ready"
        docker compose --profile test down --remove-orphans -v 2>/dev/null || true
        exit 1
    fi
    sleep 2
done
echo "Database is ready."

# 4. Start backend and worker (raise rate limit for test traffic)
echo ""
echo "--- Starting backend and worker ---"
export RATE_LIMIT_PER_MINUTE=10000
export COOKIE_SECURE=false
docker compose up -d backend worker
echo "Waiting for backend to be ready..."
RETRIES=30
until docker compose exec -T backend wget -q -O /dev/null http://localhost:8080/api/health 2>/dev/null; do
    RETRIES=$((RETRIES - 1))
    if [ $RETRIES -le 0 ]; then
        echo "FAIL: Backend did not become ready"
        echo "Backend logs:"
        docker compose logs backend --tail=30
        docker compose --profile test down --remove-orphans -v 2>/dev/null || true
        exit 1
    fi
    sleep 2
done
echo "Backend is ready."

# 5. Run unit tests
echo ""
echo "=== Running Unit Tests ==="
docker compose run --rm --no-deps -T test-runner go test ./unit_tests/... -v -count=1 2>&1
UNIT_EXIT=$?

# 6. Run API tests
echo ""
echo "=== Running API Tests ==="
docker compose run --rm -T test-runner go test ./API_tests/... -v -count=1 -tags=integration 2>&1
API_EXIT=$?

# 7. Summary
echo ""
echo "==============================="
echo "=== Test Results ==="
echo "==============================="
if [ $UNIT_EXIT -eq 0 ]; then
    echo "Unit Tests:  PASS"
else
    echo "Unit Tests:  FAIL"
fi
if [ $API_EXIT -eq 0 ]; then
    echo "API Tests:   PASS"
else
    echo "API Tests:   FAIL"
fi
echo "==============================="

# 8. Cleanup
echo ""
echo "--- Cleaning up ---"
docker compose --profile test down --remove-orphans -v 2>/dev/null || true

# 9. Exit
if [ $UNIT_EXIT -ne 0 ] || [ $API_EXIT -ne 0 ]; then
    echo ""
    echo "FAIL"
    exit 1
fi
echo ""
echo "PASS"
exit 0

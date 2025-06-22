#!/bin/bash

# run_all_tests.sh - Comprehensive test runner for MCP Go SDK
# This script runs all test types and generates a comprehensive report

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-$PROJECT_ROOT/test-results}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$OUTPUT_DIR/test-report-$TIMESTAMP.json"
COVERAGE_FILE="$OUTPUT_DIR/coverage-$TIMESTAMP.out"
BENCHMARK_FILE="$OUTPUT_DIR/benchmarks-$TIMESTAMP.txt"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
UNIT_TEST_TIMEOUT="${UNIT_TEST_TIMEOUT:-10m}"
E2E_TEST_TIMEOUT="${E2E_TEST_TIMEOUT:-20m}"
INTEGRATION_TEST_TIMEOUT="${INTEGRATION_TEST_TIMEOUT:-15m}"
BENCHMARK_TIME="${BENCHMARK_TIME:-10s}"
BENCHMARK_COUNT="${BENCHMARK_COUNT:-5}"
HARNESS_CONFIG="${HARNESS_CONFIG:-$SCRIPT_DIR/test-harness-config.json}"

# Initialize output directory
mkdir -p "$OUTPUT_DIR"

# Initialize report structure
REPORT='{
  "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
  "project": "mcp-go-sdk",
  "test_results": {},
  "coverage": {},
  "benchmarks": {},
  "harness": {},
  "summary": {}
}'

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Function to update JSON report
update_report() {
    local key=$1
    local value=$2
    REPORT=$(echo "$REPORT" | jq --arg k "$key" --argjson v "$value" '.[$k] = $v')
}

# Function to run tests and capture results
run_test_suite() {
    local suite_name=$1
    local test_command=$2
    local timeout=$3
    local output_file="$OUTPUT_DIR/${suite_name}-output-$TIMESTAMP.txt"
    local json_output_file="$OUTPUT_DIR/${suite_name}-json-$TIMESTAMP.json"
    
    log_info "Running $suite_name tests..."
    
    local start_time=$(date +%s)
    local exit_code=0
    
    # Run tests with timeout and capture output
    # Use gtimeout on macOS if available, otherwise run without timeout
    if command -v gtimeout &> /dev/null; then
        TIMEOUT_CMD="gtimeout"
    elif command -v timeout &> /dev/null; then
        TIMEOUT_CMD="timeout"
    else
        TIMEOUT_CMD=""
    fi
    
    if [ -n "$TIMEOUT_CMD" ]; then
        if $TIMEOUT_CMD "$timeout" bash -c "$test_command" > "$output_file" 2>&1; then
            exit_code=0
            log_success "$suite_name tests passed"
        else
            exit_code=$?
            log_error "$suite_name tests failed with exit code $exit_code"
        fi
    else
        # Run without timeout
        if bash -c "$test_command" > "$output_file" 2>&1; then
            exit_code=0
            log_success "$suite_name tests passed"
        else
            exit_code=$?
            log_error "$suite_name tests failed with exit code $exit_code"
        fi
    fi
    
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    # Parse test results
    local total_tests=0
    local passed_tests=0
    local failed_tests=0
    local skipped_tests=0
    
    # Try to extract test counts from output
    if grep -q "PASS" "$output_file"; then
        passed_tests=$(grep -c "PASS:" "$output_file" 2>/dev/null || echo 0)
    fi
    if grep -q "FAIL" "$output_file"; then
        failed_tests=$(grep -c "FAIL:" "$output_file" 2>/dev/null || echo 0)
    fi
    if grep -q "SKIP" "$output_file"; then
        skipped_tests=$(grep -c "SKIP:" "$output_file" 2>/dev/null || echo 0)
    fi
    
    total_tests=$((passed_tests + failed_tests + skipped_tests))
    
    # Create suite result JSON
    local suite_result=$(cat <<EOF
{
    "suite": "$suite_name",
    "status": $([ $exit_code -eq 0 ] && echo '"passed"' || echo '"failed"'),
    "exit_code": $exit_code,
    "duration_seconds": $duration,
    "total_tests": $total_tests,
    "passed": $passed_tests,
    "failed": $failed_tests,
    "skipped": $skipped_tests,
    "output_file": "$output_file",
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
)
    
    # Update main report
    REPORT=$(echo "$REPORT" | jq --arg suite "$suite_name" --argjson result "$suite_result" '.test_results[$suite] = $result')
    
    return $exit_code
}

# Main test execution
log_info "Starting comprehensive test suite at $(date)"
log_info "Project root: $PROJECT_ROOT"
log_info "Output directory: $OUTPUT_DIR"

cd "$PROJECT_ROOT"

# Track overall status
OVERALL_STATUS=0

# 1. Run Unit Tests with Coverage
log_info "="
log_info "Phase 1: Unit Tests with Coverage"
log_info "="

if run_test_suite "unit" "go test -v -race -coverprofile=$COVERAGE_FILE -covermode=atomic ./mcp/... -timeout $UNIT_TEST_TIMEOUT" "$UNIT_TEST_TIMEOUT"; then
    # Generate coverage report
    go tool cover -html="$COVERAGE_FILE" -o "$OUTPUT_DIR/coverage-$TIMESTAMP.html" 2>/dev/null || true
    
    # Extract coverage percentage
    COVERAGE_PERCENT=$(go tool cover -func="$COVERAGE_FILE" | grep total | awk '{print $3}' | sed 's/%//')
    
    REPORT=$(echo "$REPORT" | jq --arg cov "$COVERAGE_PERCENT" '.coverage = {
        "percentage": ($cov | tonumber),
        "report_file": "'$COVERAGE_FILE'",
        "html_file": "'$OUTPUT_DIR'/coverage-'$TIMESTAMP'.html"
    }')
else
    OVERALL_STATUS=1
fi

# 2. Run E2E Tests
log_info "="
log_info "Phase 2: End-to-End Tests"
log_info "="

if ! run_test_suite "e2e" "go test -v ./e2e/... -timeout $E2E_TEST_TIMEOUT" "$E2E_TEST_TIMEOUT"; then
    OVERALL_STATUS=1
fi

# 3. Run Integration Tests
log_info "="
log_info "Phase 3: Integration Tests"
log_info "="

if ! run_test_suite "integration" "go test -v -tags=integration ./mcp/... -timeout $INTEGRATION_TEST_TIMEOUT" "$INTEGRATION_TEST_TIMEOUT"; then
    OVERALL_STATUS=1
fi

# 4. Run Benchmark Tests
log_info "="
log_info "Phase 4: Benchmark Tests"
log_info "="

BENCHMARK_CMD="go test -bench=. -benchtime=$BENCHMARK_TIME -count=$BENCHMARK_COUNT -run=^$ ./mcp/... > $BENCHMARK_FILE 2>&1"

if run_test_suite "benchmark" "$BENCHMARK_CMD" "30m"; then
    # Parse benchmark results
    if [ -f "$BENCHMARK_FILE" ]; then
        # Extract benchmark data (simplified - you might want more sophisticated parsing)
        BENCHMARKS=$(grep -E "^Benchmark" "$BENCHMARK_FILE" | head -20 || true)
        
        REPORT=$(echo "$REPORT" | jq --arg bench "$BENCHMARKS" '.benchmarks = {
            "results": ($bench | split("\n")),
            "output_file": "'$BENCHMARK_FILE'"
        }')
    fi
else
    OVERALL_STATUS=1
fi

# 5. Run Test Harness
log_info "="
log_info "Phase 5: Test Harness (Compliance & Performance)"
log_info "="

# Generate test harness config if it doesn't exist
if [ ! -f "$HARNESS_CONFIG" ]; then
    cat > "$HARNESS_CONFIG" <<EOF
{
    "server_command": ["go", "run", "./examples/simple-server/main.go"],
    "server_timeout": 10000000000,
    "test_timeout": 30000000000,
    "warmup_duration": 2000000000,
    "strict_compliance": true,
    "required_methods": ["resources/list", "tools/list"],
    "optional_methods": ["prompts/list"],
    "output_format": "json",
    "output_file": "$OUTPUT_DIR/harness-results-$TIMESTAMP.json",
    "continue_on_failure": true
}
EOF
fi

# Build test harness if needed
if [ ! -f "$PROJECT_ROOT/testharness/cmd/mcp-test/mcp-test" ]; then
    log_info "Building test harness..."
    (cd "$PROJECT_ROOT/testharness/cmd/mcp-test" && go build -o mcp-test) || true
fi

HARNESS_OUTPUT="$OUTPUT_DIR/harness-output-$TIMESTAMP.txt"
HARNESS_RESULTS="$OUTPUT_DIR/harness-results-$TIMESTAMP.json"

if [ -f "$PROJECT_ROOT/testharness/cmd/mcp-test/mcp-test" ]; then
    if "$PROJECT_ROOT/testharness/cmd/mcp-test/mcp-test" -config "$HARNESS_CONFIG" run > "$HARNESS_OUTPUT" 2>&1; then
        log_success "Test harness completed successfully"
        
        # Include harness results in main report
        if [ -f "$HARNESS_RESULTS" ]; then
            HARNESS_DATA=$(cat "$HARNESS_RESULTS")
            REPORT=$(echo "$REPORT" | jq --argjson harness "$HARNESS_DATA" '.harness = $harness')
        fi
    else
        log_error "Test harness failed"
        OVERALL_STATUS=1
    fi
else
    log_warning "Test harness not built, skipping..."
fi

# 6. Generate Summary
log_info "="
log_info "Generating Test Summary"
log_info "="

# Calculate totals
TOTAL_SUITES=$(echo "$REPORT" | jq '.test_results | length')
PASSED_SUITES=$(echo "$REPORT" | jq '[.test_results[] | select(.status == "passed")] | length')
FAILED_SUITES=$(echo "$REPORT" | jq '[.test_results[] | select(.status == "failed")] | length')
TOTAL_DURATION=$(echo "$REPORT" | jq '[.test_results[].duration_seconds] | add')
TOTAL_TESTS=$(echo "$REPORT" | jq '[.test_results[].total_tests] | add')
TOTAL_PASSED=$(echo "$REPORT" | jq '[.test_results[].passed] | add')
TOTAL_FAILED=$(echo "$REPORT" | jq '[.test_results[].failed] | add')

# Update summary
REPORT=$(echo "$REPORT" | jq --arg status $([ $OVERALL_STATUS -eq 0 ] && echo "passed" || echo "failed") '.summary = {
    "overall_status": $status,
    "exit_code": '$OVERALL_STATUS',
    "total_suites": '$TOTAL_SUITES',
    "passed_suites": '$PASSED_SUITES',
    "failed_suites": '$FAILED_SUITES',
    "total_duration_seconds": '$TOTAL_DURATION',
    "total_tests": '$TOTAL_TESTS',
    "total_passed": '$TOTAL_PASSED',
    "total_failed": '$TOTAL_FAILED',
    "coverage_percentage": (.coverage.percentage // 0),
    "report_generated_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
}')

# 7. Save Final Report
echo "$REPORT" | jq '.' > "$REPORT_FILE"

# 8. Generate Human-Readable Summary
log_info "="
log_info "Test Execution Summary"
log_info "="

echo
echo "Overall Status: $([ $OVERALL_STATUS -eq 0 ] && echo -e "${GREEN}PASSED${NC}" || echo -e "${RED}FAILED${NC}")"
echo "Total Test Suites: $TOTAL_SUITES (Passed: $PASSED_SUITES, Failed: $FAILED_SUITES)"
echo "Total Tests Run: $TOTAL_TESTS (Passed: $TOTAL_PASSED, Failed: $TOTAL_FAILED)"
echo "Total Duration: ${TOTAL_DURATION}s"
echo "Code Coverage: ${COVERAGE_PERCENT:-N/A}%"
echo
echo "Detailed Results:"
echo "$REPORT" | jq -r '.test_results[] | "  - \(.suite): \(.status) (\(.duration_seconds)s, \(.total_tests) tests)"'
echo
echo "Output Files:"
echo "  - Full Report: $REPORT_FILE"
echo "  - Coverage Report: $OUTPUT_DIR/coverage-$TIMESTAMP.html"
echo "  - Benchmark Results: $BENCHMARK_FILE"
echo

# 9. Generate Quick Status File for CI/CD
echo "$OVERALL_STATUS" > "$OUTPUT_DIR/exit-code.txt"
echo "$([ $OVERALL_STATUS -eq 0 ] && echo "passed" || echo "failed")" > "$OUTPUT_DIR/status.txt"

# Exit with overall status
exit $OVERALL_STATUS
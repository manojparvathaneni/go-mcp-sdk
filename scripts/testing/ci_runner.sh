#!/bin/bash

# ci_runner.sh - CI/CD integration script for continuous testing
# This script is designed to be run in CI/CD pipelines (GitHub Actions, GitLab CI, Jenkins, etc.)

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="${RESULTS_DIR:-$PROJECT_ROOT/test-results}"

# CI/CD environment detection
CI_PLATFORM="unknown"
if [ -n "${GITHUB_ACTIONS:-}" ]; then
    CI_PLATFORM="github"
elif [ -n "${GITLAB_CI:-}" ]; then
    CI_PLATFORM="gitlab"
elif [ -n "${JENKINS_HOME:-}" ]; then
    CI_PLATFORM="jenkins"
elif [ -n "${CIRCLECI:-}" ]; then
    CI_PLATFORM="circleci"
fi

# Function to post results to various systems
post_results() {
    local analysis_file=$1
    local alert_file=$2
    
    # Slack webhook integration
    if [ -n "${SLACK_WEBHOOK_URL:-}" ]; then
        echo "Posting results to Slack..."
        
        local health_score=$(jq -r '.health_score' < "$alert_file")
        local severity=$(jq -r '.severity' < "$alert_file")
        local summary=$(jq -r '.summary' < "$alert_file")
        local color="good"
        
        if [ "$severity" = "critical" ]; then
            color="danger"
        elif [ "$severity" = "warning" ]; then
            color="warning"
        fi
        
        local slack_payload=$(cat <<EOF
{
    "attachments": [{
        "color": "$color",
        "title": "MCP Test Results - $summary",
        "fields": [
            {
                "title": "Health Score",
                "value": "${health_score}%",
                "short": true
            },
            {
                "title": "Platform",
                "value": "$CI_PLATFORM",
                "short": true
            },
            {
                "title": "Build",
                "value": "${BUILD_NUMBER:-${GITHUB_RUN_NUMBER:-Unknown}}",
                "short": true
            },
            {
                "title": "Branch",
                "value": "${BRANCH_NAME:-${GITHUB_REF_NAME:-${CI_COMMIT_REF_NAME:-Unknown}}}",
                "short": true
            }
        ],
        "footer": "MCP Test Suite",
        "ts": $(date +%s)
    }]
}
EOF
)
        
        curl -X POST -H 'Content-type: application/json' \
            --data "$slack_payload" \
            "$SLACK_WEBHOOK_URL" 2>/dev/null || echo "Failed to post to Slack"
    fi
    
    # GitHub Actions annotations
    if [ "$CI_PLATFORM" = "github" ]; then
        echo "Creating GitHub annotations..."
        
        # Extract critical issues and create annotations
        jq -r '.critical_issues[] | "::error::" + .message' < "$alert_file" || true
        jq -r '.warnings[] | "::warning::" + .message' < "$alert_file" || true
        
        # Set output variables
        echo "health_score=$(jq -r '.health_score' < "$alert_file")" >> "$GITHUB_OUTPUT"
        echo "test_status=$(jq -r '.status' < "$analysis_file")" >> "$GITHUB_OUTPUT"
    fi
    
    # GitLab CI artifacts
    if [ "$CI_PLATFORM" = "gitlab" ]; then
        echo "Configuring GitLab artifacts..."
        
        # Create JUnit report for GitLab
        if [ -f "$RESULTS_DIR/test-report-*.json" ]; then
            # Convert to JUnit format (simplified)
            cat > "$RESULTS_DIR/junit.xml" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
    <testsuite name="MCP Test Suite" tests="$(jq -r '.summary.total_tests' < "$analysis_file")" 
               failures="$(jq -r '.summary.total_failed' < "$analysis_file")"
               time="$(jq -r '.summary.total_duration_seconds' < "$analysis_file")">
        <testcase name="Overall Health" classname="MCP.Health">
            $([ "$(jq -r '.health_score' < "$alert_file")" -lt 50 ] && echo '<failure message="Health score below threshold"/>')
        </testcase>
    </testsuite>
</testsuites>
EOF
        fi
    fi
    
    # Generic webhook for custom integrations
    if [ -n "${WEBHOOK_URL:-}" ]; then
        echo "Posting to custom webhook..."
        curl -X POST -H 'Content-type: application/json' \
            --data "@$alert_file" \
            "$WEBHOOK_URL" 2>/dev/null || echo "Failed to post to webhook"
    fi
}

# Function to handle test failures
handle_failure() {
    local exit_code=$1
    local stage=$2
    
    echo "Test failure detected in stage: $stage (exit code: $exit_code)"
    
    # Create failure summary
    cat > "$RESULTS_DIR/failure-summary.json" <<EOF
{
    "stage": "$stage",
    "exit_code": $exit_code,
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "ci_platform": "$CI_PLATFORM",
    "build_url": "${BUILD_URL:-${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}}"
}
EOF
    
    # Quick analysis even on failure
    if [ -f "$RESULTS_DIR"/test-report-*.json ]; then
        "$SCRIPT_DIR/analyze_results.py" \
            --results-dir "$RESULTS_DIR" \
            --output alert \
            --output-file "$RESULTS_DIR/alert.json" || true
        
        post_results "$RESULTS_DIR/failure-summary.json" "$RESULTS_DIR/alert.json"
    fi
    
    exit $exit_code
}

# Main execution
echo "="
echo "MCP CI/CD Test Runner"
echo "="
echo "Platform: $CI_PLATFORM"
echo "Build: ${BUILD_NUMBER:-${GITHUB_RUN_NUMBER:-Unknown}}"
echo "Branch: ${BRANCH_NAME:-${GITHUB_REF_NAME:-${CI_COMMIT_REF_NAME:-Unknown}}}"
echo

# Install dependencies if needed
if ! command -v jq &> /dev/null; then
    echo "Installing jq..."
    if [ "$CI_PLATFORM" = "github" ]; then
        sudo apt-get update && sudo apt-get install -y jq
    elif [ "$CI_PLATFORM" = "gitlab" ]; then
        apk add --no-cache jq || apt-get update && apt-get install -y jq
    fi
fi

# Run tests with error handling
echo "Running comprehensive test suite..."
if ! "$SCRIPT_DIR/run_all_tests.sh"; then
    handle_failure $? "test_execution"
fi

# Analyze results
echo
echo "Analyzing test results..."
"$SCRIPT_DIR/analyze_results.py" \
    --results-dir "$RESULTS_DIR" \
    --output json \
    --output-file "$RESULTS_DIR/analysis.json"

"$SCRIPT_DIR/analyze_results.py" \
    --results-dir "$RESULTS_DIR" \
    --output alert \
    --output-file "$RESULTS_DIR/alert.json"

"$SCRIPT_DIR/analyze_results.py" \
    --results-dir "$RESULTS_DIR" \
    --output ci \
    --output-file "$RESULTS_DIR/ci-summary.json"

# Display human-readable summary
echo
"$SCRIPT_DIR/analyze_results.py" \
    --results-dir "$RESULTS_DIR" \
    --output human

# Post results to notification systems
post_results "$RESULTS_DIR/ci-summary.json" "$RESULTS_DIR/alert.json"

# Upload artifacts based on CI platform
case "$CI_PLATFORM" in
    github)
        echo "::notice::Test results available in $RESULTS_DIR"
        # GitHub Actions will automatically pick up artifacts if configured
        ;;
    gitlab)
        echo "Test artifacts available in $RESULTS_DIR"
        # GitLab CI will handle artifacts based on .gitlab-ci.yml
        ;;
    jenkins)
        # Archive artifacts for Jenkins
        if command -v zip &> /dev/null; then
            zip -r test-results.zip "$RESULTS_DIR"
        fi
        ;;
esac

# Determine exit status based on health score
HEALTH_SCORE=$(jq -r '.health_score' < "$RESULTS_DIR/alert.json")
if [ "${HEALTH_SCORE%.*}" -lt 50 ]; then
    echo
    echo "ERROR: Test suite health is below threshold (${HEALTH_SCORE}%)"
    exit 1
fi

echo
echo "Test suite completed successfully with health score: ${HEALTH_SCORE}%"
exit 0
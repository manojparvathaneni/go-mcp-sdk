#!/usr/bin/env python3

"""
analyze_results.py - Analyzes test results and generates actionable insights
"""

import json
import sys
import os
import argparse
from datetime import datetime
from typing import Dict, List, Any, Optional
import glob
from pathlib import Path

class TestResultAnalyzer:
    def __init__(self, results_dir: str):
        self.results_dir = Path(results_dir)
        self.history = []
        self.current_report = None
        
    def load_latest_report(self) -> Dict[str, Any]:
        """Load the most recent test report"""
        report_files = sorted(glob.glob(str(self.results_dir / "test-report-*.json")))
        if not report_files:
            raise FileNotFoundError("No test reports found in " + str(self.results_dir))
        
        with open(report_files[-1], 'r') as f:
            self.current_report = json.load(f)
        return self.current_report
    
    def load_history(self, days: int = 7) -> List[Dict[str, Any]]:
        """Load test history for trend analysis"""
        report_files = sorted(glob.glob(str(self.results_dir / "test-report-*.json")))
        
        for report_file in report_files[-days:]:  # Last N days of reports
            try:
                with open(report_file, 'r') as f:
                    self.history.append(json.load(f))
            except:
                continue
        
        return self.history
    
    def analyze_current_results(self) -> Dict[str, Any]:
        """Analyze the current test results"""
        if not self.current_report:
            self.load_latest_report()
        
        analysis = {
            "timestamp": datetime.now().isoformat(),
            "report_analyzed": self.current_report["timestamp"],
            "overall_health": self._calculate_health_score(),
            "critical_issues": self._identify_critical_issues(),
            "warnings": self._identify_warnings(),
            "improvements": self._identify_improvements(),
            "metrics": self._calculate_metrics(),
            "recommendations": self._generate_recommendations()
        }
        
        return analysis
    
    def analyze_trends(self) -> Dict[str, Any]:
        """Analyze trends over time"""
        if not self.history:
            self.load_history()
        
        if len(self.history) < 2:
            return {"error": "Not enough historical data for trend analysis"}
        
        trends = {
            "test_stability": self._analyze_test_stability(),
            "coverage_trend": self._analyze_coverage_trend(),
            "performance_trend": self._analyze_performance_trend(),
            "flaky_tests": self._identify_flaky_tests(),
            "regression_risks": self._identify_regression_risks()
        }
        
        return trends
    
    def _calculate_health_score(self) -> float:
        """Calculate overall health score (0-100)"""
        if not self.current_report:
            return 0.0
        
        summary = self.current_report.get("summary", {})
        
        # Factors for health score
        test_pass_rate = (summary.get("total_passed", 0) / max(summary.get("total_tests", 1), 1)) * 100
        suite_pass_rate = (summary.get("passed_suites", 0) / max(summary.get("total_suites", 1), 1)) * 100
        coverage = self.current_report.get("coverage", {}).get("percentage", 0)
        
        # Weighted health score
        health_score = (
            test_pass_rate * 0.4 +
            suite_pass_rate * 0.3 +
            min(coverage, 80) * 0.3 * 1.25  # Cap coverage contribution at 80%
        )
        
        return round(health_score, 2)
    
    def _identify_critical_issues(self) -> List[Dict[str, Any]]:
        """Identify critical issues that need immediate attention"""
        issues = []
        
        # Check for failed test suites
        for suite_name, suite_result in self.current_report.get("test_results", {}).items():
            if suite_result.get("status") == "failed":
                issues.append({
                    "type": "suite_failure",
                    "severity": "critical",
                    "suite": suite_name,
                    "failed_tests": suite_result.get("failed", 0),
                    "message": f"{suite_name} test suite failed with {suite_result.get('failed', 0)} failing tests"
                })
        
        # Check for low coverage
        coverage = self.current_report.get("coverage", {}).get("percentage", 0)
        if coverage < 50:
            issues.append({
                "type": "low_coverage",
                "severity": "critical",
                "coverage": coverage,
                "message": f"Code coverage is critically low at {coverage}%"
            })
        
        # Check for harness failures
        harness = self.current_report.get("harness", {})
        if harness and harness.get("summary", {}).get("failed", 0) > 0:
            issues.append({
                "type": "compliance_failure",
                "severity": "critical",
                "failed_tests": harness.get("summary", {}).get("failed", 0),
                "message": "MCP compliance tests are failing"
            })
        
        return issues
    
    def _identify_warnings(self) -> List[Dict[str, Any]]:
        """Identify warnings that should be addressed"""
        warnings = []
        
        # Check for declining coverage
        coverage = self.current_report.get("coverage", {}).get("percentage", 0)
        if 50 <= coverage < 70:
            warnings.append({
                "type": "moderate_coverage",
                "severity": "warning",
                "coverage": coverage,
                "message": f"Code coverage is below recommended level at {coverage}%"
            })
        
        # Check for slow tests
        for suite_name, suite_result in self.current_report.get("test_results", {}).items():
            duration = suite_result.get("duration_seconds", 0)
            if duration > 300:  # 5 minutes
                warnings.append({
                    "type": "slow_tests",
                    "severity": "warning",
                    "suite": suite_name,
                    "duration": duration,
                    "message": f"{suite_name} tests are taking {duration}s to complete"
                })
        
        # Check for skipped tests
        total_skipped = sum(
            suite.get("skipped", 0) 
            for suite in self.current_report.get("test_results", {}).values()
        )
        if total_skipped > 0:
            warnings.append({
                "type": "skipped_tests",
                "severity": "warning",
                "count": total_skipped,
                "message": f"{total_skipped} tests are being skipped"
            })
        
        return warnings
    
    def _identify_improvements(self) -> List[Dict[str, Any]]:
        """Identify areas for improvement"""
        improvements = []
        
        # Benchmark analysis
        benchmarks = self.current_report.get("benchmarks", {}).get("results", [])
        if benchmarks:
            # Simple analysis - in practice you'd want more sophisticated parsing
            improvements.append({
                "type": "performance",
                "area": "benchmarks",
                "message": "Review benchmark results for performance optimization opportunities"
            })
        
        # Coverage improvements
        coverage = self.current_report.get("coverage", {}).get("percentage", 0)
        if coverage < 80:
            improvements.append({
                "type": "coverage",
                "current": coverage,
                "target": 80,
                "message": f"Increase code coverage from {coverage}% to 80%"
            })
        
        return improvements
    
    def _calculate_metrics(self) -> Dict[str, Any]:
        """Calculate key metrics"""
        summary = self.current_report.get("summary", {})
        
        metrics = {
            "test_pass_rate": round((summary.get("total_passed", 0) / max(summary.get("total_tests", 1), 1)) * 100, 2),
            "suite_pass_rate": round((summary.get("passed_suites", 0) / max(summary.get("total_suites", 1), 1)) * 100, 2),
            "code_coverage": self.current_report.get("coverage", {}).get("percentage", 0),
            "total_execution_time": summary.get("total_duration_seconds", 0),
            "tests_per_second": round(summary.get("total_tests", 0) / max(summary.get("total_duration_seconds", 1), 1), 2)
        }
        
        return metrics
    
    def _generate_recommendations(self) -> List[str]:
        """Generate actionable recommendations"""
        recommendations = []
        
        health_score = self._calculate_health_score()
        
        if health_score < 50:
            recommendations.append("URGENT: Test suite health is critical. Immediate action required.")
        elif health_score < 70:
            recommendations.append("Test suite health needs improvement. Schedule time for test fixes.")
        
        # Specific recommendations based on issues
        for issue in self._identify_critical_issues():
            if issue["type"] == "suite_failure":
                recommendations.append(f"Fix failing tests in {issue['suite']} suite immediately")
            elif issue["type"] == "low_coverage":
                recommendations.append("Add unit tests to increase code coverage above 50%")
            elif issue["type"] == "compliance_failure":
                recommendations.append("Review and fix MCP compliance issues")
        
        for warning in self._identify_warnings():
            if warning["type"] == "slow_tests":
                recommendations.append(f"Optimize {warning['suite']} tests to reduce execution time")
            elif warning["type"] == "skipped_tests":
                recommendations.append("Review and enable skipped tests or remove them")
        
        return recommendations
    
    def _analyze_test_stability(self) -> Dict[str, Any]:
        """Analyze test stability over time"""
        if len(self.history) < 2:
            return {"status": "insufficient_data"}
        
        # Calculate pass rates over time
        pass_rates = []
        for report in self.history:
            summary = report.get("summary", {})
            if summary.get("total_tests", 0) > 0:
                pass_rate = (summary.get("total_passed", 0) / summary.get("total_tests", 1)) * 100
                pass_rates.append(pass_rate)
        
        if not pass_rates:
            return {"status": "no_data"}
        
        avg_pass_rate = sum(pass_rates) / len(pass_rates)
        stability_score = 100 - (max(pass_rates) - min(pass_rates))  # Less variation = more stable
        
        return {
            "average_pass_rate": round(avg_pass_rate, 2),
            "stability_score": round(stability_score, 2),
            "trend": "stable" if stability_score > 90 else "unstable"
        }
    
    def _analyze_coverage_trend(self) -> Dict[str, Any]:
        """Analyze coverage trend over time"""
        coverages = []
        for report in self.history:
            coverage = report.get("coverage", {}).get("percentage", 0)
            if coverage > 0:
                coverages.append(coverage)
        
        if len(coverages) < 2:
            return {"status": "insufficient_data"}
        
        trend = "increasing" if coverages[-1] > coverages[0] else "decreasing" if coverages[-1] < coverages[0] else "stable"
        
        return {
            "current": coverages[-1] if coverages else 0,
            "previous": coverages[-2] if len(coverages) > 1 else 0,
            "trend": trend,
            "change": round(coverages[-1] - coverages[0], 2) if coverages else 0
        }
    
    def _analyze_performance_trend(self) -> Dict[str, Any]:
        """Analyze performance trends"""
        execution_times = []
        for report in self.history:
            duration = report.get("summary", {}).get("total_duration_seconds", 0)
            if duration > 0:
                execution_times.append(duration)
        
        if not execution_times:
            return {"status": "no_data"}
        
        avg_time = sum(execution_times) / len(execution_times)
        trend = "slower" if execution_times[-1] > avg_time * 1.1 else "faster" if execution_times[-1] < avg_time * 0.9 else "stable"
        
        return {
            "current_duration": execution_times[-1] if execution_times else 0,
            "average_duration": round(avg_time, 2),
            "trend": trend
        }
    
    def _identify_flaky_tests(self) -> List[str]:
        """Identify potentially flaky tests"""
        # This is a simplified implementation
        # In practice, you'd track individual test results over time
        flaky_suites = []
        
        suite_results = {}
        for report in self.history:
            for suite_name, result in report.get("test_results", {}).items():
                if suite_name not in suite_results:
                    suite_results[suite_name] = []
                suite_results[suite_name].append(result.get("status"))
        
        for suite_name, statuses in suite_results.items():
            if len(set(statuses)) > 1:  # Status changed over time
                flaky_suites.append(suite_name)
        
        return flaky_suites
    
    def _identify_regression_risks(self) -> List[Dict[str, Any]]:
        """Identify potential regression risks"""
        risks = []
        
        if len(self.history) < 2:
            return risks
        
        # Compare current with previous
        current = self.history[-1]
        previous = self.history[-2]
        
        # Check for new failures
        for suite_name, result in current.get("test_results", {}).items():
            prev_result = previous.get("test_results", {}).get(suite_name, {})
            
            if result.get("status") == "failed" and prev_result.get("status") == "passed":
                risks.append({
                    "type": "new_failure",
                    "suite": suite_name,
                    "message": f"{suite_name} suite started failing"
                })
            
            # Check for increased failure count
            if result.get("failed", 0) > prev_result.get("failed", 0):
                risks.append({
                    "type": "increased_failures",
                    "suite": suite_name,
                    "previous_failures": prev_result.get("failed", 0),
                    "current_failures": result.get("failed", 0),
                    "message": f"{suite_name} has more failing tests"
                })
        
        # Check for coverage drop
        current_coverage = current.get("coverage", {}).get("percentage", 0)
        previous_coverage = previous.get("coverage", {}).get("percentage", 0)
        
        if current_coverage < previous_coverage - 5:  # 5% drop threshold
            risks.append({
                "type": "coverage_drop",
                "previous": previous_coverage,
                "current": current_coverage,
                "message": f"Code coverage dropped from {previous_coverage}% to {current_coverage}%"
            })
        
        return risks

def generate_alert_payload(analysis: Dict[str, Any]) -> Dict[str, Any]:
    """Generate alert payload for notification systems"""
    health_score = analysis.get("overall_health", 0)
    critical_issues = analysis.get("critical_issues", [])
    
    severity = "critical" if health_score < 50 or critical_issues else "warning" if health_score < 70 else "info"
    
    payload = {
        "severity": severity,
        "health_score": health_score,
        "summary": f"Test Suite Health: {health_score}%",
        "critical_issues": critical_issues,
        "warnings": analysis.get("warnings", []),
        "recommendations": analysis.get("recommendations", []),
        "metrics": analysis.get("metrics", {}),
        "timestamp": analysis.get("timestamp"),
        "should_alert": severity in ["critical", "warning"]
    }
    
    return payload

def generate_ci_output(analysis: Dict[str, Any], trends: Dict[str, Any]) -> Dict[str, Any]:
    """Generate output suitable for CI/CD systems"""
    return {
        "status": "failed" if analysis.get("overall_health", 0) < 50 else "passed",
        "health_score": analysis.get("overall_health", 0),
        "critical_issues_count": len(analysis.get("critical_issues", [])),
        "warnings_count": len(analysis.get("warnings", [])),
        "metrics": analysis.get("metrics", {}),
        "trends": {
            "stability": trends.get("test_stability", {}).get("trend", "unknown"),
            "coverage": trends.get("coverage_trend", {}).get("trend", "unknown"),
            "performance": trends.get("performance_trend", {}).get("trend", "unknown")
        },
        "regression_risks": len(trends.get("regression_risks", [])),
        "flaky_tests": len(trends.get("flaky_tests", []))
    }

def main():
    parser = argparse.ArgumentParser(description="Analyze MCP test results")
    parser.add_argument("--results-dir", default="test-results", help="Directory containing test results")
    parser.add_argument("--output", choices=["human", "json", "alert", "ci"], default="human", help="Output format")
    parser.add_argument("--history-days", type=int, default=7, help="Days of history to analyze")
    parser.add_argument("--output-file", help="Output file (default: stdout)")
    
    args = parser.parse_args()
    
    try:
        analyzer = TestResultAnalyzer(args.results_dir)
        
        # Perform analysis
        current_analysis = analyzer.analyze_current_results()
        trend_analysis = analyzer.analyze_trends()
        
        # Generate output based on format
        if args.output == "human":
            output = format_human_readable(current_analysis, trend_analysis)
        elif args.output == "json":
            output = json.dumps({
                "current": current_analysis,
                "trends": trend_analysis
            }, indent=2)
        elif args.output == "alert":
            output = json.dumps(generate_alert_payload(current_analysis), indent=2)
        elif args.output == "ci":
            output = json.dumps(generate_ci_output(current_analysis, trend_analysis), indent=2)
        
        # Write output
        if args.output_file:
            with open(args.output_file, 'w') as f:
                f.write(output)
        else:
            print(output)
        
        # Exit with appropriate code for CI/CD
        if current_analysis.get("overall_health", 0) < 50:
            sys.exit(1)
        
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

def format_human_readable(analysis: Dict[str, Any], trends: Dict[str, Any]) -> str:
    """Format analysis results for human consumption"""
    output = []
    
    output.append("=" * 80)
    output.append("MCP TEST RESULTS ANALYSIS")
    output.append("=" * 80)
    output.append("")
    
    # Overall Health
    health_score = analysis.get("overall_health", 0)
    health_emoji = "🟢" if health_score >= 80 else "🟡" if health_score >= 60 else "🔴"
    output.append(f"Overall Health Score: {health_emoji} {health_score}%")
    output.append("")
    
    # Metrics
    metrics = analysis.get("metrics", {})
    output.append("Key Metrics:")
    output.append(f"  - Test Pass Rate: {metrics.get('test_pass_rate', 0)}%")
    output.append(f"  - Suite Pass Rate: {metrics.get('suite_pass_rate', 0)}%")
    output.append(f"  - Code Coverage: {metrics.get('code_coverage', 0)}%")
    output.append(f"  - Total Execution Time: {metrics.get('total_execution_time', 0)}s")
    output.append(f"  - Tests Per Second: {metrics.get('tests_per_second', 0)}")
    output.append("")
    
    # Critical Issues
    critical_issues = analysis.get("critical_issues", [])
    if critical_issues:
        output.append(f"🔴 Critical Issues ({len(critical_issues)}):")
        for issue in critical_issues:
            output.append(f"  - {issue['message']}")
        output.append("")
    
    # Warnings
    warnings = analysis.get("warnings", [])
    if warnings:
        output.append(f"🟡 Warnings ({len(warnings)}):")
        for warning in warnings:
            output.append(f"  - {warning['message']}")
        output.append("")
    
    # Trends
    if trends and trends != {"error": "Not enough historical data for trend analysis"}:
        output.append("Trends:")
        
        stability = trends.get("test_stability", {})
        if stability.get("trend"):
            output.append(f"  - Test Stability: {stability.get('trend')} (score: {stability.get('stability_score', 0)})")
        
        coverage = trends.get("coverage_trend", {})
        if coverage.get("trend"):
            output.append(f"  - Coverage Trend: {coverage.get('trend')} ({coverage.get('change', 0):+.1f}%)")
        
        performance = trends.get("performance_trend", {})
        if performance.get("trend"):
            output.append(f"  - Performance: {performance.get('trend')}")
        
        flaky = trends.get("flaky_tests", [])
        if flaky:
            output.append(f"  - Flaky Test Suites: {', '.join(flaky)}")
        
        output.append("")
    
    # Recommendations
    recommendations = analysis.get("recommendations", [])
    if recommendations:
        output.append("Recommendations:")
        for i, rec in enumerate(recommendations, 1):
            output.append(f"  {i}. {rec}")
        output.append("")
    
    output.append("-" * 80)
    output.append(f"Report analyzed: {analysis.get('report_analyzed', 'Unknown')}")
    output.append(f"Analysis timestamp: {analysis.get('timestamp', 'Unknown')}")
    
    return "\n".join(output)

if __name__ == "__main__":
    main()
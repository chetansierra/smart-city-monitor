package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type TestResult struct {
	Endpoint      string
	TotalRequests int
	SuccessCount  int
	FailureCount  int
	AvgLatency    time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
	Latencies     []time.Duration
}

type PerformanceReport struct {
	Results         []TestResult
	TotalDuration   time.Duration
	RequestsPerSec  float64
	OverallSuccess  float64
}

const (
	baseURL           = "http://localhost:8080"
	concurrentClients = 100
	requestsPerClient = 10
)

func main() {
	fmt.Println("🚀 Starting API Performance Test")
	fmt.Printf("Base URL: %s\n", baseURL)
	fmt.Printf("Concurrent clients: %d\n", concurrentClients)
	fmt.Printf("Requests per client: %d\n\n", requestsPerClient)

	// Check if API is running
	if !checkAPIHealth() {
		fmt.Println("❌ API is not running. Please start the API gateway first.")
		return
	}

	fmt.Println("✅ API is healthy. Starting load tests...\n")

	// Define endpoints to test
	endpoints := []string{
		"/api/v1/sensors",
		"/api/v1/readings?limit=50",
		"/api/v1/readings/latest",
		"/api/v1/analytics/city-stats",
		"/api/v1/analytics/top-polluted?limit=10",
		"/api/v1/alerts?limit=20",
		"/health",
	}

	startTime := time.Now()
	var results []TestResult

	for _, endpoint := range endpoints {
		fmt.Printf("Testing: %s\n", endpoint)
		result := loadTestEndpoint(endpoint)
		results = append(results, result)
		printEndpointResult(result)
		time.Sleep(500 * time.Millisecond) // Brief pause between endpoint tests
	}

	totalDuration := time.Since(startTime)

	// Generate report
	report := generateReport(results, totalDuration)
	printReport(report)

	// Save report to file
	saveReport(report)
}

func checkAPIHealth() bool {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func loadTestEndpoint(endpoint string) TestResult {
	url := baseURL + endpoint
	var wg sync.WaitGroup
	var mu sync.Mutex

	result := TestResult{
		Endpoint:      endpoint,
		TotalRequests: concurrentClients * requestsPerClient,
		Latencies:     make([]time.Duration, 0),
		MinLatency:    time.Hour, // Initialize with a large value
	}

	// Launch concurrent clients
	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each client makes multiple requests
			for j := 0; j < requestsPerClient; j++ {
				start := time.Now()
				resp, err := http.Get(url)
				latency := time.Since(start)

				mu.Lock()
				result.Latencies = append(result.Latencies, latency)

				if err != nil || resp.StatusCode != 200 {
					result.FailureCount++
				} else {
					result.SuccessCount++
					io.Copy(io.Discard, resp.Body) // Drain the body
				}

				if resp != nil {
					resp.Body.Close()
				}

				// Update min/max
				if latency < result.MinLatency {
					result.MinLatency = latency
				}
				if latency > result.MaxLatency {
					result.MaxLatency = latency
				}

				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Calculate average latency
	var totalLatency time.Duration
	for _, lat := range result.Latencies {
		totalLatency += lat
	}
	result.AvgLatency = totalLatency / time.Duration(len(result.Latencies))

	return result
}

func generateReport(results []TestResult, totalDuration time.Duration) PerformanceReport {
	var totalRequests, totalSuccess int
	for _, r := range results {
		totalRequests += r.TotalRequests
		totalSuccess += r.SuccessCount
	}

	successRate := float64(totalSuccess) / float64(totalRequests) * 100
	reqPerSec := float64(totalRequests) / totalDuration.Seconds()

	return PerformanceReport{
		Results:         results,
		TotalDuration:   totalDuration,
		RequestsPerSec:  reqPerSec,
		OverallSuccess:  successRate,
	}
}

func printEndpointResult(result TestResult) {
	successRate := float64(result.SuccessCount) / float64(result.TotalRequests) * 100

	fmt.Printf("  ✓ Success: %d/%d (%.1f%%)\n", result.SuccessCount, result.TotalRequests, successRate)
	fmt.Printf("  ⏱  Latency - Avg: %v, Min: %v, Max: %v\n", result.AvgLatency, result.MinLatency, result.MaxLatency)
	fmt.Printf("  📊 P95: %v, P99: %v\n", calculatePercentile(result.Latencies, 95), calculatePercentile(result.Latencies, 99))
	fmt.Println()
}

func printReport(report PerformanceReport) {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("📈 PERFORMANCE TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Total Duration: %v\n", report.TotalDuration)
	fmt.Printf("Total Requests: %d\n", getTotalRequests(report.Results))
	fmt.Printf("Requests/sec: %.2f\n", report.RequestsPerSec)
	fmt.Printf("Overall Success Rate: %.2f%%\n", report.OverallSuccess)
	fmt.Println()

	fmt.Println("Endpoint Performance Breakdown:")
	fmt.Println(strings.Repeat("-", 70))
	for _, r := range report.Results {
		status := "✅"
		if r.AvgLatency > 1*time.Second {
			status = "⚠️ "
		}
		if r.AvgLatency > 2*time.Second {
			status = "❌"
		}

		fmt.Printf("%s %-45s Avg: %8v  Max: %8v\n",
			status, r.Endpoint, r.AvgLatency, r.MaxLatency)
	}
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Performance assessment
	fmt.Println("Performance Assessment:")
	slowEndpoints := 0
	for _, r := range report.Results {
		if r.AvgLatency > 200*time.Millisecond {
			slowEndpoints++
		}
	}

	if slowEndpoints == 0 {
		fmt.Println("✅ All endpoints are performing well (< 200ms)")
	} else if slowEndpoints <= 2 {
		fmt.Printf("⚠️  %d endpoint(s) need optimization (> 200ms)\n", slowEndpoints)
	} else {
		fmt.Printf("❌ %d endpoint(s) are slow (> 200ms) - optimization required\n", slowEndpoints)
	}

	if report.OverallSuccess >= 99 {
		fmt.Println("✅ Excellent reliability (>99% success rate)")
	} else if report.OverallSuccess >= 95 {
		fmt.Println("⚠️  Good reliability (>95% success rate)")
	} else {
		fmt.Println("❌ Poor reliability (<95% success rate)")
	}
}

func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Simple percentile calculation
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)

	// Bubble sort (simple for small datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func getTotalRequests(results []TestResult) int {
	total := 0
	for _, r := range results {
		total += r.TotalRequests
	}
	return total
}

func saveReport(report PerformanceReport) {
	// Create markdown report
	mdReport := generateMarkdownReport(report)

	// Save to file
	filename := fmt.Sprintf("/Users/chetansierra/Dev/personalProjects/smart-city-monitor/docs/api-performance.md")

	err := saveToFile(filename, mdReport)
	if err != nil {
		fmt.Printf("⚠️  Could not save report: %v\n", err)
		return
	}

	fmt.Printf("📄 Report saved to: %s\n", filename)
}

func generateMarkdownReport(report PerformanceReport) string {
	md := "# API Performance Test Report\n\n"
	md += fmt.Sprintf("**Test Date:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	md += "## Test Configuration\n\n"
	md += fmt.Sprintf("- Base URL: `%s`\n", baseURL)
	md += fmt.Sprintf("- Concurrent Clients: %d\n", concurrentClients)
	md += fmt.Sprintf("- Requests per Client: %d\n", requestsPerClient)
	md += fmt.Sprintf("- Total Requests: %d\n\n", getTotalRequests(report.Results))

	md += "## Overall Results\n\n"
	md += fmt.Sprintf("- **Total Duration:** %v\n", report.TotalDuration)
	md += fmt.Sprintf("- **Throughput:** %.2f requests/sec\n", report.RequestsPerSec)
	md += fmt.Sprintf("- **Success Rate:** %.2f%%\n\n", report.OverallSuccess)

	md += "## Endpoint Performance\n\n"
	md += "| Endpoint | Requests | Success | Avg Latency | Min | Max | P95 | P99 |\n"
	md += "|----------|----------|---------|-------------|-----|-----|-----|-----|\n"

	for _, r := range report.Results {
		md += fmt.Sprintf("| `%s` | %d | %.1f%% | %v | %v | %v | %v | %v |\n",
			r.Endpoint,
			r.TotalRequests,
			float64(r.SuccessCount)/float64(r.TotalRequests)*100,
			r.AvgLatency,
			r.MinLatency,
			r.MaxLatency,
			calculatePercentile(r.Latencies, 95),
			calculatePercentile(r.Latencies, 99),
		)
	}

	md += "\n## Performance Analysis\n\n"

	// Identify slow endpoints
	md += "### Endpoints by Performance\n\n"
	md += "**Fast Endpoints (< 200ms):**\n"
	hasfast := false
	for _, r := range report.Results {
		if r.AvgLatency < 200*time.Millisecond {
			md += fmt.Sprintf("- ✅ `%s` - %v\n", r.Endpoint, r.AvgLatency)
			hasfast = true
		}
	}
	if !hasfast {
		md += "- None\n"
	}

	md += "\n**Moderate Endpoints (200ms - 1s):**\n"
	hasModerate := false
	for _, r := range report.Results {
		if r.AvgLatency >= 200*time.Millisecond && r.AvgLatency < 1*time.Second {
			md += fmt.Sprintf("- ⚠️  `%s` - %v\n", r.Endpoint, r.AvgLatency)
			hasModerate = true
		}
	}
	if !hasModerate {
		md += "- None\n"
	}

	md += "\n**Slow Endpoints (> 1s):**\n"
	hasSlow := false
	for _, r := range report.Results {
		if r.AvgLatency >= 1*time.Second {
			md += fmt.Sprintf("- ❌ `%s` - %v\n", r.Endpoint, r.AvgLatency)
			hasSlow = true
		}
	}
	if !hasSlow {
		md += "- None\n"
	}

	md += "\n## Recommendations\n\n"
	if report.OverallSuccess < 95 {
		md += "- ⚠️  **Success rate is below 95%** - investigate failures\n"
	}

	slowCount := 0
	for _, r := range report.Results {
		if r.AvgLatency > 200*time.Millisecond {
			slowCount++
		}
	}

	if slowCount > 0 {
		md += fmt.Sprintf("- ⚠️  **%d endpoints** need optimization (>200ms average latency)\n", slowCount)
		md += "- Consider adding database indexes\n"
		md += "- Implement more aggressive caching\n"
		md += "- Review query complexity\n"
	} else {
		md += "- ✅ All endpoints performing within acceptable range\n"
	}

	md += "\n---\n"
	md += "*Generated by Smart City Monitor Performance Test Suite*\n"

	return md
}

func saveToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

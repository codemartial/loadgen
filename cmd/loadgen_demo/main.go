package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/codemartial/loadgen"
)

func main() {
	// Define multiple load specs to demonstrate seamless stitching
	specs := []loadgen.LoadSpec{
		{
			RPM:          300,  // 5 requests per second
			ErrorRate:    0.05, // 5% error rate
			DurationS:    30,   // Run for 30 seconds
			P50LatencyMS: 100,  // 100ms p50
			P99LatencyMS: 500,  // 500ms p99
			TimeoutMS:    1000, // 1s timeout
		},
		{
			RPM:          600,  // 10 requests per second
			ErrorRate:    0.10, // 10% error rate
			DurationS:    30,   // Run for 30 seconds
			P50LatencyMS: 200,  // 200ms p50
			P99LatencyMS: 800,  // 800ms p99
			TimeoutMS:    1500, // 1.5s timeout
		},
		{
			RPM:          900,  // 15 requests per second
			ErrorRate:    0.02, // 2% error rate
			DurationS:    30,   // Run for 30 seconds
			P50LatencyMS: 50,   // 50ms p50
			P99LatencyMS: 300,  // 300ms p99
			TimeoutMS:    800,  // 800ms timeout
		},
	}

	gen := loadgen.NewLoadGenerator(specs)

	var events []*loadgen.LoadEvent
	var durations []float64
	var successCount, failureCount int
	var timeoutFailures, errorFailures int

	// Generate events
	for event := gen.Next(); event != nil; event = gen.Next() {
		fmt.Println(event.Timestamp, " ",  event.Duration.Milliseconds())
		events = append(events, event)
		durations = append(durations, float64(event.Duration.Milliseconds()))

		if event.Success {
			successCount++
		} else {
			failureCount++
			// Check if failure was due to timeout
			if float64(event.Duration.Milliseconds()) > specs[0].TimeoutMS {
				timeoutFailures++
			} else {
				errorFailures++
			}
		}

		if len(events) >= 500 {
			break
		}
	}

	fmt.Printf("Generated %d events\n\n", len(events))

	// Calculate statistics per spec
	fmt.Println("=== Per-Spec Analysis ===")
	specBoundaries := []int{0}
	currentSpec := 0
	for i := 1; i < len(events); i++ {
		// Detect spec boundaries by looking at large time jumps or timestamp resets
		if i > 0 && events[i].Timestamp.Sub(events[i-1].Timestamp) > 2*time.Second {
			specBoundaries = append(specBoundaries, i)
			currentSpec++
		}
	}
	specBoundaries = append(specBoundaries, len(events))

	for specIdx := 0; specIdx < len(specBoundaries)-1 && specIdx < len(specs); specIdx++ {
		start := specBoundaries[specIdx]
		end := specBoundaries[specIdx+1]
		specEvents := events[start:end]

		if len(specEvents) == 0 {
			continue
		}

		fmt.Printf("\nSpec %d (RPM=%d, ErrorRate=%.2f, P50=%.0fms, P99=%.0fms, Timeout=%.0fms):\n",
			specIdx+1, specs[specIdx].RPM, specs[specIdx].ErrorRate,
			specs[specIdx].P50LatencyMS, specs[specIdx].P99LatencyMS, specs[specIdx].TimeoutMS)

		var specDurations []float64
		var specSuccess, specFail int
		for _, e := range specEvents {
			specDurations = append(specDurations, float64(e.Duration.Milliseconds()))
			if e.Success {
				specSuccess++
			} else {
				specFail++
			}
		}

		// Calculate percentiles
		sort.Float64s(specDurations)
		p50 := percentile(specDurations, 50)
		p99 := percentile(specDurations, 99)
		p95 := percentile(specDurations, 95)

		// Calculate inter-arrival times
		var interArrivals []float64
		for i := 1; i < len(specEvents); i++ {
			interArrivals = append(interArrivals, specEvents[i].Timestamp.Sub(specEvents[i-1].Timestamp).Seconds()*1000)
		}
		sort.Float64s(interArrivals)

		actualErrorRate := float64(specFail) / float64(len(specEvents))
		timeSpan := specEvents[len(specEvents)-1].Timestamp.Sub(specEvents[0].Timestamp).Seconds()
		actualRPM := float64(len(specEvents)) / timeSpan * 60

		fmt.Printf("  Events: %d\n", len(specEvents))
		fmt.Printf("  Time span: %.2fs\n", timeSpan)
		fmt.Printf("  Actual RPM: %.0f (expected: %d)\n", actualRPM, specs[specIdx].RPM)
		fmt.Printf("  Success: %d, Failures: %d\n", specSuccess, specFail)
		fmt.Printf("  Actual error rate: %.3f (expected: %.3f)\n", actualErrorRate, specs[specIdx].ErrorRate)
		fmt.Printf("  Latency p50: %.2fms (expected: %.0fms)\n", p50, specs[specIdx].P50LatencyMS)
		fmt.Printf("  Latency p95: %.2fms\n", p95)
		fmt.Printf("  Latency p99: %.2fms (expected: %.0fms)\n", p99, specs[specIdx].P99LatencyMS)
		if len(interArrivals) > 0 {
			avgInterArrival := percentile(interArrivals, 50)
			expectedInterArrival := (60.0 / float64(specs[specIdx].RPM)) * 1000
			fmt.Printf("  Avg inter-arrival: %.2fms (expected: %.2fms)\n", avgInterArrival, expectedInterArrival)
		}
	}

	// Overall statistics
	fmt.Printf("\n=== Overall Statistics ===\n")
	fmt.Printf("Total events: %d\n", len(events))
	fmt.Printf("Success: %d (%.1f%%)\n", successCount, float64(successCount)/float64(len(events))*100)
	fmt.Printf("Failures: %d (%.1f%%)\n", failureCount, float64(failureCount)/float64(len(events))*100)
	fmt.Printf("  - Timeout failures: %d\n", timeoutFailures)
	fmt.Printf("  - Error failures: %d\n", errorFailures)

	// Show sample events
	fmt.Printf("\n=== Sample Events (first 10) ===\n")
	for i := 0; i < 10 && i < len(events); i++ {
		e := events[i]
		fmt.Printf("Event %d: T=%s, Duration=%dms, Success=%v\n",
			i+1, e.Timestamp.Format("15:04:05.000"), e.Duration.Milliseconds(), e.Success)
	}

	fmt.Printf("\n=== Sample Events (around spec boundary ~event 150) ===\n")
	for i := 145; i < 155 && i < len(events); i++ {
		e := events[i]
		fmt.Printf("Event %d: T=%s, Duration=%dms, Success=%v\n",
			i+1, e.Timestamp.Format("15:04:05.000"), e.Duration.Milliseconds(), e.Success)
	}
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)) * float64(p) / 100.0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

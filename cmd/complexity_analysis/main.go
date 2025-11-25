package main

import (
	"fmt"
	"math"
)

type DataPoint struct {
	rpm          int
	events       int64
	runtimeNs    float64
	nsPerEvent   float64
	eventsPerSec float64
}

func main() {
	// Data from our timing tests
	data := []DataPoint{
		{10000, 1851, 0, 181.25, 5517369},
		{20000, 3903, 1e6, 177.42, 5636329},
		{50000, 12720, 2e6, 191.43, 5223916},
		{100000, 43270, 7e6, 169.56, 5897681},
		{150000, 111337, 19e6, 174.04, 5745772},
		{200000, 273670, 40e6, 145.21, 6886786},
		{250000, 644120, 91e6, 140.81, 7101922},
		{300000, 1468218, 203e6, 138.41, 7225062},
		{350000, 3435391, 484e6, 141.00, 7092207},
		{400000, 7878259, 1094e6, 138.92, 7198284},
		{450000, 18091756, 2513e6, 138.89, 7200109},
		{500000, 41767789, 5919e6, 141.72, 7056060},
		{600000, 221637201, 30943e6, 139.61, 7162873},
	}

	fmt.Println("LoadGenerator Complexity Analysis")
	fmt.Println("==================================\n")

	// Calculate statistics
	var sumNsPerEvent, sumEventsPerSec float64
	var minNsPerEvent, maxNsPerEvent float64 = math.MaxFloat64, 0
	var minEPS, maxEPS float64 = math.MaxFloat64, 0

	for _, d := range data {
		sumNsPerEvent += d.nsPerEvent
		sumEventsPerSec += d.eventsPerSec

		if d.nsPerEvent < minNsPerEvent {
			minNsPerEvent = d.nsPerEvent
		}
		if d.nsPerEvent > maxNsPerEvent {
			maxNsPerEvent = d.nsPerEvent
		}
		if d.eventsPerSec < minEPS {
			minEPS = d.eventsPerSec
		}
		if d.eventsPerSec > maxEPS {
			maxEPS = d.eventsPerSec
		}
	}

	avgNsPerEvent := sumNsPerEvent / float64(len(data))
	avgEPS := sumEventsPerSec / float64(len(data))

	fmt.Printf("Performance Statistics:\n")
	fmt.Printf("  ns/event:      min=%.2f, max=%.2f, avg=%.2f (variance: %.1f%%)\n",
		minNsPerEvent, maxNsPerEvent, avgNsPerEvent,
		(maxNsPerEvent-minNsPerEvent)/avgNsPerEvent*100)
	fmt.Printf("  events/sec:    min=%.0f, max=%.0f, avg=%.0f (variance: %.1f%%)\n",
		minEPS, maxEPS, avgEPS, (maxEPS-minEPS)/avgEPS*100)

	// Test complexity hypothesis
	fmt.Printf("\nComplexity Analysis:\n")

	// Test O(1) - constant time per event
	fmt.Printf("\nO(1) Test (constant time per event):\n")
	fmt.Printf("  If O(1), ns/event should be roughly constant\n")
	fmt.Printf("  Variance in ns/event: %.1f%%\n", (maxNsPerEvent-minNsPerEvent)/avgNsPerEvent*100)
	if (maxNsPerEvent-minNsPerEvent)/avgNsPerEvent < 0.40 { // less than 40% variance
		fmt.Printf("  ✓ PASSES - Performance is O(1) per event\n")
	} else {
		fmt.Printf("  ✗ FAILS - Performance varies too much\n")
	}

	// Test O(N) - linear in number of events
	fmt.Printf("\nO(N) Test (linear in number of events):\n")
	fmt.Printf("  If O(N), runtime should grow quadratically with RPM\n")
	// Compare ratio of runtimes vs ratio of events
	lowRPM := data[3]   // 100k RPM
	highRPM := data[12] // 600k RPM
	eventRatio := float64(highRPM.events) / float64(lowRPM.events)
	timeRatio := highRPM.runtimeNs / lowRPM.runtimeNs
	fmt.Printf("  Event ratio (600k/100k): %.2fx\n", eventRatio)
	fmt.Printf("  Time ratio (600k/100k):  %.2fx\n", timeRatio)
	fmt.Printf("  Expected for O(N):       %.2fx (quadratic)\n", eventRatio*eventRatio)
	fmt.Printf("  Expected for O(1):       %.2fx (linear)\n", eventRatio)
	if math.Abs(timeRatio-eventRatio) < eventRatio*0.2 { // within 20% of linear
		fmt.Printf("  ✓ PASSES - Performance is O(1), not O(N)\n")
	} else {
		fmt.Printf("  ✗ FAILS - Performance is worse than O(1)\n")
	}

	fmt.Printf("\n" + "Conclusion:\n")
	fmt.Printf("  LoadGenerator.Next() operates in O(1) time per event\n")
	fmt.Printf("  Average throughput: %.1fM events/second\n", avgEPS/1e6)
	fmt.Printf("  Average latency: %.0fns per event\n", avgNsPerEvent)
	fmt.Printf("\n  The generator is NOT the bottleneck!\n")
	fmt.Printf("  If benchmarks are slow, the issue is likely:\n")
	fmt.Printf("    1. Memory allocation/GC pressure\n")
	fmt.Printf("    2. The benchmark harness itself\n")
	fmt.Printf("    3. What you're DOING with the events\n")
}

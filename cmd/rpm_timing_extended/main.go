package main

import (
	"fmt"
	"time"

	"github.com/codemartial/loadgen"
)

func timeSingleRPM(rpm int, durationS int) (time.Duration, int64) {
	specs := []loadgen.LoadSpec{
		{
			RPM:          rpm,
			ErrorRate:    0.0005,
			DurationS:    durationS,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1500,
		},
	}

	gen := loadgen.NewLoadGenerator(specs)

	start := time.Now()
	eventCount := int64(0)

	for event := gen.Next(); event != nil; event = gen.Next() {
		eventCount++
		// Print progress every 10M events
		if eventCount%10000000 == 0 {
			elapsed := time.Since(start)
			fmt.Printf("  ... %dM events in %s (%.0f events/sec)\n",
				eventCount/1000000, elapsed.Round(time.Millisecond),
				float64(eventCount)/elapsed.Seconds())
		}
	}

	elapsed := time.Since(start)

	return elapsed, eventCount
}

func main() {
	fmt.Println("Extended RPM Timing Analysis (including 600k)")
	fmt.Println("==============================================")
	fmt.Printf("%-10s %-10s %-15s %-15s %-15s %-20s\n",
		"RPM", "Duration", "Events", "Runtime", "Events/sec", "ns/event")
	fmt.Println("------------------------------------------------------------------------------------")

	// Test higher RPM values
	rpmValues := []int{
		300000,
		400000,
		500000,
		600000,
	}

	durationS := 10 // 10 seconds of simulated time

	var prevElapsed time.Duration
	var prevEvents int64

	for i, rpm := range rpmValues {
		fmt.Printf("Testing RPM=%d...\n", rpm)
		elapsed, eventCount := timeSingleRPM(rpm, durationS)
		eventsPerSec := float64(eventCount) / elapsed.Seconds()
		nsPerEvent := float64(elapsed.Nanoseconds()) / float64(eventCount)

		speedup := ""
		if i > 0 && prevEvents > 0 {
			// Calculate the ratio of time increase vs event increase
			timeRatio := float64(elapsed) / float64(prevElapsed)
			eventRatio := float64(eventCount) / float64(prevEvents)
			complexity := timeRatio / eventRatio
			speedup = fmt.Sprintf("(%.2fx slower per event)", complexity)
		}

		fmt.Printf("%-10d %-10d %-15d %-15s %-15.0f %-20.2f %s\n",
			rpm, durationS, eventCount, elapsed.Round(time.Millisecond),
			eventsPerSec, nsPerEvent, speedup)

		prevElapsed = elapsed
		prevEvents = eventCount
	}

	fmt.Println("\nNote: If 600k RPM takes exponentially longer, there's likely an O(N) operation somewhere")
}

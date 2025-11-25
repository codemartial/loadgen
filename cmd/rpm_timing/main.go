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
	}

	elapsed := time.Since(start)

	return elapsed, eventCount
}

func main() {
	fmt.Println("RPM Timing Analysis")
	fmt.Println("===================")
	fmt.Printf("%-10s %-10s %-15s %-15s %-15s %-20s\n",
		"RPM", "Duration", "Events", "Runtime", "Events/sec", "ns/event")
	fmt.Println("------------------------------------------------------------------------------------")

	// Test different RPM values
	rpmValues := []int{
		10000,
		20000,
		50000,
		100000,
		150000,
		200000,
		250000,
		300000,
		350000,
		400000,
		450000,
		500000,
	}

	durationS := 10 // 10 seconds of simulated time

	var prevElapsed time.Duration
	var prevEvents int64

	for i, rpm := range rpmValues {
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

	fmt.Println("\nAnalyzing complexity:")
	fmt.Println("If O(1): runtime should scale linearly with events")
	fmt.Println("If O(log N): ns/event should grow slowly")
	fmt.Println("If O(N): runtime should scale quadratically with events")
}

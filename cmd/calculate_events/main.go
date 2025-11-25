package main

import (
	"fmt"
)

func main() {
	fmt.Println("Event Count Calculation for Weekly Traffic Benchmark")
	fmt.Println("=====================================================\n")

	// Each day has 3 phases (low, mid, high), each lasting 8 hours (28800 seconds)
	specs := []struct {
		name     string
		rpm      int
		duration int
	}{
		// Monday
		{"Mon Low", 9375000, 28800},
		{"Mon Mid", 18750000, 28800},
		{"Mon High", 28125000, 28800},
		// Tuesday
		{"Tue Low", 11250000, 28800},
		{"Tue Mid", 22500000, 28800},
		{"Tue High", 33750000, 28800},
		// Wednesday
		{"Wed Low", 13125000, 28800},
		{"Wed Mid", 26250000, 28800},
		{"Wed High", 39375000, 28800},
		// Thursday
		{"Thu Low", 15000000, 28800},
		{"Thu Mid", 30000000, 28800},
		{"Thu High", 45000000, 28800},
		// Friday
		{"Fri Low", 16875000, 28800},
		{"Fri Mid", 33750000, 28800},
		{"Fri High", 50625000, 28800},
		// Saturday
		{"Sat Low", 18750000, 28800},
		{"Sat Mid", 37500000, 28800},
		{"Sat High", 56250000, 28800},
		// Sunday
		{"Sun Low", 20625000, 28800},
		{"Sun Mid", 41250000, 28800},
		{"Sun High", 61875000, 28800},
	}

	totalEvents := int64(0)
	totalDuration := 0

	fmt.Printf("%-12s %12s %12s %20s %20s\n", "Phase", "RPM", "Duration(s)", "Events", "Cumulative Events")
	fmt.Println("------------------------------------------------------------------------------------")

	for _, spec := range specs {
		// RPM * minutes
		events := int64(spec.rpm) * int64(spec.duration) / 60
		totalEvents += events
		totalDuration += spec.duration

		fmt.Printf("%-12s %12d %12d %20d %20d\n",
			spec.name, spec.rpm, spec.duration, events, totalEvents)
	}

	fmt.Println("------------------------------------------------------------------------------------")
	fmt.Printf("Total Duration: %d seconds (%.1f hours, %.1f days)\n",
		totalDuration, float64(totalDuration)/3600, float64(totalDuration)/86400)
	fmt.Printf("Total Events: %d (%.2e)\n", totalEvents, float64(totalEvents))
	fmt.Printf("Average RPM: %.0f\n", float64(totalEvents)*60/float64(totalDuration))

	// Estimate memory usage at 48 bytes per event
	bytesPerEvent := 48.0
	totalMemory := float64(totalEvents) * bytesPerEvent
	fmt.Printf("\nEstimated Memory (at 48B/event): %.2f GB\n", totalMemory/1e9)

	// Estimate runtime at different throughputs
	fmt.Printf("\nEstimated Runtime:\n")
	throughputs := []float64{1e6, 5e6, 7e6, 10e6}
	for _, tp := range throughputs {
		runtime := float64(totalEvents) / tp
		fmt.Printf("  At %.1fM events/sec: %.1f seconds (%.1f minutes)\n",
			tp/1e6, runtime, runtime/60)
	}

	// Check what causes exponential growth
	fmt.Printf("\nWhy might this be slow?\n")
	fmt.Printf("The issue is likely NOT O(N) complexity in LoadGenerator.\n")
	fmt.Printf("With %.2e events, even O(1) takes a long time!\n", float64(totalEvents))
	fmt.Printf("\nAt 7M events/sec (measured), this should take %.1f minutes\n",
		float64(totalEvents)/7e6/60)

	// Check for potential O(N) behavior in the test itself
	fmt.Printf("\nPotential issues:\n")
	fmt.Printf("1. Memory allocations: %.2f GB needed\n", totalMemory/1e9)
	fmt.Printf("2. GC overhead with billions of allocations\n")
	fmt.Printf("3. If measuring per-event metrics, the measurement itself could be O(N)\n")
	fmt.Printf("4. runtime.ReadMemStats() can be slow with high allocation rates\n")

	// Calculate approximate number of events for each RPM level
	fmt.Printf("\nEvent count growth by RPM:\n")
	testRPMs := []int{100000, 200000, 300000, 400000, 500000, 600000}
	for _, rpm := range testRPMs {
		// Use exponential distribution to estimate actual events
		// Average inter-arrival time for Poisson process = 1/lambda
		// For RPM, lambda = RPM/60 per second
		// Expected events in T seconds = lambda * T
		expectedEvents := float64(rpm) * 10.0 / 60.0
		// But due to exponential distribution, actual varies
		// For large numbers, it approaches expectedEvents

		// The actual events we measured show significant growth
		// Let's see if there's a pattern
		fmt.Printf("  %6d RPM, 10s → ~%.0f expected events\n", rpm, expectedEvents)
	}

	// Compare with actual measurements
	fmt.Printf("\nActual measured events (from our tests):\n")
	actualData := []struct {
		rpm    int
		events int64
	}{
		{300000, 1468218},
		{400000, 7878259},
		{500000, 41767789},
		{600000, 221637201},
	}

	for _, d := range actualData {
		expected := float64(d.rpm) * 10.0 / 60.0
		ratio := float64(d.events) / expected
		fmt.Printf("  %6d RPM → %10d events (%.0fx expected)\n", d.rpm, d.events, ratio)
	}

	fmt.Printf("\n🚨 WAIT - this is the problem! Events are growing exponentially!\n")
	fmt.Printf("Expected events for 600k RPM in 10s: %.0f\n", 600000.0*10/60)
	fmt.Printf("Actual events: %d (%.0fx more than expected!)\n",
		actualData[len(actualData)-1].events,
		float64(actualData[len(actualData)-1].events)/(600000.0*10/60))
}

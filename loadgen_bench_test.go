package loadgen

import (
	"runtime"
	"testing"
)

// BenchmarkWeeklyTraffic simulates a full week of business website traffic
// with realistic daily and hourly patterns
func BenchmarkWeeklyTraffic(b *testing.B) {
	specs := []LoadSpec{
		// Monday (Day 1)
		{RPM: 9375, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1500},  // Low
		{RPM: 18750, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 150, P99LatencyMS: 600, TimeoutMS: 1500}, // Mid
		{RPM: 28125, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 200, P99LatencyMS: 700, TimeoutMS: 1500}, // High

		// Tuesday (Day 2, +20% of D1)
		{RPM: 11250, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1500}, // Low
		{RPM: 22500, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 150, P99LatencyMS: 600, TimeoutMS: 1500}, // Mid
		{RPM: 33750, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 200, P99LatencyMS: 700, TimeoutMS: 1500}, // High

		// Wednesday (Day 3, +40% of D1)
		{RPM: 13125, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1500}, // Low
		{RPM: 26250, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 150, P99LatencyMS: 600, TimeoutMS: 1500}, // Mid
		{RPM: 39375, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 200, P99LatencyMS: 700, TimeoutMS: 1500}, // High

		// Thursday (Day 4, +60% of D1)
		{RPM: 15000, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1500}, // Low
		{RPM: 30000, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 150, P99LatencyMS: 600, TimeoutMS: 1500}, // Mid
		{RPM: 45000, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 200, P99LatencyMS: 700, TimeoutMS: 1500}, // High

		// Friday (Day 5, +80% of D1)
		{RPM: 16875, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1500}, // Low
		{RPM: 33750, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 150, P99LatencyMS: 600, TimeoutMS: 1500}, // Mid
		{RPM: 50625, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 200, P99LatencyMS: 700, TimeoutMS: 1500}, // High

		// Saturday (Day 6, +100% of D1)
		{RPM: 18750, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 110, P99LatencyMS: 550, TimeoutMS: 1500}, // Low
		{RPM: 37500, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 160, P99LatencyMS: 650, TimeoutMS: 1500}, // Mid
		{RPM: 56250, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 210, P99LatencyMS: 750, TimeoutMS: 1500}, // High

		// Sunday (Day 7, +120% of D1)
		{RPM: 20625, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 120, P99LatencyMS: 600, TimeoutMS: 1500}, // Low
		{RPM: 41250, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 170, P99LatencyMS: 700, TimeoutMS: 1500}, // Mid
		{RPM: 61875, ErrorRate: 0.05, DurationS: 28800, P50LatencyMS: 220, P99LatencyMS: 800, TimeoutMS: 1500}, // High
	}

	// Run once since this is a massive simulation
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	b.ResetTimer()

	load := NewLoadGenerator(specs)
	gen := NewEventStream(load)
	eventCount := int64(0)

	b.ResetTimer()
	event := gen.Next()
	//	for i := 0; i < b.N && event != nil; i++ {
	for event != nil {
		event = gen.Next()
		eventCount++
	}
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)

	// Calculate metrics
	totalAlloc := memAfter.TotalAlloc - memBefore.TotalAlloc
	totalMallocs := memAfter.Mallocs - memBefore.Mallocs
	bytesPerEvent := float64(totalAlloc) / float64(eventCount)
	allocsPerEvent := float64(totalMallocs) / float64(eventCount)

	b.ReportMetric(float64(eventCount), "events")
	b.ReportMetric(float64(eventCount)/b.Elapsed().Seconds(), "events/sec")
	b.ReportMetric(bytesPerEvent, "B/event")
	b.ReportMetric(allocsPerEvent, "allocs/event")
}

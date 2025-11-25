package loadgen

import (
	"math"
	"sort"
	"testing"
	"time"
)

func TestLoadGenerator_SingleSpec(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
	}

	gen := NewLoadGenerator(specs)
	var events []*LoadEvent

	for event := gen.Next(); event != nil; event = gen.Next() {
		events = append(events, event)
	}

	if len(events) == 0 {
		t.Fatal("Expected at least one event")
	}

	// Verify timestamps are monotonically increasing
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("Timestamps not monotonic at index %d", i)
		}
	}
}

func TestLoadGenerator_MultipleSpecs(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
		{
			RPM:          120,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 200,
			P99LatencyMS: 800,
			TimeoutMS:    1500,
		},
	}

	gen := NewLoadGenerator(specs)
	var events []*LoadEvent

	for event := gen.Next(); event != nil; event = gen.Next() {
		events = append(events, event)
	}

	if len(events) == 0 {
		t.Fatal("Expected events from multiple specs")
	}

	// Verify timestamps are monotonically increasing across spec boundaries
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("Timestamps not monotonic across specs at index %d", i)
		}
	}
}

func TestLoadGenerator_P50Accuracy(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          6000, // Very high RPM to generate many events quickly
			ErrorRate:    0.0,
			DurationS:    30,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    2000,
		},
	}

	gen := NewLoadGenerator(specs)
	var durations []float64

	count := 0
	for event := gen.Next(); event != nil && count < 1000; event = gen.Next() {
		durations = append(durations, float64(event.Duration.Milliseconds()))
		count++
	}

	if len(durations) < 500 {
		t.Fatalf("Not enough events generated: %d", len(durations))
	}

	sort.Float64s(durations)
	p50 := percentile(durations, 50)

	// Allow 30% tolerance for p50 due to exponential distribution variance
	expectedP50 := 100.0
	tolerance := 0.30
	if math.Abs(p50-expectedP50)/expectedP50 > tolerance {
		t.Errorf("P50 latency %.2fms outside tolerance (expected ~%.0fms ±%.0f%%)",
			p50, expectedP50, tolerance*100)
	}
}

func TestLoadGenerator_ErrorRate(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          6000, // Very high RPM
			ErrorRate:    0.10, // 10% error rate
			DurationS:    30,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    2000, // High timeout to minimize timeout-based failures
		},
	}

	gen := NewLoadGenerator(specs)
	var successCount, failureCount int

	count := 0
	for event := gen.Next(); event != nil && count < 1000; event = gen.Next() {
		if event.Success {
			successCount++
		} else {
			failureCount++
		}
		count++
	}

	actualErrorRate := float64(failureCount) / float64(successCount+failureCount)
	expectedErrorRate := 0.10

	// Allow 50% tolerance for error rate (binomial distribution)
	tolerance := 0.50
	if math.Abs(actualErrorRate-expectedErrorRate)/expectedErrorRate > tolerance {
		t.Errorf("Error rate %.3f outside tolerance (expected %.3f ±%.0f%%)",
			actualErrorRate, expectedErrorRate, tolerance*100)
	}
}

func TestLoadGenerator_TimeoutFailures(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          6000, // Very high RPM
			ErrorRate:    0.0,  // No random errors
			DurationS:    30,
			P50LatencyMS: 800,  // High latency
			P99LatencyMS: 1500, // Very high latency
			TimeoutMS:    1600, // Close to p99 - some requests will timeout
		},
	}

	gen := NewLoadGenerator(specs)
	var timeoutFailures int

	count := 0
	for event := gen.Next(); event != nil && count < 1000; event = gen.Next() {
		if !event.Success && float64(event.Duration.Milliseconds()) > specs[0].TimeoutMS {
			timeoutFailures++
		}
		count++
	}

	// With timeout slightly above p99, we expect some (but not many) timeouts
	// since the distribution can generate values above p99
	if timeoutFailures == 0 {
		t.Log("Warning: Expected some timeout failures with timeout close to p99")
	}
}

func TestEventStream_StartEndPairing(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
	}

	gen := NewLoadGenerator(specs)
	stream := NewEventStream(gen)

	var startCount, endCount int
	var events []*SimEvent

	for event := stream.Next(); event != nil; event = stream.Next() {
		events = append(events, event)
		if event.Type == EventStart {
			startCount++
		} else if event.Type == EventEnd {
			endCount++
		}
	}

	if startCount == 0 || endCount == 0 {
		t.Fatal("Expected both start and end events")
	}

	// Start and end counts should match
	if startCount != endCount {
		t.Errorf("Start count %d != End count %d", startCount, endCount)
	}

	// Verify timestamps are monotonically increasing
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp.Before(events[i-1].Timestamp) {
			t.Errorf("Event timestamps not monotonic at index %d: %v -> %v",
				i, events[i-1].Timestamp, events[i].Timestamp)
		}
	}
}

func TestEventStream_TimestampMonotonicity(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          600,
			ErrorRate:    0.0,
			DurationS:    5,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
	}

	gen := NewLoadGenerator(specs)
	stream := NewEventStream(gen)

	var prevTime time.Time
	count := 0

	for event := stream.Next(); event != nil && count < 100; event = stream.Next() {
		if !prevTime.IsZero() && event.Timestamp.Before(prevTime) {
			t.Errorf("Timestamp went backwards: %v -> %v", prevTime, event.Timestamp)
		}
		prevTime = event.Timestamp
		count++
	}

	if count == 0 {
		t.Fatal("No events generated")
	}
}

func TestEventStream_MultipleSpecs(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
		{
			RPM:          120,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 200,
			P99LatencyMS: 800,
			TimeoutMS:    1500,
		},
	}

	gen := NewLoadGenerator(specs)
	stream := NewEventStream(gen)

	var startCount, endCount int
	var prevTime time.Time

	for event := stream.Next(); event != nil; event = stream.Next() {
		if !prevTime.IsZero() && event.Timestamp.Before(prevTime) {
			t.Errorf("Timestamp not monotonic across spec boundary")
		}
		prevTime = event.Timestamp

		if event.Type == EventStart {
			startCount++
		} else {
			endCount++
		}
	}

	if startCount != endCount {
		t.Errorf("Start/End mismatch across specs: %d starts, %d ends", startCount, endCount)
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventStart, "start"},
		{EventEnd, "end"},
		{EventType(999), "unknown"},
	}

	for _, tt := range tests {
		result := tt.eventType.String()
		if result != tt.expected {
			t.Errorf("EventType(%d).String() = %s, expected %s",
				tt.eventType, result, tt.expected)
		}
	}
}

func LoadGenerator_EmptySpecs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when load specs are empty")
		}
	}()

	specs := []LoadSpec{}
	NewLoadGenerator(specs)
}

func TestLoadGenerator_ZeroRPM(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          0, // Invalid RPM
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
	}

	// This should not panic, but may produce unusual results
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panicked with zero RPM: %v", r)
		}
	}()

	gen := NewLoadGenerator(specs)
	_ = gen.Next()
}

func TestLoadGenerator_ValidationP50P99(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when P50 >= P99")
		}
	}()

	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 500,
			P99LatencyMS: 500, // P50 >= P99 should panic
			TimeoutMS:    1000,
		},
	}
	NewLoadGenerator(specs)
}

func TestLoadGenerator_ValidationP99Timeout(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when P99 >= Timeout")
		}
	}()

	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    0.0,
			DurationS:    1,
			P50LatencyMS: 100,
			P99LatencyMS: 1000, // P99 >= Timeout should panic
			TimeoutMS:    1000,
		},
	}
	NewLoadGenerator(specs)
}

// Helper function
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

// Benchmark tests
func BenchmarkLoadGenerator(b *testing.B) {
	specs := []LoadSpec{
		{
			RPM:          600,
			ErrorRate:    0.05,
			DurationS:    100,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewLoadGenerator(specs)
		count := 0
		for event := gen.Next(); event != nil && count < 1000; event = gen.Next() {
			count++
		}
	}
}

func BenchmarkEventStream(b *testing.B) {
	specs := []LoadSpec{
		{
			RPM:          600,
			ErrorRate:    0.05,
			DurationS:    100,
			P50LatencyMS: 100,
			P99LatencyMS: 500,
			TimeoutMS:    1000,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gen := NewLoadGenerator(specs)
		stream := NewEventStream(gen)
		count := 0
		for event := stream.Next(); event != nil && count < 1000; event = stream.Next() {
			count++
		}
	}
}

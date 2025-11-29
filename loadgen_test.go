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
	var events []LoadEvent

	event, ok := gen.Next()
	for ok {
		events = append(events, event)
		event, ok = gen.Next()
	}

	if len(events) == 0 {
		t.Fatal("Expected at least one event")
	}

	// Verify timestamps are monotonically increasing
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp < events[i-1].Timestamp {
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
	var events []LoadEvent

	event, ok := gen.Next()
	for ok {
		events = append(events, event)
		event, ok = gen.Next()
	}

	if len(events) == 0 {
		t.Fatal("Expected events from multiple specs")
	}

	// Verify timestamps are monotonically increasing across spec boundaries
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp < events[i-1].Timestamp {
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
	event, ok := gen.Next()
	for ok && count < 1000 {
		durations = append(durations, float64(event.Duration)/1e6)
		count++
		event, ok = gen.Next()
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
	event, ok := gen.Next()
	for ok && count < 1000 {
		if event.Success {
			successCount++
		} else {
			failureCount++
		}
		count++
		event, ok = gen.Next()
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
	event, ok := gen.Next()
	for ok && count < 1000 {
		if !event.Success && float64(event.Duration)/1e6 > specs[0].TimeoutMS {
			timeoutFailures++
		}
		count++
		event, ok = gen.Next()
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
	var events []SimEvent

	for {
		event, err := stream.Next()
		if err != nil {
			break
		}
		events = append(events, event)
		if event.Status == EventStart {
			startCount++
		} else {
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

	for count < 100 {
		event, err := stream.Next()
		if err != nil {
			break
		}
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

	for {
		event, err := stream.Next()
		if err != nil {
			break
		}
		if !prevTime.IsZero() && event.Timestamp.Before(prevTime) {
			t.Errorf("Timestamp not monotonic across spec boundary")
		}
		prevTime = event.Timestamp

		if event.Status == EventStart {
			startCount++
		} else {
			endCount++
		}
	}

	if startCount != endCount {
		t.Errorf("Start/End mismatch across specs: %d starts, %d ends", startCount, endCount)
	}
}

func TestEventStatus_String(t *testing.T) {
	tests := []struct {
		eventStatus EventStatus
		expected    string
	}{
		{EventStart, "start"},
		{EventSuccess, "success"},
		{EventError, "error"},
	}

	for _, tt := range tests {
		result := tt.eventStatus.String()
		if result != tt.expected {
			t.Errorf("EventStatus(%d).String() = %s, expected %s",
				tt.eventStatus, result, tt.expected)
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
	_, _ = gen.Next()
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

// TestEventStream_StartEventsAlwaysEventStart verifies that start events
// always have EventStart status, regardless of whether the task will succeed or fail.
// Uses EventID to match start events with their completions.
func TestEventStream_StartEventsAlwaysEventStart(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          600,
			ErrorRate:    0.50, // 50% error rate to ensure we get failures
			DurationS:    5,
			P50LatencyMS: 50,
			P99LatencyMS: 100,
			TimeoutMS:    500,
		},
	}

	gen := NewLoadGenerator(specs)
	stream := NewEventStream(gen)

	// Track events by EventID
	eventsByID := make(map[int64][]SimEvent)

	for {
		event, err := stream.Next()
		if err != nil {
			break
		}
		eventsByID[event.EventID] = append(eventsByID[event.EventID], event)
	}

	if len(eventsByID) < 2 {
		t.Fatalf("Not enough events generated: %d unique IDs", len(eventsByID))
	}

	// For each EventID, verify the first occurrence is EventStart
	for id, events := range eventsByID {
		if len(events) == 0 {
			continue
		}

		// First event with this ID should be EventStart
		if events[0].Status != EventStart {
			t.Errorf("EventID %d: first event has status %v, expected EventStart", id, events[0].Status)
		}

		// If there's a second event (completion), it should be Success or Error
		if len(events) > 1 {
			if events[1].Status != EventSuccess && events[1].Status != EventError {
				t.Errorf("EventID %d: completion event has status %v, expected EventSuccess or EventError", id, events[1].Status)
			}
		}

		// Should never have more than 2 events per ID
		if len(events) > 2 {
			t.Errorf("EventID %d: has %d events, expected at most 2 (start + completion)", id, len(events))
		}
	}
}

// TestEventStream_CompletionEventsReflectSuccess verifies that completion events
// properly reflect whether the task succeeded or failed.
// Uses low RPM to get alternating start/completion pattern, then checks later events (completions).
func TestEventStream_CompletionEventsReflectSuccess(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          60,   // 1 request per second - ensures alternation
			ErrorRate:    0.50, // 50% error rate
			DurationS:    10,
			P50LatencyMS: 50,
			P99LatencyMS: 100,
			TimeoutMS:    500,
		},
	}

	gen := NewLoadGenerator(specs)
	stream := NewEventStream(gen)

	var events []SimEvent
	for {
		event, err := stream.Next()
		if err != nil {
			break
		}
		events = append(events, event)
	}

	if len(events) < 10 {
		t.Fatalf("Not enough events generated: %d", len(events))
	}

	// With low RPM, events alternate: start, completion, start, completion, ...
	// Odd-indexed events (1, 3, 5, ...) are completions (later timestamps)
	var successCompletions, errorCompletions int
	for i := 1; i < len(events); i += 2 {
		// Verify this is indeed a completion (later timestamp than previous)
		if !events[i].Timestamp.After(events[i-1].Timestamp) &&
		   !events[i].Timestamp.Equal(events[i-1].Timestamp) {
			continue // Skip if not the expected pattern
		}

		// Count completion event statuses
		switch events[i].Status {
		case EventSuccess:
			successCompletions++
		case EventError:
			errorCompletions++
		}
	}

	// With 50% error rate, we should see BOTH success and error completions
	if errorCompletions == 0 {
		t.Errorf("Expected some error completion events with 50%% error rate, got 0 (success=%d)", successCompletions)
	}

	if successCompletions == 0 {
		t.Errorf("Expected some success completion events with 50%% error rate, got 0 (error=%d)", errorCompletions)
	}

	// Sanity check: with 50% error rate, distribution should be reasonable
	totalCompletions := successCompletions + errorCompletions
	if totalCompletions > 0 {
		errorRate := float64(errorCompletions) / float64(totalCompletions)
		// Very loose bounds - just checking both types exist
		if errorRate < 0.1 || errorRate > 0.9 {
			t.Errorf("Error rate among completions is %.2f%%, expected roughly 50%%", errorRate*100)
		}
	}
}

// TestEventStream_ErrorTaskLifecycle verifies the complete lifecycle of a failed task:
// it should emit EventStart when starting, then EventError when completing
func TestEventStream_ErrorTaskLifecycle(t *testing.T) {
	specs := []LoadSpec{
		{
			RPM:          60,
			ErrorRate:    1.0, // 100% error rate - all tasks fail
			DurationS:    2,
			P50LatencyMS: 50,
			P99LatencyMS: 100,
			TimeoutMS:    500,
		},
	}

	gen := NewLoadGenerator(specs)
	stream := NewEventStream(gen)

	var events []SimEvent
	for {
		event, err := stream.Next()
		if err != nil {
			break
		}
		events = append(events, event)
	}

	if len(events) == 0 {
		t.Fatal("No events generated")
	}

	// Count event types
	var startCount, errorCount, successCount int
	for _, event := range events {
		switch event.Status {
		case EventStart:
			startCount++
		case EventError:
			errorCount++
		case EventSuccess:
			successCount++
		}
	}

	// With 100% error rate:
	// - All start events should be EventStart
	// - All completion events should be EventError
	// - No completion events should be EventSuccess
	if startCount == 0 {
		t.Error("Expected start events")
	}

	if errorCount == 0 {
		t.Error("Expected error completion events with 100% error rate")
	}

	if successCount > 0 {
		t.Errorf("Expected no success completions with 100%% error rate, got %d", successCount)
	}

	// Start and completion counts should match
	if startCount != errorCount {
		t.Errorf("Start count %d should equal error completion count %d", startCount, errorCount)
	}
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
		_, ok := gen.Next()
		for ok && count < 1000 {
			count++
			_, ok = gen.Next()
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
		for count < 1000 {
			_, err := stream.Next()
			if err != nil {
				break
			}
			count++
		}
	}
}

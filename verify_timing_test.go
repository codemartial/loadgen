package loadgen

import (
	"fmt"
	"testing"
	"time"
)

func TestTimingEndsAtNow(t *testing.T) {
	specs := []LoadSpec{
		{RPM: 60000, ErrorRate: 0.0, DurationS: 5, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1500},
		{RPM: 120000, ErrorRate: 0.0, DurationS: 3, P50LatencyMS: 150, P99LatencyMS: 600, TimeoutMS: 1500},
	}

	creationTime := time.Now()
	gen := NewLoadGenerator(specs)

	var firstEvent, lastEvent *LoadEvent
	count := 0

	for event := gen.Next(); event != nil; event = gen.Next() {
		if count == 0 {
			firstEvent = event
		}
		lastEvent = event
		count++
	}

	fmt.Printf("Creation time: %s\n", creationTime)
	fmt.Printf("First event:   %s\n", firstEvent.Timestamp)
	fmt.Printf("Last event:    %s\n", lastEvent.Timestamp)
	fmt.Printf("Expected end:  ~%s (creation time)\n", creationTime)

	totalDuration := 5 + 3 // 8 seconds
	expectedStart := creationTime.Add(-time.Duration(totalDuration) * time.Second)

	fmt.Printf("\nExpected start: %s\n", expectedStart)
	fmt.Printf("Actual start:   %s\n", firstEvent.Timestamp)
	fmt.Printf("Difference:     %s\n", firstEvent.Timestamp.Sub(expectedStart))

	fmt.Printf("\nLast event should be near creation time\n")
	fmt.Printf("Time from last event to creation: %s\n", creationTime.Sub(lastEvent.Timestamp))

	// Last event should be within 1 second of creation time (can be slightly after due to random intervals)
	timeDiff := creationTime.Sub(lastEvent.Timestamp).Abs()
	if timeDiff > time.Second {
		t.Errorf("Last event should end near creation time, got %s difference", timeDiff)
	}

	// First event should start approximately totalDuration before creation time
	startDiff := firstEvent.Timestamp.Sub(expectedStart).Abs()
	if startDiff > 100*time.Millisecond {
		t.Errorf("First event should start ~%d seconds before creation time, got %s difference", totalDuration, startDiff)
	}
}

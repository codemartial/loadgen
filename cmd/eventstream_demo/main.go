package main

import (
	"fmt"

	"github.com/codemartial/loadgen"
)

func main() {
	// Define load specs
	specs := []loadgen.LoadSpec{
		{
			RPM:          600,  // 10 requests per second
			ErrorRate:    0.05, // 5% error rate
			DurationS:    5,    // Run for 5 seconds
			P50LatencyMS: 100,  // 100ms p50
			P99LatencyMS: 500,  // 500ms p99
			TimeoutMS:    1000, // 1s timeout
		},
	}

	gen := loadgen.NewLoadGenerator(specs)
	stream := loadgen.NewEventStream(gen)

	var startCount, endCount int
	eventNum := 0

	fmt.Println("=== EventStream Demo: Start/End Events ===\n")
	fmt.Println("First 30 events:")
	fmt.Printf("%-5s %-10s %-15s %-10s\n", "No.", "Type", "Timestamp", "Success")
	fmt.Println("--------------------------------------------------------")

	for event := stream.Next(); event != nil; event = stream.Next() {
		eventType := "START"
		if event.Type == loadgen.EventEnd {
			eventType = "END"
			endCount++
		} else {
			startCount++
		}

		if eventNum < 30 {
			fmt.Printf("%-5d %-10s %s  %-10v\n",
				eventNum+1,
				eventType,
				event.Timestamp.Format("15:04:05.000"),
				event.Success,
			)
		}
		eventNum++
	}

	fmt.Println("\n=== Summary ===")
	fmt.Printf("Total events: %d\n", eventNum)
	fmt.Printf("Start events: %d\n", startCount)
	fmt.Printf("End events: %d\n", endCount)
	fmt.Printf("Start/End balanced: %v\n", startCount == endCount)
}

package loadgen

import (
	"fmt"
	"testing"
)

func TestEventCount(t *testing.T) {
	testCases := []struct {
		rpm      int
		duration int
	}{
		{300000, 10},
		{400000, 10},
		{500000, 10},
		{600000, 10},
	}

	for _, tc := range testCases {
		specs := []LoadSpec{
			{
				RPM:          tc.rpm,
				ErrorRate:    0.0,
				DurationS:    tc.duration,
				P50LatencyMS: 100,
				P99LatencyMS: 500,
				TimeoutMS:    1500,
			},
		}

		gen := NewLoadGenerator(specs)
		count := 0

		for event := gen.Next(); event != nil; event = gen.Next() {
			count++
		}

		expected := tc.rpm * tc.duration / 60
		ratio := float64(count) / float64(expected)

		fmt.Printf("RPM=%d, Duration=%ds: expected=%d, actual=%d, ratio=%.2fx\n",
			tc.rpm, tc.duration, expected, count, ratio)

		if ratio < 0.95 || ratio > 1.05 {
			t.Errorf("Event count out of range: expected ~%d, got %d (%.2fx)",
				expected, count, ratio)
		}
	}
}

package main

import (
	"fmt"
	"math"
	"sort"

	"github.com/codemartial/loadgen"
)

func main() {
	p50Target := 100.0
	p99Target := 500.0

	specs := []loadgen.LoadSpec{
		{
			RPM:          600000, // 10000 per second
			ErrorRate:    0.0,
			DurationS:    10, // Run for 10 seconds
			P50LatencyMS: p50Target,
			P99LatencyMS: p99Target,
			TimeoutMS:    10000,
		},
	}

	gen := loadgen.NewLoadGenerator(specs)

	// Collect samples
	samples := make([]float64, 0, 100000)
	for {
		event := gen.Next()
		if event == nil {
			break
		}
		samples = append(samples, float64(event.Duration.Microseconds())/1000.0)
	}

	sort.Float64s(samples)

	fmt.Printf("Collected %d samples\n", len(samples))
	p01 := samples[int(0.01*float64(len(samples)))]
	p05 := samples[int(0.05*float64(len(samples)))]
	p10 := samples[int(0.10*float64(len(samples)))]
	p25 := samples[int(0.25*float64(len(samples)))]
	p50 := samples[int(0.50*float64(len(samples)))]
	p75 := samples[int(0.75*float64(len(samples)))]
	p90 := samples[int(0.90*float64(len(samples)))]
	p95 := samples[int(0.95*float64(len(samples)))]
	p99 := samples[int(0.99*float64(len(samples)))]
	p999 := samples[int(0.999*float64(len(samples)))]
	min := samples[0]
	max := samples[len(samples)-1]

	fmt.Printf("\nTarget: p50=%.0f, p99=%.0f\n", p50Target, p99Target)
	fmt.Printf("\nPercentiles:\n")
	fmt.Printf("  p01  = %8.2f\n", p01)
	fmt.Printf("  p05  = %8.2f\n", p05)
	fmt.Printf("  p10  = %8.2f\n", p10)
	fmt.Printf("  p25  = %8.2f\n", p25)
	fmt.Printf("  p50  = %8.2f (target: %.0f, error: %.2f%%)\n", p50, p50Target, math.Abs(p50-p50Target)/p50Target*100)
	fmt.Printf("  p75  = %8.2f\n", p75)
	fmt.Printf("  p90  = %8.2f\n", p90)
	fmt.Printf("  p95  = %8.2f\n", p95)
	fmt.Printf("  p99  = %8.2f (target: %.0f, error: %.2f%%)\n", p99, p99Target, math.Abs(p99-p99Target)/p99Target*100)
	fmt.Printf("  p999 = %8.2f\n", p999)
	fmt.Printf("  min  = %8.2f\n", min)
	fmt.Printf("  max  = %8.2f\n", max)
}

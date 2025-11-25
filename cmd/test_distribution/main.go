package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

func generateDuration(x, p50, p99 float64) float64 {
	k := math.Log(2*p99/p50-1) / math.Log(0.99/0.5)
	return (p50 / 2) * (1 + math.Pow(x/0.5, k))
}

func main() {
	p50Target := 100.0
	p99Target := 500.0

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate 10000 samples
	samples := make([]float64, 10000)
	for i := 0; i < 10000; i++ {
		x := rng.Float64()
		samples[i] = generateDuration(x, p50Target, p99Target)
	}

	sort.Float64s(samples)

	p50 := samples[int(0.50*float64(len(samples)))]
	p95 := samples[int(0.95*float64(len(samples)))]
	p99 := samples[int(0.99*float64(len(samples)))]
	min := samples[0]
	max := samples[len(samples)-1]

	fmt.Printf("Target: p50=%.0f, p99=%.0f\n", p50Target, p99Target)
	fmt.Printf("Actual: p50=%.2f, p95=%.2f, p99=%.2f\n", p50, p95, p99)
	fmt.Printf("Range: min=%.2f, max=%.2f\n", min, max)
	fmt.Printf("\nDirect calculation tests:\n")
	fmt.Printf("f(0.5) = %.2f (should be %.0f)\n", generateDuration(0.5, p50Target, p99Target), p50Target)
	fmt.Printf("f(0.99) = %.2f (should be %.0f)\n", generateDuration(0.99, p50Target, p99Target), p99Target)
}

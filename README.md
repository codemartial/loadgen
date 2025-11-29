# loadgen

A fast, deterministic synthetic load generator for Go.

## What is this?

Ever needed to test how your service handles realistic traffic patterns? Not just "hit it with a million requests" but actual business traffic - the kind with daily peaks, varying response times, and occasional errors?

loadgen generates synthetic load events with realistic statistical properties. No actual HTTP requests, no network overhead - just pure event generation that you can pipe into your test harness.

## Features

- **Statistical realism**: Rational function latency distributions with heavy tails matching real p50/p99 targets
- **Multiple load phases**: Model daily traffic patterns with different specs
- **Error injection**: Configurable error rates and timeout failures
- **Zero clock dependency**: Simulated time means tests run at CPU speed, not wall-clock speed
- **Memory efficient**: 12 bytes and 0.5 allocations per event
- **Fast**: Generates 15+ million events per second on commodity hardware

## Installation

```bash
go get github.com/codemartial/loadgen
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/codemartial/loadgen"
)

func main() {
    // Define your load pattern
    specs := []loadgen.LoadSpec{
        {
            RPM:          1000,        // requests per minute
            ErrorRate:    0.05,        // 5% error rate
            DurationS:    60,          // run for 60 seconds
            P50LatencyMS: 100,         // 50th percentile latency
            P99LatencyMS: 500,         // 99th percentile latency
            TimeoutMS:    1000,        // timeout threshold
        },
    }

    // Create generator and event stream
    gen := loadgen.NewLoadGenerator(specs)
    stream := loadgen.NewEventStream(gen)

    // Process events
    event, err := stream.Next()
    for err == nil {
        switch event.Status {
        case loadgen.EventStart:
            fmt.Printf("Request started at %v\n", event.Timestamp)
        case loadgen.EventSuccess:
            fmt.Printf("Request completed at %v\n", event.Timestamp)
        case loadgen.EventError:
            fmt.Printf("Request failed at %v\n", event.Timestamp)
        }
        event, err = stream.Next()
    }
}
```

## How it works

### Load Generator

The `LoadGenerator` creates individual load events using:
- **Exponential distribution** for inter-arrival times (Poisson arrival process - realistic request spacing)
- **Piecewise rational function** for response durations:
  - Main distribution (x ≤ 0.99): `y = a + b*x/(1-c*x)` - fits p50/2, p50, and p99 exactly
  - Heavy tail (x > 0.99): Linear interpolation from p99 to timeout - models extreme latencies
- **Configurable error injection** (random errors + timeout-based failures)

Each event emits a Unix nanosecond timestamp, response duration, and success/error status. These are raw load events - just data points saying "a request started at time T and took D nanoseconds."

### Event Stream

The `EventStream` converts those raw load events into discrete start/success/error events, simulating concurrent requests:
- Maintains a heap of active requests
- Advances simulated time efficiently (no actual sleeping)
- Yields events in chronological order
- Tracks which requests are "in flight" at any moment

This gives you the full picture of system load over time, not just individual requests.

The stream tracks event IDs, so each start event can be matched with its corresponding completion event via the `EventID` field.

## Typical Use Cases

### Benchmark testing
```go
specs := []loadgen.LoadSpec{
    {RPM: 60000, ErrorRate: 0.01, DurationS: 300,
     P50LatencyMS: 50, P99LatencyMS: 200, TimeoutMS: 500},
}
// Feed events into your service and measure actual behavior
```

### Daily traffic simulation
```go
specs := []loadgen.LoadSpec{
    {RPM: 1000, ...},   // overnight low
    {RPM: 5000, ...},   // morning ramp
    {RPM: 10000, ...},  // peak hours
    {RPM: 3000, ...},   // evening decline
}
// See how your system handles realistic daily patterns
```

### Stress testing
```go
specs := []loadgen.LoadSpec{
    {RPM: 10000, ErrorRate: 0.02, ...},   // normal load
    {RPM: 50000, ErrorRate: 0.15, ...},   // traffic spike + degradation
    {RPM: 10000, ErrorRate: 0.02, ...},   // recovery period
}
// Test system behavior during incidents
```

## Performance Characteristics

Benchmarked on a week-long traffic simulation (604M events):
- **Throughput**: 15.1M events/sec
- **Memory per event**: 12 bytes
- **Allocations per event**: 0.5
- **Total runtime**: 40 seconds for 604M events

The generator is compute-bound by basic arithmetic operations, not memory allocation.

## License

MIT. Do what you want with it.

## Credits

Written because existing load testing tools were either too slow, too complex, or required actual network operations.

Built for testing [levee](https://github.com/codemartial/levee).

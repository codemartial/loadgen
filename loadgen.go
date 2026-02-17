package loadgen

import (
	"container/heap"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

/*
  1. SimEvent - Contains timestamp, event type (start/end), and success flag
  2. timeHeap - Min-heap implementation using Go's container/heap (standard library)
  3. EventStream.Next() - Implements the exact algorithm:
    - Expires finished tasks by popping from heap
    - Advances simulated time to next interesting moment (min of next task or next expiry)
    - Yields start events for tasks at current time
    - Tracks active tasks in the heap

  Usage:
  specs := []LoadSpec{{RPM: 1000, ErrorRate: 0.05, DurationS: 60, P50LatencyMS: 100, P99LatencyMS: 500, TimeoutMS: 1000}}
  gen := NewLoadGenerator(specs)
  stream := NewEventStream(gen)

  event, err := stream.Next()
  for err == nil {
      // event.Timestamp - simulated time
      // event.Status - EventStart, EventSuccess, or EventError
      event, err = stream.Next()
  }

  The stream automatically converts durations into start/end pairs without waiting on wall clock time!
*/

// LoadSpec describes the properties of a synthetic load pattern
type LoadSpec struct {
	RPM          int     // Requests per minute
	ErrorRate    float64 // Error rate (0.0-1.0)
	DurationS    int     // Duration of this load pattern in seconds
	P50LatencyMS float64 // p50 response time in milliseconds
	P99LatencyMS float64 // p99 response time in milliseconds
	TimeoutMS    float64 // Timeout in milliseconds (durations > timeout = failure)
}

// LoadEvent represents a single load event with timestamp, duration, and success status
type LoadEvent struct {
	Timestamp int64 // Unix nanoseconds
	Duration  int64 // nanoseconds
	Success   bool
}

// LoadGenerator generates synthetic load events based on LoadSpecs
type LoadGenerator struct {
	specs            []LoadSpec
	currentSpec      int
	nextEvent        int64     // Unix nanoseconds
	specStart        int64     // Unix nanoseconds
	startTime        time.Time // Reference time for the start of the entire simulation
	rng              *rand.Rand
	a, b, c          float64 // rational function parameters: y = a + b*x/(1-c*x)
	interArrivalTime float64 // cached inter-arrival time
}

// NewLoadGenerator creates a new load generator from an array of LoadSpecs
func NewLoadGenerator(specs []LoadSpec) *LoadGenerator {
	return NewLoadGeneratorWithSeed(specs, uint64(time.Now().UnixNano()))
}

// NewLoadGeneratorWithSeed creates a new load generator with a specific RNG seed for deterministic output
func NewLoadGeneratorWithSeed(specs []LoadSpec, seed uint64) *LoadGenerator {
	if len(specs) == 0 {
		panic(fmt.Sprintf("No load specifications provided"))
	}
	// Validate specs: timeout > p99 > p50
	for i, spec := range specs {
		if spec.P50LatencyMS >= spec.P99LatencyMS {
			panic(fmt.Sprintf("spec[%d]: P50LatencyMS (%.2f) must be < P99LatencyMS (%.2f)", i, spec.P50LatencyMS, spec.P99LatencyMS))
		}
		if spec.P99LatencyMS >= spec.TimeoutMS {
			panic(fmt.Sprintf("spec[%d]: P99LatencyMS (%.2f) must be < TimeoutMS (%.2f)", i, spec.P99LatencyMS, spec.TimeoutMS))
		}
	}

	// Calculate total duration of all specs
	totalDurationNano := int64(0)
	for _, spec := range specs {
		totalDurationNano += int64(spec.DurationS) * int64(time.Second)
	}

	// Set start time so that the load ends at time.Now()
	now := time.Now()
	startTime := now.Add(-time.Duration(totalDurationNano))
	startNano := startTime.UnixNano()

	lg := &LoadGenerator{
		specs:       specs,
		currentSpec: 0,
		nextEvent:   startNano,
		specStart:   startNano,
		startTime:   startTime,
		rng:         rand.New(rand.NewPCG(seed, seed>>32)),
	}

	lg.updateDistParams(specs[0])
	return lg
}

// updateDistParams calculates rational function parameters from p50 and p99
// Solves the system:
//   y = a + b*x/(1-c*x)
//   x=0    → y = p50/2
//   x=0.5  → y = p50
//   x=0.99 → y = p99
func (lg *LoadGenerator) updateDistParams(spec LoadSpec) {
	lg.interArrivalTime = (60.0 * 1e9) / float64(spec.RPM)

	p50 := spec.P50LatencyMS
	p99 := spec.P99LatencyMS

	// From constraint 1: a = p50/2
	lg.a = p50 / 2

	// From constraint 2 and 3, solve for c:
	// c = (1.49*p50 - p99) / (0.99*(p50 - p99))
	lg.c = (1.49*p50 - p99) / (0.99 * (p50 - p99))

	// From constraint 2, solve for b:
	lg.b = p50 * (1 - 0.5*lg.c)
}

// Next returns the next load event, or false if all specs are exhausted
func (lg *LoadGenerator) Next() (LoadEvent, bool) {
	if lg.currentSpec >= len(lg.specs) {
		return LoadEvent{}, false
	}

	spec := lg.specs[lg.currentSpec]
	specDuration := int64(spec.DurationS) * int64(time.Second)

	// Check if current spec duration has elapsed
	if lg.nextEvent-lg.specStart >= specDuration {
		lg.currentSpec++
		if lg.currentSpec >= len(lg.specs) {
			return LoadEvent{}, false
		}
		lg.specStart = lg.nextEvent
		spec = lg.specs[lg.currentSpec]
		lg.updateDistParams(spec)
	}

	// Current event timestamp
	timestamp := lg.nextEvent

	// Calculate inter-arrival time based on RPM (exponential distribution)
	nextInterval := lg.rng.ExpFloat64() * lg.interArrivalTime
	lg.nextEvent += int64(nextInterval)

	// Generate response duration using piecewise function:
	// - For x <= 0.99: rational function y = a + b*x/(1-c*x)
	// - For x > 0.99: linear from P99 to timeout (heavy tail)
	x := lg.rng.Float64()
	var t_delta float64

	if x <= 0.99 {
		// Rational function for main distribution
		t_delta = lg.a + lg.b*x/(1-lg.c*x)
	} else {
		// Linear tail from P99 (at x=0.99) to timeout (at x=1.0)
		// slope = (timeout - p99) / (1.0 - 0.99) = (timeout - p99) / 0.01
		p99 := spec.P99LatencyMS
		maxValue := spec.TimeoutMS
		slope := (maxValue - p99) / 0.01
		t_delta = p99 + slope*(x-0.99)
	}

	// Determine success based on error rate
	success := lg.rng.Float64() >= spec.ErrorRate

	// Clamp successful requests below timeout (0.99 for float safety margin)
	if success {
		safeTimeout := 0.99 * spec.TimeoutMS
		if t_delta > safeTimeout {
			t_delta = safeTimeout
		}
	}

	duration := int64(t_delta * 1e6) // Convert milliseconds to nanoseconds

	return LoadEvent{
		Timestamp: timestamp,
		Duration:  duration,
		Success:   success,
	}, true
}

// EventStatus represents the status of a simulation event
type EventStatus int

const (
	EventStart EventStatus = iota
	EventSuccess
	EventError
)

// String returns the string representation of EventStatus
func (e EventStatus) String() string {
	switch e {
	case EventStart:
		return "start"
	case EventSuccess:
		return "success"
	case EventError:
		return "error"
	default:
		return "unknown"
	}
}

// SimEvent represents a discrete simulation event with timestamp and status
type SimEvent struct {
	EventID   int64
	Timestamp time.Time
	Status    EventStatus
}

type pendingEvent struct {
	seq    int64
	ts     int64
	status EventStatus
}

// eventHeap implements heap.Interface for pendingEvent (min-heap)
type eventHeap []pendingEvent

func (h eventHeap) Len() int           { return len(h) }
func (h eventHeap) Less(i, j int) bool { return h[i].ts < h[j].ts }
func (h eventHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *eventHeap) Push(x interface{}) {
	*h = append(*h, x.(pendingEvent))
}

func (h eventHeap) Peek() pendingEvent {
	return h[0]
}

// Pop removes and returns the minimum element (for heap.Interface compatibility)
func (h *eventHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// PopEvent removes and returns the minimum element without interface boxing
func (h *eventHeap) PopEvent() pendingEvent {
	old := *h
	n := len(old)
	x := old[0]
	old[0] = old[n-1]
	*h = old[0 : n-1]
	if len(*h) > 0 {
		down(h, 0, len(*h))
	}
	return x
}

// down implements the heap down operation
func down(h *eventHeap, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}
		j := j1
		if j2 := j1 + 1; j2 < n && (*h)[j2].ts < (*h)[j1].ts {
			j = j2
		}
		if (*h)[i].ts <= (*h)[j].ts {
			break
		}
		(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
		i = j
	}
	return i > i0
}

// EventStream converts LoadEvents into discrete start/end SimEvents
type EventStream struct {
	gen         *LoadGenerator
	currentTime int64 // Unix nanoseconds
	activeTasks *eventHeap
	peekedEvent LoadEvent
	hasPeeked   bool
	initialized bool
	eventCount  int64
}

// NewEventStream creates a new event stream from a load generator
func NewEventStream(gen *LoadGenerator) *EventStream {
	// Calculate max heap size: max concurrent requests
	// = max(DurationS * TimeoutMS) for each spec
	maxConcurrent := 0
	for _, spec := range gen.specs {
		// Max requests in flight = RPM * (TimeoutMS / 60000)
		concurrent := int(float64(spec.RPM) * (spec.TimeoutMS / 60000.0))
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
	}

	// Preallocate heap with calculated capacity
	h := make(eventHeap, 0, maxConcurrent)
	heap.Init(&h)

	return &EventStream{
		gen:         gen,
		activeTasks: &h,
		initialized: false,
	}
}

// Next returns the next simulation event (start or end), or nil if done
func (es *EventStream) Next() (SimEvent, error) {
	if !es.initialized {
		es.currentTime = 0
		es.initialized = true
	}

	for {
		// 1. Expire all tasks that have finished by now
		for es.activeTasks.Len() > 0 && es.activeTasks.Peek().ts <= es.currentTime {
			t := es.activeTasks.PopEvent()
			return SimEvent{
				EventID:   t.seq,
				Timestamp: time.Unix(0, int64(t.ts)),
				Status:    t.status,
			}, nil
		}

		// 2. Advance time to the next interesting moment
		var nextTaskTime int64
		if !es.hasPeeked {
			var ok bool
			es.peekedEvent, ok = es.gen.Next()
			if !ok {
				// No more tasks to generate
				if es.activeTasks.Len() == 0 {
					return SimEvent{}, errors.New("done")
				}
				// Jump to next expiry
				es.currentTime = es.activeTasks.Peek().ts
				continue
			}
			es.hasPeeked = true
		}
		nextTaskTime = es.peekedEvent.Timestamp

		var nextEventTime int64
		if es.activeTasks.Len() > 0 {
			nextEventTime = min(es.activeTasks.Peek().ts, nextTaskTime)
		} else {
			nextEventTime = nextTaskTime
		}

		// 3. Move simulated time forward
		es.currentTime = nextEventTime

		// 4. Generate new tasks that should start at or before current_time
		for es.hasPeeked && es.peekedEvent.Timestamp <= es.currentTime {
			task := es.peekedEvent
			es.hasPeeked = false

			// Calculate end time as int64
			endTime := task.Timestamp + task.Duration
			es.eventCount += 1
			status := EventError
			if task.Success {
				status = EventSuccess
			}
			heap.Push(es.activeTasks, pendingEvent{es.eventCount, endTime, status})

			// Yield start event (convert to time.Time only here)
			result := SimEvent{
				EventID:   es.eventCount,
				Timestamp: time.Unix(0, task.Timestamp),
				Status:    EventStart,
			}

			// Peek next task for next iteration
			var ok bool
			es.peekedEvent, ok = es.gen.Next()
			if ok {
				es.hasPeeked = true
			}

			return result, nil
		}
	}
}

package loadgen

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"math/rand"
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
	specs       []LoadSpec
	currentSpec int
	nextEvent   int64     // Unix nanoseconds
	specStart   int64     // Unix nanoseconds
	startTime   time.Time // Reference time for the start of the entire simulation
	rng         *rand.Rand
	k           float64
	scale       float64
}

// NewLoadGenerator creates a new load generator from an array of LoadSpecs
func NewLoadGenerator(specs []LoadSpec) *LoadGenerator {
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

	p50 := specs[0].P50LatencyMS
	p99 := specs[0].P99LatencyMS

	return &LoadGenerator{
		specs:       specs,
		currentSpec: 0,
		nextEvent:   startNano,
		specStart:   startNano,
		startTime:   startTime,
		rng:         rand.New(rand.NewSource(now.UnixNano())),
		k:           math.Log(2*p99/p50-1) / math.Log(0.99/0.5),
		scale:       0.5 * p50,
	}
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
		lg.k = math.Log(2*spec.P99LatencyMS/spec.P50LatencyMS-1) / math.Log(0.99/0.5)
		lg.scale = 0.5 * spec.P50LatencyMS
	}

	// Current event timestamp
	timestamp := lg.nextEvent

	// Calculate inter-arrival time based on RPM (exponential distribution)
	interArrival := (60.0 * 1e9) / float64(spec.RPM)
	nextInterval := -math.Log(lg.rng.Float64()) * interArrival
	lg.nextEvent += int64(nextInterval)

	// Generate response duration using power-law distribution
	x := lg.rng.Float64()
	t_delta := lg.scale * (1 + math.Exp(lg.k*(math.Log(x)-logHalf)))
	duration := int64(t_delta * 1e6) // Convert milliseconds to nanoseconds

	// Determine success: false if duration > timeout OR random error
	success := true
	timeout := int64(spec.TimeoutMS * 1e6)
	if duration > timeout {
		success = false
	} else if lg.rng.Float64() < spec.ErrorRate {
		success = false
	}

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
	Timestamp time.Time
	Status    EventStatus
}

// timeHeap implements heap.Interface for int64 nanoseconds (min-heap)
type timeHeap []int64

func (h timeHeap) Len() int           { return len(h) }
func (h timeHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h timeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *timeHeap) Push(x interface{}) {
	*h = append(*h, x.(int64))
}

func (h *timeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h timeHeap) Peek() int64 {
	return h[0]
}

// PopInt64 removes and returns the minimum element without interface boxing
func (h *timeHeap) PopInt64() int64 {
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
func down(h *timeHeap, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}
		j := j1
		if j2 := j1 + 1; j2 < n && (*h)[j2] < (*h)[j1] {
			j = j2
		}
		if (*h)[i] <= (*h)[j] {
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
	activeTasks *timeHeap
	peekedEvent LoadEvent
	hasPeeked   bool
	initialized bool
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
	h := make(timeHeap, 0, maxConcurrent)
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
		for es.activeTasks.Len() > 0 && es.activeTasks.Peek() <= es.currentTime {
			es.activeTasks.PopInt64()
			return SimEvent{
				Timestamp: time.Unix(0, es.currentTime),
				Status:    EventSuccess,
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
				es.currentTime = es.activeTasks.Peek()
				continue
			}
			es.hasPeeked = true
		}
		nextTaskTime = es.peekedEvent.Timestamp

		var nextEventTime int64
		if es.activeTasks.Len() > 0 {
			nextExpiry := es.activeTasks.Peek()
			if nextTaskTime < nextExpiry {
				nextEventTime = nextTaskTime
			} else {
				nextEventTime = nextExpiry
			}
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
			heap.Push(es.activeTasks, endTime)

			// Determine status
			status := EventStart
			if !task.Success {
				status = EventError
			}

			// Yield start event (convert to time.Time only here)
			result := SimEvent{
				Timestamp: time.Unix(0, task.Timestamp),
				Status:    status,
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

// Optimisations
const logHalf = -0.6931471805599453

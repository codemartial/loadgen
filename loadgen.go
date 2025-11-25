package loadgen

import (
	"container/heap"
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
  specs := []LoadSpec{{RPH: 1000, ErrorRate: 0.05, DurationMS: 60000, P50DurMS: 100, P99DurMS: 500, TimeoutMS: 1000}}
  gen := NewLoadGenerator(specs)
  stream := NewEventStream(gen)

  for event := stream.Next(); event != nil; event = stream.Next() {
      // event.Timestamp - simulated time
      // event.Type - EventStart or EventEnd
      // event.Success - true/false
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
	Timestamp time.Time
	Duration  time.Duration
	Success   bool
}

// LoadGenerator generates synthetic load events based on LoadSpecs
type LoadGenerator struct {
	specs         []LoadSpec
	currentSpec   int
	nextEventNano int64 // Unix nanoseconds
	specStartNano int64 // Unix nanoseconds
	startTime     time.Time // Reference time for the start of the entire simulation
	rng           *rand.Rand
	k             float64
	scale         float64
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
		specs:         specs,
		currentSpec:   0,
		nextEventNano: startNano,
		specStartNano: startNano,
		startTime:     startTime,
		rng:           rand.New(rand.NewSource(now.UnixNano())),
		k:             math.Log(2*p99/p50-1) / math.Log(0.99/0.5),
		scale:         0.5 * p50,
	}
}

// Next returns the next load event, or nil if all specs are exhausted
func (lg *LoadGenerator) Next() *LoadEvent {
	if lg.currentSpec >= len(lg.specs) {
		return nil
	}

	spec := lg.specs[lg.currentSpec]
	specDurationNano := int64(spec.DurationS) * int64(time.Second)

	// Check if current spec duration has elapsed
	if lg.nextEventNano-lg.specStartNano >= specDurationNano {
		lg.currentSpec++
		if lg.currentSpec >= len(lg.specs) {
			return nil
		}
		lg.specStartNano = lg.nextEventNano
		spec = lg.specs[lg.currentSpec]
		lg.k = math.Log(2*spec.P99LatencyMS/spec.P50LatencyMS-1) / math.Log(0.99/0.5)
		lg.scale = 0.5 * spec.P50LatencyMS
	}

	// Generate event timestamp
	timestamp := time.Unix(0, lg.nextEventNano)

	// Calculate inter-arrival time based on RPM (exponential distribution)
	// Work in nanoseconds to avoid precision loss
	interArrivalNano := (60.0 * 1e9) / float64(spec.RPM)
	nextIntervalNano := -math.Log(lg.rng.Float64()) * interArrivalNano
	lg.nextEventNano += int64(nextIntervalNano)

	// Generate response duration using power-law distribution
	// Maps uniform random [0,1] to durations matching p50 and p99
	x := lg.rng.Float64()
	t_delta := lg.scale * (1 + math.Exp(lg.k*(math.Log(x)-logHalf)))
	durationNano := int64(t_delta * 1e6) // Convert milliseconds to nanoseconds
	duration := time.Duration(durationNano)

	// Determine success: false if duration > timeout OR random error
	success := true
	timeoutNano := int64(spec.TimeoutMS * 1e6)
	if durationNano > timeoutNano {
		success = false
	} else if lg.rng.Float64() < spec.ErrorRate {
		success = false
	}

	return &LoadEvent{
		Timestamp: timestamp,
		Duration:  duration,
		Success:   success,
	}
}

// EventType represents whether an event is a start or end
type EventType int

const (
	EventStart EventType = iota
	EventEnd
)

// String returns the string representation of EventType
func (e EventType) String() string {
	switch e {
	case EventStart:
		return "start"
	case EventEnd:
		return "end"
	default:
		return "unknown"
	}
}

// SimEvent represents a discrete simulation event with timestamp and type
type SimEvent struct {
	Timestamp time.Time
	Type      EventType
	Success   bool // For end events, indicates if the task succeeded
}

// timeHeap implements heap.Interface for time.Time (min-heap)
type timeHeap []time.Time

func (h timeHeap) Len() int           { return len(h) }
func (h timeHeap) Less(i, j int) bool { return h[i].Before(h[j]) }
func (h timeHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *timeHeap) Push(x interface{}) {
	*h = append(*h, x.(time.Time))
}

func (h *timeHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h timeHeap) Peek() time.Time {
	return h[0]
}

// EventStream converts LoadEvents into discrete start/end SimEvents
type EventStream struct {
	gen         *LoadGenerator
	currentTime time.Time
	activeTasks *timeHeap
	peekedEvent *LoadEvent
	initialized bool
}

// NewEventStream creates a new event stream from a load generator
func NewEventStream(gen *LoadGenerator) *EventStream {
	h := &timeHeap{}
	heap.Init(h)
	return &EventStream{
		gen:         gen,
		activeTasks: h,
		initialized: false,
	}
}

// Next returns the next simulation event (start or end)
func (es *EventStream) Next() *SimEvent {
	if !es.initialized {
		es.currentTime = time.Time{}
		es.initialized = true
	}

	for {
		// 1. Expire all tasks that have finished by now
		for es.activeTasks.Len() > 0 && !es.activeTasks.Peek().After(es.currentTime) {
			heap.Pop(es.activeTasks)
			return &SimEvent{
				Timestamp: es.currentTime,
				Type:      EventEnd,
				Success:   true, // We'll track this separately if needed
			}
		}

		// 2. Advance time to the next interesting moment
		var nextTaskTime time.Time
		if es.peekedEvent == nil {
			es.peekedEvent = es.gen.Next()
			if es.peekedEvent == nil {
				// No more tasks to generate
				if es.activeTasks.Len() == 0 {
					return nil
				}
				// Jump to next expiry
				es.currentTime = es.activeTasks.Peek()
				continue
			}
		}
		nextTaskTime = es.peekedEvent.Timestamp

		var nextEventTime time.Time
		if es.activeTasks.Len() > 0 {
			nextExpiry := es.activeTasks.Peek()
			if nextTaskTime.Before(nextExpiry) {
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
		for es.peekedEvent != nil && !es.peekedEvent.Timestamp.After(es.currentTime) {
			task := es.peekedEvent
			es.peekedEvent = nil

			// Calculate end time
			endTime := task.Timestamp.Add(task.Duration)
			heap.Push(es.activeTasks, endTime)

			// Yield start event
			result := &SimEvent{
				Timestamp: task.Timestamp,
				Type:      EventStart,
				Success:   task.Success,
			}

			// Peek next task for next iteration
			es.peekedEvent = es.gen.Next()

			return result
		}
	}
}

// Optimisations
const lutEnd = 0.125
const lutSize = 1024
const logHalf = -0.6931471805599453

var lut [lutSize]float64

func init() {
	for i := 1; i < lutSize; i++ {
		u := float64(i) / float64(lutSize) * lutEnd
		lut[i] = math.Log(u)
	}
	lut[0] = lut[1] // avoid -inf
}

func log(u float64) float64 {
	if u >= lutEnd {
		// cubic polynomial
		y := u - 1.0
		return y * (1.0 + y*(-0.5+y*(0.333331-0.25*y)))
	}

	// lookup with linear interpolation
	idx := u * float64(lutSize-1) / lutEnd
	i := int(idx)
	frac := idx - float64(i)
	return lut[i] + frac*(lut[i+1]-lut[i])
}

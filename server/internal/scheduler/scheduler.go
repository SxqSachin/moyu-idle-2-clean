package scheduler

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

type Event struct {
	ID        string
	PlayerID  int64
	Type      string
	TriggerAt time.Time
	Payload   map[string]any
}

type Handler func(context.Context, Event)

type EngineType string

const (
	EngineMinHeap   EngineType = "minheap"
	EngineTimeWheel EngineType = "timewheel"
)

type Options struct {
	Engine    EngineType
	ShardCount int
	Tick      time.Duration
	WheelSize int
}

type Scheduler struct {
	shards []shard
	stop   chan struct{}
	wg     sync.WaitGroup
	tick   time.Duration
}

func New(opts Options) *Scheduler {
	if opts.ShardCount < 1 {
		opts.ShardCount = 1
	}
	if opts.Tick <= 0 {
		opts.Tick = 100 * time.Millisecond
	}
	if opts.WheelSize < 8 {
		opts.WheelSize = 256
	}

	s := &Scheduler{
		shards: make([]shard, opts.ShardCount),
		stop:   make(chan struct{}),
		tick:   opts.Tick,
	}
	for i := range s.shards {
		s.shards[i] = shard{
			queue: newQueue(opts.Engine, opts.Tick, opts.WheelSize),
		}
	}
	return s
}

func (s *Scheduler) Start(ctx context.Context) {
	for i := range s.shards {
		s.wg.Add(1)
		go s.loop(ctx, i)
	}
}

func (s *Scheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
}

func (s *Scheduler) Add(e Event) {
	idx := int(e.PlayerID % int64(len(s.shards)))
	if idx < 0 {
		idx = -idx
	}
	sh := &s.shards[idx]
	sh.mu.Lock()
	sh.queue.Add(e)
	sh.mu.Unlock()
}

func (s *Scheduler) SetHandler(h Handler) {
	for i := range s.shards {
		s.shards[i].handler = h
	}
}

func (s *Scheduler) loop(ctx context.Context, idx int) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	sh := &s.shards[idx]
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			now := time.Now()
			sh.mu.Lock()
			due := sh.queue.DrainDue(now)
			handler := sh.handler
			sh.mu.Unlock()
			if handler != nil {
				for _, e := range due {
					handler(ctx, e)
				}
			}
		}
	}
}

type shard struct {
	mu      sync.Mutex
	queue   eventQueue
	handler Handler
}

type eventQueue interface {
	Add(Event)
	DrainDue(time.Time) []Event
}

func newQueue(engine EngineType, tick time.Duration, wheelSize int) eventQueue {
	if engine == EngineTimeWheel {
		return newTimeWheelQueue(tick, wheelSize)
	}
	return newMinHeapQueue()
}

type minHeapQueue struct {
	events eventHeap
}

func newMinHeapQueue() *minHeapQueue {
	q := &minHeapQueue{events: make(eventHeap, 0)}
	heap.Init(&q.events)
	return q
}

func (q *minHeapQueue) Add(e Event) {
	heap.Push(&q.events, e)
}

func (q *minHeapQueue) DrainDue(now time.Time) []Event {
	var due []Event
	for q.events.Len() > 0 {
		top := q.events[0]
		if top.TriggerAt.After(now) {
			break
		}
		due = append(due, heap.Pop(&q.events).(Event))
	}
	return due
}

type timeWheelQueue struct {
	tick       time.Duration
	wheelSize  int
	buckets    [][]wheelItem
	current    int
	lastAdvance time.Time
}

type wheelItem struct {
	event  Event
	rounds int
}

func newTimeWheelQueue(tick time.Duration, wheelSize int) *timeWheelQueue {
	return &timeWheelQueue{
		tick:        tick,
		wheelSize:   wheelSize,
		buckets:     make([][]wheelItem, wheelSize),
		lastAdvance: time.Now(),
	}
}

func (q *timeWheelQueue) Add(e Event) {
	now := time.Now()
	delay := e.TriggerAt.Sub(now)
	if delay < 0 {
		delay = 0
	}
	ticks := int(delay / q.tick)
	if delay%q.tick != 0 {
		ticks++
	}
	slot := (q.current + ticks) % q.wheelSize
	rounds := ticks / q.wheelSize
	q.buckets[slot] = append(q.buckets[slot], wheelItem{event: e, rounds: rounds})
}

func (q *timeWheelQueue) DrainDue(now time.Time) []Event {
	var due []Event
	for q.lastAdvance.Add(q.tick).Before(now) || q.lastAdvance.Add(q.tick).Equal(now) {
		q.lastAdvance = q.lastAdvance.Add(q.tick)
		q.current = (q.current + 1) % q.wheelSize
		bucket := q.buckets[q.current]
		q.buckets[q.current] = q.buckets[q.current][:0]
		for _, item := range bucket {
			if item.rounds > 0 {
				item.rounds--
				q.buckets[q.current] = append(q.buckets[q.current], item)
				continue
			}
			if item.event.TriggerAt.After(now) {
				q.Add(item.event)
				continue
			}
			due = append(due, item.event)
		}
	}
	return due
}

type eventHeap []Event

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	return h[i].TriggerAt.Before(h[j].TriggerAt)
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *eventHeap) Push(x any) {
	*h = append(*h, x.(Event))
}
func (h *eventHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}


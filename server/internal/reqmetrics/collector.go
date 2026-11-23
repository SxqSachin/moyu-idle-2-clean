package reqmetrics

import (
	"context"
	"math"
	"sync"
	"time"
)

const ContextKey = "request.metrics.collector"

type Cost struct {
	Total        float64 `json:"total"`
	Auth         float64 `json:"auth"`
	AuthParse    float64 `json:"authParse"`
	AuthDB       float64 `json:"authDb"`
	PG           float64 `json:"pg"`
	PGAcquire    float64 `json:"pgAcquire"`
	NATS         float64 `json:"nats"`
	Redis        float64 `json:"redis"`
	Server       float64 `json:"server"`
	Business     float64 `json:"business"`
	PoolAcquired int32   `json:"poolAcquired"`
	PoolIdle     int32   `json:"poolIdle"`
	PoolMax      int32   `json:"poolMax"`
}

type Collector struct {
	mu sync.Mutex

	startAt       time.Time
	businessStart time.Time
	businessEnd   time.Time
	authDur       time.Duration
	authParseDur  time.Duration
	authDBDur     time.Duration
	pgDur         time.Duration
	pgAcquireDur  time.Duration
	natsDur       time.Duration
	redisDur      time.Duration
	poolAcquired  int32
	poolIdle      int32
	poolMax       int32
}

func New(start time.Time) *Collector {
	return &Collector{
		startAt:       start,
		businessStart: start,
	}
}

func FromContext(ctx context.Context) *Collector {
	if ctx == nil {
		return nil
	}
	if raw := ctx.Value(ContextKey); raw != nil {
		if collector, ok := raw.(*Collector); ok {
			return collector
		}
	}
	return nil
}

func (c *Collector) SetBusinessStart(at time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.businessStart = at
	c.mu.Unlock()
}

func (c *Collector) SetBusinessEnd(at time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.businessEnd = at
	c.mu.Unlock()
}

func (c *Collector) AddAuth(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.authDur += d
	c.mu.Unlock()
}

func (c *Collector) AddAuthParse(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.authParseDur += d
	c.mu.Unlock()
}

func (c *Collector) AddAuthDB(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.authDBDur += d
	c.mu.Unlock()
}

func (c *Collector) AddPG(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pgDur += d
	c.mu.Unlock()
}

func (c *Collector) AddPGAcquire(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pgAcquireDur += d
	c.mu.Unlock()
}

func (c *Collector) AddNATS(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.natsDur += d
	c.mu.Unlock()
}

func (c *Collector) AddRedis(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.redisDur += d
	c.mu.Unlock()
}

func (c *Collector) SetPoolStats(acquired, idle, maxConns int32) {
	if c == nil {
		return
	}
	c.mu.Lock()
	if shouldReplacePoolStats(c.poolAcquired, c.poolMax, acquired, maxConns) {
		c.poolAcquired = acquired
		c.poolIdle = idle
		c.poolMax = maxConns
	}
	c.mu.Unlock()
}

func shouldReplacePoolStats(currentAcquired, currentMax, nextAcquired, nextMax int32) bool {
	if currentMax <= 0 {
		return true
	}
	if nextMax <= 0 {
		return false
	}
	currentScore := int64(currentAcquired) * int64(nextMax)
	nextScore := int64(nextAcquired) * int64(currentMax)
	if nextScore != currentScore {
		return nextScore > currentScore
	}
	return nextAcquired > currentAcquired
}

func (c *Collector) Snapshot(now time.Time) Cost {
	if c == nil {
		return Cost{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if now.Before(c.startAt) {
		now = c.startAt
	}
	totalDur := now.Sub(c.startAt)

	businessStart := c.businessStart
	if businessStart.IsZero() {
		businessStart = c.startAt
	}
	businessEnd := c.businessEnd
	if businessEnd.IsZero() || businessEnd.Before(businessStart) {
		businessEnd = now
	}
	businessDur := businessEnd.Sub(businessStart)
	if businessDur < 0 {
		businessDur = 0
	}
	if businessDur > totalDur {
		businessDur = totalDur
	}
	serverDur := totalDur - businessDur
	if serverDur < 0 {
		serverDur = 0
	}

	return Cost{
		Total:        durationMS(totalDur),
		Auth:         durationMS(c.authDur),
		AuthParse:    durationMS(c.authParseDur),
		AuthDB:       durationMS(c.authDBDur),
		PG:           durationMS(c.pgDur),
		PGAcquire:    durationMS(c.pgAcquireDur),
		NATS:         durationMS(c.natsDur),
		Redis:        durationMS(c.redisDur),
		Server:       durationMS(serverDur),
		Business:     durationMS(businessDur),
		PoolAcquired: c.poolAcquired,
		PoolIdle:     c.poolIdle,
		PoolMax:      c.poolMax,
	}
}

func durationMS(d time.Duration) float64 {
	ms := float64(d) / float64(time.Millisecond)
	if ms < 0 {
		ms = 0
	}
	return math.Round(ms*1000) / 1000
}

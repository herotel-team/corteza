package rbac

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cortezaproject/corteza/server/pkg/slice"
	"go.uber.org/zap"
)

type (
	StatsLogger struct {
		lock sync.RWMutex
		log  *zap.Logger

		// Channels for async comms
		cacheHitChan  chan statsWrap
		cacheMissChan chan statsWrap
		timingChan    chan time.Duration

		// Counters
		cacheHits    uint
		cacheMisses  uint
		cacheUpdates uint
		avgTiming    time.Duration
		minTiming    time.Duration
		maxTiming    time.Duration

		// Track a limited set of things
		// Using a circular buffer we can easily not consume too much data
		lastHits    *slice.Circular[string]
		lastMisses  *slice.Circular[string]
		lastTimings *slice.Circular[time.Duration]
	}

	// statsWrap wraps the state to log
	statsWrap struct {
		roles    []uint64
		resource string
		op       string
	}
)

// Stats returns the tracked stats
func (l *StatsLogger) Stats() (cacheHit uint, cacheMiss uint, cacheUpdates uint, avgTiming, minTiming, maxTiming time.Duration, lastHits []string, lastMisses []string, lastTimings []time.Duration) {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.cacheHits,
		l.cacheMisses,
		l.cacheUpdates,
		l.avgTiming,
		l.minTiming,
		l.maxTiming,
		l.lastHits.Slice(),
		l.lastMisses.Slice(),
		l.lastTimings.Slice()
}

// Timing logs the giving duration
func (l *StatsLogger) Timing(timing time.Duration) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.log.Info("record timing", zap.Duration("timing", timing))

	{
		l.avgTiming = (l.avgTiming + timing) / 2
	}

	{
		if l.minTiming == 0 {
			l.minTiming = timing
		}
		if timing < l.minTiming {
			l.minTiming = timing
		}
	}

	{
		if l.maxTiming == 0 {
			l.maxTiming = timing
		}
		if timing > l.maxTiming {
			l.maxTiming = timing
		}
	}

	{
		if l.lastTimings == nil {
			l.lastTimings = slice.NewCircular[time.Duration](500)
		}

		l.lastTimings.Add(timing)
	}
}

func (l *StatsLogger) CacheHit(roles []uint64, resource string, op string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.log.Info("cache hit", zap.Any("roles", roles), zap.String("resource", resource), zap.String("op", op))

	l.cacheHits++
	if l.lastHits == nil {
		l.lastHits = slice.NewCircular[string](10000)
	}
	l.lastHits.Add(l.strfEntry(roles, resource, op))
}

func (l *StatsLogger) CacheMiss(roles []uint64, resource string, op string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.log.Info("cache miss", zap.Any("roles", roles), zap.String("resource", resource), zap.String("op", op))

	l.cacheMisses++
	if l.lastMisses == nil {
		l.lastMisses = slice.NewCircular[string](10000)
	}
	l.lastMisses.Add(l.strfEntry(roles, resource, op))
}

func (l *StatsLogger) CacheUpdate(in *Rule) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.log.Info("cache update", zap.Any("rule", in))

	l.cacheUpdates++
}

// // // // // // // // // // // // // // // // // // // // // // // // //
// Utils

func (l *StatsLogger) strfEntry(roles []uint64, resource string, op string) string {
	sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })

	return fmt.Sprintf("%v %s %s", roles, op, resource)
}

func (l *StatsLogger) watch(ctx context.Context) {
	t := time.NewTicker(time.Minute * 5)

	go func() {
		for {
			select {
			case <-t.C:
				l.log.Info("stats logger tick")

			case rs := <-l.cacheMissChan:
				l.CacheMiss(rs.roles, rs.resource, rs.op)

			case rs := <-l.cacheHitChan:
				l.CacheHit(rs.roles, rs.resource, rs.op)

			case tt := <-l.timingChan:
				l.Timing(tt)

			case <-ctx.Done():
				l.log.Info("terminating watcher")
			}
		}
	}()
}

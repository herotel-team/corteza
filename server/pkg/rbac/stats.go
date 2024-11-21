package rbac

type (
	// @todo :)
	stats struct {
		cacheHitChan  chan string
		cacheMissChan chan string

		// cacheHits

	}

	noopStatLogger struct{}
)

func Statser() {

}

func (l *stats) CacheHit([]uint64, string, string)  {}
func (l *stats) CacheMiss([]uint64, string, string) {}
func (l *stats) CacheUpdate(in *Rule)               {}

// Noop

func (l *noopStatLogger) CacheHit([]uint64, string, string)  {}
func (l *noopStatLogger) CacheMiss([]uint64, string, string) {}
func (l *noopStatLogger) CacheUpdate(in *Rule)               {}

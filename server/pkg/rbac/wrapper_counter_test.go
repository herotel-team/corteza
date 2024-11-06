package rbac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapperCounter(t *testing.T) {
	// @note since I'm leaving the decayInterval empty we don't need to fiddle
	// with lastAccess timestamps
	svc := &usageCounter[string]{
		index: map[string]counterItem[string]{},

		sigEvictThreshold: 0.5,
		decayFactor:       0.5,
	}

	svc.inc("k1")
	aux := svc.index["k1"]
	require.Equal(t, 1.0, aux.score)

	svc.inc("k2")
	aux = svc.index["k1"]
	require.Equal(t, 1.0, aux.score)
	aux = svc.index["k2"]
	require.Equal(t, 1.0, aux.score)

	svc.inc("k1")
	aux = svc.index["k1"]
	require.Equal(t, 2.0, aux.score)
	aux = svc.index["k2"]
	require.Equal(t, 1.0, aux.score)

	svc.decay()
	aux = svc.index["k1"]
	require.Equal(t, 1.0, aux.score)
	aux = svc.index["k2"]
	require.Equal(t, 0.5, aux.score)

	cleaned := svc.evict()
	require.Len(t, cleaned, 1)
	aux, ok := svc.index["k1"]
	require.True(t, ok)

	aux, ok = svc.index["k2"]
	require.False(t, ok)

	svc.decay()
	aux = svc.index["k1"]
	require.Equal(t, 0.5, aux.score)

	cleaned = svc.evict()
	require.Len(t, cleaned, 1)
	aux, ok = svc.index["k1"]
	require.False(t, ok)
}

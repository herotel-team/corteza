package rbac

import (
	"fmt"
	"strings"
	"sync"
)

type (
	wrapperIndex struct {
		mux   sync.RWMutex
		index *ruleIndex

		// indexed permits only max level identifiers
		indexed map[string]bool
	}
)

func (svc *wrapperIndex) add(role uint64, resource string, rules ...*Rule) (added bool) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	if svc.indexed == nil {
		svc.indexed = make(map[string]bool, 24)
	}

	if svc.index == nil {
		svc.index = &ruleIndex{}
	}

	// Since we're only allowed to index under full resource identifiers
	// we'll only optionally update indexes if something new comes in.
	if strings.Contains(resource, "*") {
		return svc.addWild(role, resource, rules...)
	} else {
		return svc.addPlain(role, resource, rules...)
	}
}

// addWild handles scenario where we would grant permissions for a wildcard
//
// In case of a wild card we need to check if any matching resource falls under it
// if so, add it to the index, if not, ignore.
//
// In no case should we add this to the indexed map since it only permits max lvl identifiers.
func (svc *wrapperIndex) addWild(role uint64, resource string, rules ...*Rule) (added bool) {
	give := false
	rKey := svc.makeKey(role, resource)

	for k := range svc.indexed {
		give = give || strings.HasPrefix(k, rKey)
	}

	if !give {
		return false
	}

	svc.index.add(rules...)
	return true
}

func (svc *wrapperIndex) addPlain(role uint64, resource string, rules ...*Rule) (added bool) {
	svc.indexed[svc.makeKey(role, resource)] = true
	svc.index.add(rules...)

	return true
}

func (svc *wrapperIndex) get(role uint64, op string, res string) (out []*Rule) {
	if svc == nil {
		return
	}

	svc.mux.RLock()
	defer svc.mux.RUnlock()

	if svc.index == nil {
		return
	}

	return svc.index.get(role, op, res)
}

func (svc *wrapperIndex) getIndexed() (out []string) {
	for k := range svc.indexed {
		out = append(out, k)
	}

	return
}

func (svc *wrapperIndex) getSize() int {
	svc.mux.RLock()
	defer svc.mux.RUnlock()

	return len(svc.indexed)
}

// isIndexed returns true if the resource is either indexed or potentially indexed
//
// If we're providing a max level resource identifier, it must occur in the index
// If we're providing a wildcard, we always assume it's in there
//
// # Underlying functions need to respect this
//
// @todo consider keeping track of prefixes so we can know for a fact.
// It doesn't really matter at this point since referencing functions don't care about this
func (svc *wrapperIndex) isIndexed(role uint64, resource string) (ok bool) {
	// In case of wildcards, assume we have it indexed; further functions need
	// to handle this properly
	if strings.Contains(resource, "*") {
		return true
	}

	svc.mux.RLock()
	defer svc.mux.RUnlock()

	if svc.indexed == nil {
		return false
	}

	return svc.indexed[svc.makeKey(role, resource)]
}

func (svc *wrapperIndex) makeKey(role uint64, resource string) string {
	pp := strings.SplitN(resource, "*", 2)
	resource = strings.TrimRight(pp[0], "/")

	return fmt.Sprintf("%d:%s", role, resource)
}

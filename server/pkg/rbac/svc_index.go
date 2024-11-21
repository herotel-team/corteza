package rbac

import (
	"fmt"
	"sync"
)

type (
	wrapperIndex struct {
		mux     sync.RWMutex
		index   *ruleIndex
		indexed map[string]bool
	}
)

func (svc *wrapperIndex) add(role uint64, resource string, rules ...*Rule) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	if svc.indexed == nil {
		svc.indexed = make(map[string]bool, 24)
	}

	if svc.index == nil {
		svc.index = &ruleIndex{}
	}

	svc.indexed[svc.mkkey(role, resource)] = true
	svc.index.add(rules...)
}

func (svc *wrapperIndex) get(role uint64, op string, res string) (out []*Rule) {
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

func (svc *wrapperIndex) isIndexed(role uint64, resource string) (ok bool) {
	svc.mux.RLock()
	defer svc.mux.RUnlock()

	if svc.indexed == nil {
		return false
	}

	return svc.indexed[svc.mkkey(role, resource)]
}

func (svc *wrapperIndex) mkkey(role uint64, resource string) string {
	return fmt.Sprintf("%d:%s", role, resource)
}

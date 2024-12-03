package rbac

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cortezaproject/corteza/server/system/types"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

type (
	Service struct {
		mux        sync.RWMutex
		cfg        Config
		logger     *zap.Logger
		StatLogger *StatsLogger

		noop       bool
		noopAccess Access

		usageCounter *usageCounter[string]
		index        *wrapperIndex
		roles        []*Role

		RuleStorage rbacRulesStore
		RoleStorage rbacRoleStore
	}

	Config struct {
		// MaxIndexSize limits the max size of the in memory index
		//
		// When set to -1, max size is used
		// When set to 0, the in memory index is not used
		MaxIndexSize int

		// Synchronous lets us make all the procedures synchronous for ease of testing
		// This should always be false in production
		Synchronous bool

		// ReindexStrategy specifies how the index needs to be recalcualted
		// The default option is ReindexStrategyMemory
		//
		// If both speed and memory are needed, consider reducing MaxIndexSize
		// while using ReindexStrategySpeed
		ReindexStrategy ReindexStrategy

		// DecayFactor states how fast an indexed value looses it's score
		DecayFactor float64
		// DecayInterval states how often we should decay indexed items
		DecayInterval time.Duration
		// CleanupInterval states how often stale or poor performers should be thrown out
		CleanupInterval time.Duration
		// ReindexInterval states how often we should reindex based on updated scores
		ReindexInterval time.Duration
		// IndexFlushInterval states how often the index state should be flushed to the database
		IndexFlushInterval time.Duration

		// RuleStorage provides the methods to interact with rules
		RuleStorage rbacRulesStore
		// RoleStorage provides the methods to interact with roles
		RoleStorage rbacRoleStore

		// PullInitialState provides the initial index state
		//
		// The string slice provides index keys which should then be further processed
		// to determine the actual index state.
		//
		// When working with resource rule combos, the key will be `{roleID}:{resourceIdentifier}`
		PullInitialState func(ctx context.Context, n int) ([]string, error)
		// FlushIndexState takes the current index state and flushes it to the database
		// @todo for now it's a noop; we should preserve
		FlushIndexState func(context.Context, []string) error
	}

	// evaluationState is a little helper to keep all the things we need in place
	evaluationState struct {
		unindexedRoles partRoles
		indexedRoles   partRoles

		unindexedRules [5]map[uint64][]*Rule

		res string
		op  string
	}

	expCtrItem struct {
		Key   string  `json:"key"`
		Score float64 `json:"score"`

		// added denotes when the item was added to the counter
		Added time.Time `json:"added"`
		// lastScored denotes when the item was last scored (either via decay or access)
		LastScored time.Time `json:"lastScored"`
		// lastAccess denotes when the item was last accessed, needed
		LastAccess time.Time `json:"lastAccess"`
	}

	Stats struct {
		CacheHits    uint          `json:"cacheHits"`
		CacheMisses  uint          `json:"cacheMisses"`
		CacheUpdates uint          `json:"cacheUpdates"`
		AvgTiming    time.Duration `json:"avgTiming"`
		MinTiming    time.Duration `json:"minTiming"`
		MaxTiming    time.Duration `json:"maxTiming"`

		IndexSize int `json:"indexSize"`

		LastHits    []string        `json:"lastHits"`
		LastMisses  []string        `json:"lastMisses"`
		LastTimings []time.Duration `json:"lastTimings"`

		Counters []expCtrItem `json:"counters"`
	}

	RuleFilter struct {
		Resource  []string
		Operation string
		RoleID    uint64

		Limit uint
	}

	RoleSettings struct {
		Bypass        []uint64
		Authenticated []uint64
		Anonymous     []uint64
	}

	ReindexStrategy string
)

const (
	// ReindexStrategyDefault defaults to ReindexStrategyMemory
	ReindexStrategyDefault ReindexStrategy = ""
	// ReindexStrategyMemory prioritizes memory consumption over speed
	//
	// This mode firstly clears out stale values and then pulls in existing.
	// The memory consumption should remain about the same through this process.
	ReindexStrategyMemory ReindexStrategy = "memory"
	// ReindexStrategySpeed prioritizes speed over memory
	//
	// This mode firstly builds the new index with the same (or larger) size as
	// the current one (the new index falls under the upper limit).
	// The memory consumption, worst case, will be 2x the upper limit.
	ReindexStrategySpeed ReindexStrategy = "speed"

	RuleResourceType = "corteza::generic:rbac-rule"
)

var (
	// Global RBAC service
	gWrapper *Service
)

// Global returns global RBAC service
func Global() *Service {
	return gWrapper
}

// SetGlobal re-sets global service
func SetGlobal(svc *Service) {
	gWrapper = svc
}

// NoopSvc creates a blank RBAC service which always returns the stated access
func NoopSvc(access Access, cc Config) (svc *Service) {
	return &Service{
		noop:       true,
		noopAccess: access,
		logger:     zap.NewNop(),

		RuleStorage: cc.RuleStorage,
		RoleStorage: cc.RoleStorage,

		cfg: cc,
	}
}

// NewService initializes the wrapper service with all the required surrounding bits
func NewService(ctx context.Context, l *zap.Logger, store rbacRulesStore, cc Config) (svc *Service, err error) {
	cc = defaultWrapperConfig(cc)

	uc := initUsageCounter(ctx, cc)
	sl := initStatsLogger(ctx, l)
	svc = initSvc(ctx, l, cc, sl, uc)

	// Init bits and pieces
	svc.roles, err = svc.loadRoles(ctx)
	if err != nil {
		return
	}

	svc.index, err = svc.loadIndex(ctx)
	if err != nil {
		return
	}

	return
}

func NewServiceMust(ctx context.Context, l *zap.Logger, store rbacRulesStore, cc Config) (svc *Service) {
	svc, err := NewService(ctx, l, store, cc)
	if err != nil {
		panic(fmt.Sprintf("NewServiceMust failed with: %v", err))
	}

	return
}

func initUsageCounter(ctx context.Context, cc Config) (svc *usageCounter[string]) {
	svc = &usageCounter[string]{
		incChan: make(chan string, 1024),

		decayFactor:     cc.DecayFactor,
		decayInterval:   cc.DecayInterval,
		cleanupInterval: cc.CleanupInterval,

		checkKeyInclusion: func(k string, role uint64) bool {
			return strings.HasPrefix(k, strconv.FormatUint(role, 10))
		},
	}

	svc.watch(ctx)
	return
}

func initStatsLogger(ctx context.Context, l *zap.Logger) (svc *StatsLogger) {
	svc = &StatsLogger{
		log:           l.Named("rbac stats logger"),
		cacheHitChan:  make(chan statsWrap, 1024),
		cacheMissChan: make(chan statsWrap, 1024),
		timingChan:    make(chan time.Duration, 1024),
	}

	svc.watch(ctx)
	return
}

func initSvc(ctx context.Context, l *zap.Logger, cc Config, sl *StatsLogger, uc *usageCounter[string]) (svc *Service) {
	svc = &Service{
		logger: l,

		cfg:        cc,
		StatLogger: sl,

		usageCounter: uc,

		RuleStorage: cc.RuleStorage,
		RoleStorage: cc.RoleStorage,
	}

	svc.watch(ctx)

	return
}

func defaultWrapperConfig(base Config) (out Config) {
	out = base

	// -1 disables partitioning so everything is pulled in memory
	if base.MaxIndexSize == 0 {
		out.MaxIndexSize = -1
	}

	// Noop to avoid branching down the line
	if base.FlushIndexState == nil {
		out.FlushIndexState = func(ctx context.Context, s []string) error { return nil }
	}

	if base.ReindexStrategy == ReindexStrategyDefault {
		out.ReindexStrategy = ReindexStrategyMemory
	}

	return out
}

// Can returns true if the given resource can be accessed
func (svc *Service) Can(ses Session, op string, res Resource) (ok bool) {
	ac, err := svc.Check(ses, op, res)
	if err != nil {
		svc.logger.Error("check failed with error",
			zap.String("op", op),
			zap.String("resource", res.RbacResource()),
			zap.Error(err),
		)
		return false
	}

	return ac == Allow
}

// Check returns the RBAC evaluation of the resource access
func (svc *Service) Check(ses Session, op string, res Resource) (a Access, err error) {
	if svc.noop {
		svc.logger.Debug(fmt.Sprintf("check bypass %v %v %v: %v", ses, op, res, svc.noopAccess))
		return svc.noopAccess, nil
	}

	svc.logger.Debug(fmt.Sprintf("check %v %v %v", ses, op, res))

	if hasWildcards(res.RbacResource()) {
		// prevent use of wildcard resources for checking permissions
		return Inherit, nil
	}

	roles := evalRoles(ses, res, svc.roles...)

	// @todo something more robust?
	svc.incCounter(roles, res)

	return svc.check(ses.Context(), roles, op, res.RbacResource(), nil)
}

// Trace checks RBAC rules and returns all decision trace log
func (svc *Service) Trace(ses Session, op string, res Resource) (*Trace, error) {
	var (
		t = new(Trace)
	)

	if hasWildcards(res.RbacResource()) {
		// a special case for when user has contextual roles
		// AND trace is done on a resource with wildcards
		ctxRolesDebug := partRoles{ContextRole: make(map[uint64]bool)}
		for _, memberOf := range ses.Roles() {
			for _, role := range svc.roles {
				if role.kind != ContextRole {
					continue
				}

				if role.id != memberOf {
					continue
				}

				// member of contextual role
				//
				// this is a tricky situation:
				// when doing regular check this is an unlikely scenario since
				// check can not be done on a resource with wildcards
				//
				// all contextual roles we're members off will be collected
				ctxRolesDebug[ContextRole][memberOf] = true

			}
		}

		if len(ctxRolesDebug[ContextRole]) > 0 {
			// session has at least one contextual role
			// and since we're checking this on a wildcard resource
			// there is no need to procede with RBAC check
			baseTraceInfo(t, res.RbacResource(), op, ctxRolesDebug)
			resolve(t, Inherit, unknownContext)
			return t, nil
		}
	}

	var (
		fRoles = evalRoles(ses, res, svc.roles...)
	)

	_, err := svc.check(ses.Context(), fRoles, op, res.RbacResource(), nil)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// Grant appends and/or overwrites internal rules slice
//
// All rules with Inherit are removed
func (svc *Service) Grant(ctx context.Context, rules ...*Rule) (err error) {
	for _, r := range rules {
		if svc.logger == nil {
			continue
		}

		svc.logger.Debug(r.Access.String() + " " + r.Operation + " on " + r.Resource + " to " + strconv.FormatUint(r.RoleID, 10))
	}

	svc.mux.Lock()
	if svc.index == nil {
		svc.index = &wrapperIndex{}
	}
	// @todo we might manage to optimize this a bit by grouping
	for _, r := range rules {
		// If this resource role combo isn't indexed, we don't care
		if !svc.index.isIndexed(r.RoleID, r.Resource) {
			continue
		}

		// If it is, we need to assure this thing is inside the index now
		svc.index.add(r.RoleID, r.Resource, r)
		svc.StatLogger.CacheUpdate(r)
	}
	svc.mux.Unlock()

	// Flush changes to database :)

	err = svc.flush(ctx, rules...)
	if err != nil {
		return
	}

	return
}

func (svc *Service) Stats() (out Stats, err error) {
	svc.usageCounter.lock.RLock()
	defer svc.usageCounter.lock.RUnlock()

	for k, itm := range svc.usageCounter.index {
		out.Counters = append(out.Counters, expCtrItem{
			Key:        k,
			Score:      itm.score,
			Added:      itm.added,
			LastScored: itm.lastScored,
			LastAccess: itm.lastAccess,
		})
	}

	out.CacheHits,
		out.CacheMisses,
		out.CacheUpdates,
		out.AvgTiming,
		out.MinTiming,
		out.MaxTiming,
		out.LastHits,
		out.LastMisses,
		out.LastTimings = svc.StatLogger.Stats()

	out.IndexSize = svc.index.getSize()

	return
}

func (svc *Service) UpdateRoles(rr ...*Role) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	stats := statRoles(rr...)
	svc.logger.Debug(
		"updating roles",
		zap.Int("before", len(svc.roles)),
		zap.Int("after", len(rr)),
		zap.Int("bypass", stats[BypassRole]),
		zap.Int("context", stats[ContextRole]),
		zap.Int("common", stats[CommonRole]),
		zap.Int("authenticated", stats[AuthenticatedRole]),
		zap.Int("anonymous", stats[AnonymousRole]),
	)

	removed := removedRoles(svc.roles, rr...)
	svc.cleanupCounter(removed...)

	// @todo log update stats?
	svc.roles = rr
}

// FindRulesByRoleID returns all RBAC rules that belong to a role
func (svc *Service) FindRulesByRoleID(ctx context.Context, roleID uint64) (rr RuleSet, err error) {
	aux, _, err := svc.RuleStorage.SearchRbacRules(ctx, RuleFilter{
		RoleID: roleID,
	})
	if err != nil {
		return
	}

	for _, x := range aux {
		rr = append(rr, &Rule{
			RoleID:    x.RoleID,
			Resource:  x.Resource,
			Operation: x.Operation,
			Access:    x.Access,
		})
	}

	return
}

// Remove role removes the role from the service
//
// @todo this won't clean out the removed rules until the next reload
func (svc *Service) RemoveRole(r *Role) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	for i, xr := range svc.roles {
		if xr.id != r.id {
			continue
		}

		svc.roles = append(svc.roles[:i], svc.roles[i+1:]...)
		return
	}
}

// IndexSize returns the number of indexed role/rule combos
func (svc *Service) IndexSize() int {
	if svc.index == nil {
		return 0
	}

	return svc.index.getSize()
}

// SignificantRoles returns two list of significant roles.
//
// See sigRoles on rules for more details
func (svc *Service) SignificantRoles(ctx context.Context, res Resource, op string) (aRR, dRR []uint64, err error) {
	svc.mux.RLock()
	defer svc.mux.RUnlock()

	aux, _, err := svc.RuleStorage.SearchRbacRules(ctx, RuleFilter{
		Resource:  []string{res.RbacResource()},
		Operation: op,
	})
	if err != nil {
		return
	}

	aRR, dRR = aux.sigRoles(res.RbacResource(), op)
	return
}

func (svc *Service) Rules(ctx context.Context) (out RuleSet, err error) {
	out, _, err = svc.RuleStorage.SearchRbacRules(ctx, RuleFilter{})
	return
}

// Clear cleans out all the data
func (svc *Service) Clear() {
	svc.usageCounter = nil
	svc.index = nil
	svc.roles = nil
}

// // // // // // // // // // // // // // // // // // // // // // // // //
// Supporting

func (svc *Service) check(ctx context.Context, rolesByKind partRoles, op, res string, trace *Trace) (a Access, err error) {
	// Preflight to resolve some pre-known states which need to bypass the standard flow
	a, resolved := svc.preflightCheck(rolesByKind)
	if resolved {
		return
	}

	st := evaluationState{op: op, res: res}
	st.indexedRoles, st.unindexedRoles, err = svc.segmentRoles(rolesByKind, res)
	if err != nil {
		return Inherit, err
	}

	// @todo can we do something with this?
	svc.logCachePerformance(st.indexedRoles, st.unindexedRoles, res, op)

	if trace != nil {
		// from this point on, there is a chance trace (if set)
		// will contain some rules.
		//
		// Stable order needs to be ensured: there is no production
		// code that relies on that but tests might fail and API
		// response would be flaky.
		defer sortTraceRules(trace)
	}

	// @todo should we cache this for n seconds? just in case it's going to happen again soon?
	var timing time.Duration
	st.unindexedRules, timing, err = svc.pullUnindexed(ctx, st.unindexedRoles, op, res)
	if err != nil {
		return Inherit, err
	}

	svc.logDbTiming(timing)

	a, err = svc.evaluate(
		[]roleKind{ContextRole, CommonRole, AuthenticatedRole, AnonymousRole},
		trace,
		st,
		rolesByKind,
	)
	if err != nil {
		return
	}

	return
}

func (svc *Service) evaluate(roleOrder []roleKind, trace *Trace, st evaluationState, rolesByKind partRoles) (a Access, err error) {
	var (
		match   *Rule
		allowed bool
	)

	// Priority is important here. We want to have
	// stable RBAC check behaviour and ability
	// to override allow/deny depending on how niche the role (type) is:
	//  - context (eg owners) are more niche than common
	//  - rules for common roles are more important than authenticated and anonymous role types
	//
	// Note that bypass roles are intentionally ignored here; if user is member of
	// bypass role there is no need to check any other rule
	for _, kind := range roleOrder {
		// not a member of any role of this kind
		if len(rolesByKind[kind]) == 0 {
			continue
		}

		// reset allowed to false
		// for each role kind
		allowed = false

		for r := range rolesByKind[kind] {
			match = svc.getMatchingRule(st, kind, r)

			// check all rules for each role the security-context
			if match == nil {
				// no rules match
				continue
			}

			if trace != nil {
				// if trace is enabled, append
				// each matching rule
				trace.Rules = append(trace.Rules, match)
			}

			if match.Access == Deny {
				// if we stumble upon Deny we short-circuit the check
				return resolve(nil, Deny, ""), nil
			}

			if match.Access == Allow {
				// allow rule found, we need to check rules on other roles
				// before we allow it
				allowed = true
			}
		}

		if allowed {
			// at least one of the roles (per role type) in the security context
			// allows operation on a resource
			return resolve(nil, Allow, ""), nil
		}
	}

	return
}

// preflightCheck covers a few edge-case-esk scenarios
func (svc *Service) preflightCheck(roles partRoles) (a Access, resolved bool) {
	if member(roles, AnonymousRole) && len(roles) > 1 {
		// Integrity check; when user is member of anonymous role
		// should not be member of any other type of role
		return resolve(nil, Deny, failedIntegrityCheck), true
	}

	if member(roles, BypassRole) {
		// if user has at least one bypass role, we allow access
		return resolve(nil, Allow, bypassRoleMembership), true
	}

	return Inherit, false
}

// // // // // // // // // // // // // // // // // // // // // // // // //
// DB stuff

func (svc *Service) flush(ctx context.Context, rules ...*Rule) (err error) {
	// @todo is this stil valid?
	// if svc.store == nil {
	// 	svc.logger.Debug("rule flushing disabled (no store)")
	// 	return
	// }

	upsert, delete := upsertableDeletableRules(rules)
	err = svc.RuleStorage.DeleteRbacRule(ctx, delete...)
	if err != nil {
		return
	}

	err = svc.RuleStorage.UpsertRbacRule(ctx, upsert...)
	if err != nil {
		return
	}

	if svc.logger != nil {
		svc.logger.Debug(
			"flushed rules",
			zap.Int("deleted", len(delete)),
			zap.Int("upserted", len(upsert)),
		)
	}

	return
}

func (svc *Service) pullUnindexed(ctx context.Context, unindexed partRoles, op, res string) (out [5]map[uint64][]*Rule, timing time.Duration, err error) {
	resPerm := make([]string, 0, 8)
	resPerm = append(resPerm, res)

	// Get all the resource permissions
	// @todo get permissions for parent resources; this will probs be some lookup table
	now := time.Now()
	defer func() {
		timing = time.Since(now)
	}()

	rr := strings.Split(res, "/")
	for i := len(rr) - 1; i >= 0; i-- {
		rr[i] = "*"
		resPerm = append(resPerm, strings.Join(rr, "/"))
	}

	for rk, rr := range unindexed {
		for r := range rr {
			var auxRr []*Rule
			auxRr, _, err = svc.RuleStorage.SearchRbacRules(ctx, RuleFilter{
				RoleID:    r,
				Resource:  resPerm,
				Operation: op,
			})
			if err != nil {
				return
			}

			if out[rk] == nil {
				out[rk] = map[uint64][]*Rule{
					r: auxRr,
				}
			} else {
				out[rk][r] = auxRr
			}
		}
	}

	return
}

func (svc *Service) pullForRole(ctx context.Context, roleID uint64) (out []*Rule, err error) {
	out, _, err = svc.RuleStorage.SearchRbacRules(ctx, RuleFilter{
		RoleID: roleID,
	})
	if err != nil {
		return
	}

	return
}

func (svc *Service) pullRules(ctx context.Context, role uint64, resource string) (rules []*Rule, err error) {
	resPerm := make([]string, 0, 8)
	resPerm = append(resPerm, resource)

	// Get all the resource permissions
	// @todo get permissions for parent resources; this will probs be some lookup table
	rr := strings.Split(resource, "/")
	for i := len(rr) - 1; i > 0; i-- {
		rr[i] = "*"
		resPerm = append(resPerm, strings.Join(rr, "/"))
	}

	var aux RuleSet
	aux, _, err = svc.RuleStorage.SearchRbacRules(ctx, RuleFilter{
		Resource: resPerm,
		RoleID:   role,
	})

	rules = append(rules, aux...)

	return
}

func (svc *Service) ReloadRoles(ctx context.Context) (err error) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	crt := svc.roles

	svc.roles, err = svc.loadRoles(ctx)
	if err != nil {
		return
	}

	rmd := removedRoles(crt, svc.roles...)
	svc.cleanupCounter(rmd...)

	return
}

func (svc *Service) loadRoles(ctx context.Context) (out []*Role, err error) {
	auxRoles, _, err := svc.cfg.RoleStorage.SearchRoles(ctx, types.RoleFilter{})
	if err != nil {
		return
	}

	for _, ar := range auxRoles {
		out = append(out, &Role{
			id:     ar.ID,
			handle: ar.Handle,
			kind:   CommonRole,
		})
	}

	return
}

// // // // // // // // // // // // // // // // // // // // // // // // //
// Utils

// upsertableDeletableRules figures out what rules need to be upserted or deleted
func upsertableDeletableRules(rules []*Rule) (upsert, delete []*Rule) {
	for _, r := range rules {
		if r.Access == Inherit {
			delete = append(delete, r)
		} else {
			upsert = append(upsert, r)
		}
	}

	return
}

func (svc *Service) getMatchingRule(st evaluationState, kind roleKind, role uint64) (rule *Rule) {
	var (
		aux   []*Rule
		rules RuleSet
	)

	// Indexed
	aux = svc.index.get(role, st.op, st.res)
	rules = append(rules, aux...)

	// Unindexed
	aux = st.unindexedRules[kind][role]
	rules = append(rules, aux...)

	set := RuleSet(rules)
	sort.Sort(set)

	for _, s := range set {
		if s.Access == Inherit {
			continue
		}

		return s
	}

	return nil
}

// segmentRoles determines what roles are indexed and unindexed
func (svc *Service) segmentRoles(roles partRoles, resource string) (indexed, unindexed partRoles, err error) {
	svc.mux.RLock()
	defer svc.mux.RUnlock()

	unindexed = partRoles{}
	indexed = partRoles{}

	if svc.index == nil || svc.index.index == nil || svc.index.index.empty() {
		return indexed, roles, nil
	}

	unindexed[CommonRole] = make(map[uint64]bool)
	indexed[CommonRole] = make(map[uint64]bool)

	for k, rg := range roles {
		for r := range rg {
			if svc.index.isIndexed(r, resource) {
				if indexed[k] == nil {
					indexed[k] = make(map[uint64]bool)
				}

				indexed[k][r] = true
				continue
			}

			if unindexed[k] == nil {
				unindexed[k] = make(map[uint64]bool)
			}

			unindexed[k][r] = true
		}
	}

	return
}

// CloneRulesByRoleID clone all rules of a Role S to a specific Role T by removing its existing rules
func (svc *Service) CloneRulesByRoleID(ctx context.Context, fromRoleID uint64, toRoleID ...uint64) (err error) {
	var (
		updatedRules RuleSet
	)

	// Make sure rules of fromRoleID stays intact
	rr, err := svc.pullForRole(ctx, fromRoleID)
	if err != nil {
		return
	}

	for _, roleID := range toRoleID {
		// Remove existing rules
		var existingRules []*Rule
		existingRules, err = svc.pullForRole(ctx, roleID)
		if err != nil {
			return
		}

		for _, rule := range existingRules {
			// Make sure to remove existing rules
			rule.Access = Inherit
		}
		updatedRules = append(updatedRules, existingRules...)

		// Clone rules from role S to role T
		for _, rule := range rr {
			// Make sure everything is properly set
			r := *rule
			r.RoleID = roleID
			updatedRules = append(updatedRules, &r)
		}
	}

	return svc.Grant(ctx, updatedRules...)
}

// incCounter sends some messages to the usage counter
func (svc *Service) incCounter(roles partRoles, res Resource) {
	if svc.cfg.Synchronous {
		svc.incCounterSync(roles, res)
	} else {
		svc.incCounterAsync(roles, res)
	}
}

func (svc *Service) cleanupCounter(roles ...*Role) {
	if svc.cfg.Synchronous {
		svc.cleanupCounterSync(roles...)
	} else {
		svc.cleanupCounterAsync(roles...)
	}
}

func (svc *Service) incCounterSync(roles partRoles, res Resource) {
	for _, rr := range roles {
		for r := range rr {
			svc.usageCounter.inc(fmt.Sprintf("%d:%s", r, res.RbacResource()))
		}
	}
}

func (svc *Service) incCounterAsync(roles partRoles, res Resource) {
	if svc.usageCounter != nil && svc.usageCounter.incChan != nil {
		for _, rr := range roles {
			for r := range rr {
				svc.usageCounter.incChan <- fmt.Sprintf("%d:%s", r, res.RbacResource())
			}
		}
	}
}

func (svc *Service) cleanupCounterSync(roles ...*Role) {
	for _, r := range roles {
		gWrapper.usageCounter.cleanRoleKeys(r.id)
	}
}

func (svc *Service) cleanupCounterAsync(roles ...*Role) {
	if svc.usageCounter != nil && svc.usageCounter.rmChan != nil {
		for _, r := range roles {
			svc.usageCounter.rmChan <- r.id
		}
	}
}

func (svc *Service) updateWrapperIndex(ctx context.Context) (err error) {
	switch svc.cfg.ReindexStrategy {
	case ReindexStrategyMemory:
		return svc.updateWrapperIndexMemFirst(ctx)
	case ReindexStrategySpeed:
		return svc.updateWrapperIndexSpeedFirst(ctx)
	}

	return
}

func (svc *Service) updateWrapperIndexMemFirst(ctx context.Context) (err error) {
	auxIndex, err := svc.buildNewIndex(ctx)
	if err != nil {
		return
	}

	svc.swapIndexes(auxIndex)
	return
}

func (svc *Service) updateWrapperIndexSpeedFirst(ctx context.Context) (err error) {
	svc.mux.Lock()
	defer svc.mux.Unlock()

	svc.index = nil

	auxIndex, err := svc.buildNewIndex(ctx)
	if err != nil {
		return
	}

	svc.index = auxIndex
	return
}

// // // // // // // // // // // // // // // // // // // // // // // // // //
// Boilerplate & state management stuff

func (svc *Service) indexForResources(ctx context.Context, res ...string) (index *wrapperIndex, err error) {
	index = &wrapperIndex{}
	var auxRules []*Rule

	for _, b := range res {
		pp := strings.SplitN(b, ":", 2)
		role := cast.ToUint64(pp[0])
		resource := pp[1]

		auxRules, err = svc.pullRules(ctx, role, resource)
		if err != nil {
			return
		}

		index.add(role, resource, auxRules...)
	}

	return
}

func (svc *Service) loadIndex(ctx context.Context) (out *wrapperIndex, err error) {
	// How do we figure out what resources we have?
	// do we just start from empty?

	// I suppose this fnc would provide some assortment of resources...
	// For now, we'll just yank out some list of records?
	// At the end get some modules and stuff?
	// Records would be those things that need max performance I suppose so it'd be a good starting point

	if svc.cfg.PullInitialState == nil {
		return &wrapperIndex{}, nil
	}

	rr, err := svc.cfg.PullInitialState(ctx, svc.cfg.MaxIndexSize)
	if err != nil {
		return
	}

	return svc.indexForResources(ctx, rr...)
}

func (svc *Service) buildNewIndex(ctx context.Context) (index *wrapperIndex, err error) {
	svc.usageCounter.lock.RLock()
	defer svc.usageCounter.lock.RUnlock()

	res := svc.usageCounter.bestPerformers(svc.cfg.MaxIndexSize)
	return svc.indexForResources(ctx, res...)
}

func (svc *Service) swapIndexes(auxIndex *wrapperIndex) {
	if auxIndex == nil {
		return
	}

	svc.mux.Lock()
	defer svc.mux.Unlock()

	svc.index = auxIndex
}

// Performance monitoring
func (svc *Service) logDbTiming(timing time.Duration) {
	if svc.cfg.Synchronous {
		svc.logAccessSync(timing)
	} else {
		svc.logAccessAsync(timing)
	}
}

func (svc *Service) logAccessSync(timing time.Duration) {
	svc.StatLogger.Timing(timing)
}

func (svc *Service) logAccessAsync(timing time.Duration) {
	if svc.StatLogger != nil && svc.StatLogger.timingChan != nil {
		svc.StatLogger.timingChan <- timing
	}
}

func (svc *Service) logCachePerformance(hits, misses partRoles, resource, op string) {
	if svc.cfg.Synchronous {
		svc.logCachePerformanceSync(hits, misses, resource, op)
	} else {
		svc.logCachePerformanceAsync(hits, misses, resource, op)
	}
}

func (svc *Service) logCachePerformanceSync(hits, misses partRoles, resource, op string) {
	{
		rls := make([]uint64, 0, 4)

		for _, rr := range hits {
			for r := range rr {
				rls = append(rls, r)
			}
		}

		if len(rls) > 0 {
			svc.StatLogger.CacheHit(rls, resource, op)
		}
	}

	{
		rls := make([]uint64, 0, 4)

		for _, rr := range misses {
			for r := range rr {
				rls = append(rls, r)
			}
		}

		if len(rls) > 0 {
			svc.StatLogger.CacheMiss(rls, resource, op)
		}
	}
}

func (svc *Service) logCachePerformanceAsync(hits, misses partRoles, resource, op string) {
	// Hits
	if svc.StatLogger != nil && svc.StatLogger.cacheHitChan != nil {
		rls := make([]uint64, 0, 4)

		for _, rr := range hits {
			for r := range rr {
				rls = append(rls, r)
			}
		}

		if len(rls) > 0 {
			svc.StatLogger.cacheHitChan <- statsWrap{
				roles:    rls,
				resource: resource, op: op,
			}
		}
	}

	// Misses
	if svc.StatLogger != nil && svc.StatLogger.cacheMissChan != nil {
		rls := make([]uint64, 0, 4)

		for _, rr := range misses {
			for r := range rr {
				rls = append(rls, r)
			}
		}

		if len(rls) > 0 {
			svc.StatLogger.cacheMissChan <- statsWrap{
				roles:    rls,
				resource: resource, op: op,
			}
		}
	}
}

// Debugger stuff

func (svc *Service) DebuggerSetIndex(role uint64, resource string, rules ...*Rule) (err error) {
	index := &wrapperIndex{}

	index.add(role, resource, rules...)

	svc.index = index

	return
}

func (svc *Service) DebuggerAddIndex(role uint64, resource string, rules ...*Rule) (err error) {
	index := svc.index

	index.add(role, resource, rules...)

	svc.index = index

	return
}

// // // // // // // // // // // // // // // // // // // // // // // // //
// Processing n stuff

func (svc *Service) watch(ctx context.Context) {
	tck := time.NewTicker(time.Minute * 5)

	tInt := svc.cfg.IndexFlushInterval
	if tInt == 0 {
		tInt = time.Minute * 5
	}
	tTck := time.NewTicker(tInt)
	_ = tTck

	flushInt := svc.cfg.IndexFlushInterval
	if flushInt == 0 {
		flushInt = time.Minute * 30
	}
	flushTck := time.NewTicker(flushInt)
	_ = flushTck

	rexInt := svc.cfg.ReindexInterval
	if rexInt == 0 {
		rexInt = time.Minute * 30
	}
	rexTck := time.NewTicker(rexInt)
	_ = rexTck

	defer func() {
		tck.Stop()
		tTck.Stop()
		flushTck.Stop()
		rexTck.Stop()
	}()

	lg := svc.logger.Named("rbac service wrapper")

	go func() {
		for {
			select {
			case <-tck.C:
				lg.Info("tick")

			case <-rexTck.C:
				lg.Info("reindex")

				err := svc.updateWrapperIndex(ctx)
				if err != nil {
					lg.Error("reindex failed", zap.Error(err))
				}

			case <-flushTck.C:
				err := svc.cfg.FlushIndexState(ctx, svc.index.getIndexed())
				if err != nil {
					lg.Error("failed to flush the index state", zap.Error(err))
				}

			case <-ctx.Done():
				return
			}
		}
	}()
}

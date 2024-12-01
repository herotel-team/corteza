package rbac

import (
	"testing"

	"github.com/cortezaproject/corteza/server/pkg/rbac"
)

func TestGrant(t *testing.T) {
	t.Run("completely empty index", func(t *testing.T) {
		ctx,
			req,
			svc,
			storage := initState(t, 0)

		svc.Grant(ctx, &rbac.Rule{
			RoleID:    1,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		})

		// No cache update since resource not indexed
		stats := mustStats(req, svc)
		req.Len(storage.upserts, 1)
		req.Equal(uint(0), stats.CacheUpdates)
	})

	t.Run("granting existing resource", func(t *testing.T) {
		ctx,
			req,
			svc,
			storage := initState(t, 0)

		must(req, svc.DebuggerSetIndex(1, "smt/1/1/1", &rbac.Rule{
			RoleID:    1,
			Resource:  "smt/1/1/1",
			Operation: "write",
			Access:    rbac.Allow,
		}))

		svc.Grant(ctx, &rbac.Rule{
			RoleID:    1,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		})

		// Updated the index since resource indexed
		stt := mustStats(req, svc)
		req.Len(storage.upserts, 1)
		req.Equal(uint(1), stt.CacheUpdates)
	})
}

func TestCheck(t *testing.T) {
	t.Run("completely empty index", func(t *testing.T) {
		ctx,
			req,
			svc,
			storage := initState(
			t,
			0,
			svcWithRoles(rbac.CommonRole.Make(1, "")),
		)

		storage.returnRuleSearch = []*rbac.Rule{{
			RoleID:    1,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		}}

		req.True(svc.Can(sesWrap{
			identity: 1,
			roles:    []uint64{1},
			context:  ctx,
		}, "read", resWrap{resource: "smt/1/1/1"}))

		checkHitRatios(req, mustStats(req, svc), 0, 1)
	})

	t.Run("half index, half unindex", func(t *testing.T) {
		ctx,
			req,
			svc,
			storage := initState(
			t,
			0,
			svcWithRoles(rbac.CommonRole.Make(1, ""), rbac.CommonRole.Make(2, "")),
		)

		storage.returnRuleSearch = []*rbac.Rule{{
			RoleID:    2,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		}}

		must(req, svc.DebuggerSetIndex(1, "smt/1/1/1", &rbac.Rule{
			RoleID:    1,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		}))

		req.True(svc.Can(sesWrap{
			identity: 1,
			roles:    []uint64{1, 2},
			context:  ctx,
		}, "read", resWrap{resource: "smt/1/1/1"}))

		checkHitRatios(req, mustStats(req, svc), 1, 1, [][]uint64{{1}}, [][]uint64{{2}})
	})

	t.Run("all hits", func(t *testing.T) {
		ctx,
			req,
			svc,
			_ := initState(
			t,
			0,
			svcWithRoles(
				rbac.CommonRole.Make(1, ""),
				rbac.CommonRole.Make(2, ""),
			),
		)

		must(req, svc.DebuggerSetIndex(1, "smt/1/1/1", &rbac.Rule{
			RoleID:    1,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		}))

		must(req, svc.DebuggerAddIndex(2, "smt/1/1/1", &rbac.Rule{
			RoleID:    2,
			Resource:  "smt/1/1/1",
			Operation: "read",
			Access:    rbac.Allow,
		}))

		req.True(svc.Can(sesWrap{
			identity: 1,
			roles:    []uint64{1, 2},
			context:  ctx,
		}, "read", resWrap{resource: "smt/1/1/1"}))

		checkHitRatios(req, mustStats(req, svc), 1, 0, [][]uint64{{1, 2}})
	})
}

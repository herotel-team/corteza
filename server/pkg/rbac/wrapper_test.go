package rbac

import (
	"context"
	"testing"

	"github.com/cortezaproject/corteza/server/system/types"
	"github.com/stretchr/testify/require"
)

func TestRoleSegmentation(t *testing.T) {
	req := require.New(t)

	wx := &wrapperIndex{}
	w := Service{
		index: wx,
	}

	rl1 := uint64(1001)
	rl2 := uint64(2001)
	res1 := "abc/1/2/3"
	res2 := "def/1/2/3"

	wx.add(rl1, res1, &Rule{
		RoleID:    rl1,
		Resource:  res1,
		Operation: "read",
		Access:    Allow,
	})

	rls := partRoles{}
	rls[CommonRole] = map[uint64]bool{
		rl1: true,
		rl2: true,
	}

	indexed, unindexed, err := w.segmentRoles(rls, res1)
	req.NoError(err)

	req.True(indexed[CommonRole][rl1])
	req.False(indexed[CommonRole][rl2])

	req.True(unindexed[CommonRole][rl2])
	req.False(unindexed[CommonRole][rl1])

	//
	//

	indexed, unindexed, err = w.segmentRoles(rls, res2)
	req.NoError(err)

	req.False(indexed[CommonRole][rl1])
	req.False(indexed[CommonRole][rl2])

	req.True(unindexed[CommonRole][rl1])
	req.True(unindexed[CommonRole][rl2])
}

func TestRoleSegmentationEmpty(t *testing.T) {
	req := require.New(t)

	wx := &wrapperIndex{}
	w := Service{
		index: wx,
	}

	rl1 := uint64(1001)
	rl2 := uint64(2001)
	res1 := "abc/1/2/3"

	rls := partRoles{}
	rls[CommonRole] = map[uint64]bool{
		rl1: true,
		rl2: true,
	}

	_, unindexed, err := w.segmentRoles(rls, res1)
	req.NoError(err)

	req.True(unindexed[CommonRole][rl1])
	req.True(unindexed[CommonRole][rl2])
}

type (
	tRuleStore struct {
		searches []RuleFilter
	}
)

func TestPullRules(t *testing.T) {
	req := require.New(t)
	ruleS := &tRuleStore{}
	ctx := context.Background()

	wx := &Service{
		RuleStorage: ruleS,
	}

	wx.pullRules(ctx, 1, "res/1/2/3")
	req.Len(ruleS.searches, 1)
	req.Equal([]string{"res/1/2/3", "res/1/2/*", "res/1/*/*", "res/*/*/*"}, ruleS.searches[0].Resource)
	req.Equal(uint64(1), ruleS.searches[0].RoleID)

	wx.pullRules(ctx, 1, "res/1")
	req.Len(ruleS.searches, 2)
	req.Equal([]string{"res/1", "res/*"}, ruleS.searches[1].Resource)
	req.Equal(uint64(1), ruleS.searches[1].RoleID)

	wx.pullRules(ctx, 1, "res")
	req.Len(ruleS.searches, 3)
	req.Equal([]string{"res"}, ruleS.searches[2].Resource)
	req.Equal(uint64(1), ruleS.searches[2].RoleID)
}

func TestGetMatchingRule_pureIndex(t *testing.T) {
	req := require.New(t)

	stt := evaluationState{
		res: "res/1/2/3",
		op:  "read",

		unindexedRoles: partRoles{CommonRole: map[uint64]bool{}},
		indexedRoles:   partRoles{CommonRole: map[uint64]bool{1: true}},
		unindexedRules: [5]map[uint64][]*Rule{},
	}

	t.Run("1", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		wx.index.add(1, "res/1/2/3", &Rule{
			RoleID:    1,
			Resource:  "res/1/*/*",
			Operation: "read",
			Access:    Deny,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/1/2/3",
			Operation: "read",
			Access:    Inherit,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/1/2/*",
			Operation: "read",
			Access:    Allow,
		})

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Equal("res/1/2/*", auxRule.Resource)
		req.Equal(Allow, auxRule.Access)
	})

	t.Run("2", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		wx.index.add(1, "res/1/2/3", &Rule{
			RoleID:    1,
			Resource:  "res/1/*/*",
			Operation: "read",
			Access:    Deny,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/1/2/3",
			Operation: "read",
			Access:    Deny,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/1/2/*",
			Operation: "read",
			Access:    Deny,
		})

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Equal("res/1/2/3", auxRule.Resource)
		req.Equal(Deny, auxRule.Access)
	})

	t.Run("3", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		wx.index.add(1, "res/1/2/3", &Rule{
			RoleID:    1,
			Resource:  "res/1/*/*",
			Operation: "read",
			Access:    Inherit,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/*/*/*",
			Operation: "read",
			Access:    Inherit,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/1/2/3",
			Operation: "read",
			Access:    Inherit,
		}, &Rule{
			RoleID:    1,
			Resource:  "res/1/2/*",
			Operation: "read",
			Access:    Inherit,
		})

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Nil(auxRule)
	})
}

func TestGetMatchingRule_pureStored(t *testing.T) {
	req := require.New(t)

	stt := evaluationState{
		res: "res/1/2/3",
		op:  "read",

		unindexedRoles: partRoles{CommonRole: map[uint64]bool{1: true}},
		indexedRoles:   partRoles{CommonRole: map[uint64]bool{}},
		unindexedRules: [5]map[uint64][]*Rule{},
	}

	t.Run("1", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		stt.unindexedRules = [5]map[uint64][]*Rule{CommonRole: {
			1: {&Rule{
				RoleID:    1,
				Resource:  "res/1/*/*",
				Operation: "read",
				Access:    Deny,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Allow,
			}},
		}}

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Equal("res/1/2/*", auxRule.Resource)
		req.Equal(Allow, auxRule.Access)
	})

	t.Run("2", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		stt.unindexedRules = [5]map[uint64][]*Rule{CommonRole: {
			1: {&Rule{
				RoleID:    1,
				Resource:  "res/1/*/*",
				Operation: "read",
				Access:    Deny,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Deny,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Deny,
			}},
		}}

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Equal("res/1/2/3", auxRule.Resource)
		req.Equal(Deny, auxRule.Access)
	})

	t.Run("3", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		stt.unindexedRules = [5]map[uint64][]*Rule{CommonRole: {
			3: {&Rule{
				RoleID:    1,
				Resource:  "res/1/*/*",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/*/*/*",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Inherit,
			}},
		}}

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Nil(auxRule)
	})
}

func TestGetMatchingRule_mixed(t *testing.T) {
	req := require.New(t)

	stt := evaluationState{
		res: "res/1/2/3",
		op:  "read",

		unindexedRoles: partRoles{CommonRole: map[uint64]bool{1: true}},
		indexedRoles:   partRoles{CommonRole: map[uint64]bool{}},
		unindexedRules: [5]map[uint64][]*Rule{},
	}

	t.Run("1", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		stt.unindexedRules = [5]map[uint64][]*Rule{CommonRole: {
			1: {&Rule{
				RoleID:    1,
				Resource:  "res/1/*/*",
				Operation: "read",
				Access:    Deny,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Allow,
			}},
		}}

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Equal("res/1/2/*", auxRule.Resource)
		req.Equal(Allow, auxRule.Access)
	})

	t.Run("2", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		stt.unindexedRules = [5]map[uint64][]*Rule{CommonRole: {
			1: {&Rule{
				RoleID:    1,
				Resource:  "res/1/*/*",
				Operation: "read",
				Access:    Deny,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Deny,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Deny,
			}},
		}}

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Equal("res/1/2/3", auxRule.Resource)
		req.Equal(Deny, auxRule.Access)
	})

	t.Run("3", func(t *testing.T) {
		wx := &Service{
			index: &wrapperIndex{},
		}

		stt.unindexedRules = [5]map[uint64][]*Rule{CommonRole: {
			3: {&Rule{
				RoleID:    1,
				Resource:  "res/1/*/*",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/*/*/*",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Inherit,
			}, &Rule{
				RoleID:    1,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Inherit,
			}},
		}}

		auxRule := wx.getMatchingRule(stt, CommonRole, 1)
		req.Nil(auxRule)
	})
}

func TestCombiningSources(t *testing.T) {
	req := require.New(t)
	wx := &Service{
		index: &wrapperIndex{},
	}

	wx.index.add(1, "res/1/2/3", &Rule{
		RoleID:    1,
		Resource:  "res/1/2/3",
		Operation: "read",
		Access:    Inherit,
	}, &Rule{
		RoleID:    1,
		Resource:  "res/1/2/*",
		Operation: "read",
		Access:    Allow,
	}, &Rule{
		RoleID:    2,
		Resource:  "res/1/2/3",
		Operation: "read",
		Access:    Deny,
	})

	stt := evaluationState{
		res: "res/1/2/3",
		op:  "read",

		unindexedRoles: partRoles{CommonRole: map[uint64]bool{3: true}},
		indexedRoles:   partRoles{CommonRole: map[uint64]bool{1: true}},
		unindexedRules: [5]map[uint64][]*Rule{CommonRole: {
			3: {{
				RoleID:    3,
				Resource:  "res/1/2/3",
				Operation: "read",
				Access:    Inherit,
			}, {
				RoleID:    3,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Deny,
			}},
		}},
	}

	auxRule := wx.getMatchingRule(stt, CommonRole, 1)
	req.Equal("res/1/2/*", auxRule.Resource)
	req.Equal(Allow, auxRule.Access)

	auxRule = wx.getMatchingRule(stt, CommonRole, 3)
	req.Equal("res/1/2/*", auxRule.Resource)
	req.Equal(Deny, auxRule.Access)

	wx.index.add(3, "res/1/2/3", &Rule{
		RoleID:    3,
		Resource:  "res/1/2/3",
		Operation: "read",
		Access:    Inherit,
	})
	stt = evaluationState{
		res: "res/1/2/3",
		op:  "read",

		unindexedRoles: partRoles{CommonRole: map[uint64]bool{3: true}},
		indexedRoles:   partRoles{CommonRole: map[uint64]bool{1: true}},
		unindexedRules: [5]map[uint64][]*Rule{CommonRole: {
			3: {{
				RoleID:    3,
				Resource:  "res/1/2/*",
				Operation: "read",
				Access:    Deny,
			}},
		}},
	}

	auxRule = wx.getMatchingRule(stt, CommonRole, 3)
	req.Equal("res/1/2/*", auxRule.Resource)
	req.Equal(Deny, auxRule.Access)
}

func (tt *tRuleStore) SearchRbacRules(ctx context.Context, f RuleFilter) (rs RuleSet, rf RuleFilter, err error) {
	tt.searches = append(tt.searches, f)
	return
}

func (tt *tRuleStore) UpsertRbacRule(ctx context.Context, rr ...*Rule) (err error) {
	return
}

func (tt *tRuleStore) DeleteRbacRule(ctx context.Context, rr ...*Rule) (err error) {
	return
}

func (tt *tRuleStore) TruncateRbacRules(ctx context.Context) (err error) {
	return
}

func (tt *tRuleStore) SearchRoles(ctx context.Context, f types.RoleFilter) (rs types.RoleSet, rf types.RoleFilter, err error) {
	return
}

package rbac

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeKey(t *testing.T) {
	role := uint64(10)
	tcc := []struct {
		in  string
		out string
	}{{
		in:  "corteza::compose:module-field/*/*/*",
		out: fmt.Sprintf("%d:corteza::compose:module-field", role),
	}, {
		in:  "corteza::compose:module-field/1/*/*",
		out: fmt.Sprintf("%d:corteza::compose:module-field/1", role),
	}, {
		in:  "corteza::compose:module-field/1/2/*",
		out: fmt.Sprintf("%d:corteza::compose:module-field/1/2", role),
	}, {
		in:  "corteza::compose:module-field/1/2/3",
		out: fmt.Sprintf("%d:corteza::compose:module-field/1/2/3", role),
	}}

	wx := wrapperIndex{}
	for _, tc := range tcc {
		t.Run(tc.in, func(t *testing.T) {
			wx.makeKey(role, tc.in)
		})
	}
}

func TestIndexing(t *testing.T) {
	role := uint64(10)
	req := require.New(t)

	svc := wrapperIndex{}
	req.True(svc.add(role, "corteza::compose:module-field/1/2/3"))
	req.True(svc.add(role, "corteza::compose:module-field/1/4/6"))

	req.True(svc.add(role, "corteza::compose:module-field/1/*/*"))
	req.True(svc.add(role, "corteza::compose:module-field/1/4/*"))

	// False since no resource matches this wildcard
	req.False(svc.add(role, "corteza::compose:module-field/1/5/*"))
	req.False(svc.add(role, "corteza::compose:module-field/2/*/*"))

	// False since it's a completely different resource
	req.False(svc.add(role, "corteza::compose:record/1/2/*"))
}

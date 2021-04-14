package core

import (
	"reflect"
	"testing"
	"time"

	. "github.com/kraken-hpc/kraken/core"
	pb "github.com/kraken-hpc/kraken/core/proto"
	ct "github.com/kraken-hpc/kraken/core/proto/customtypes"
	"github.com/kraken-hpc/kraken/lib/types"
	uuid "github.com/satori/go.uuid"
)

// fixture builds out a semi-realistic set of mutations
// for power control with IPMI vs Redfish
func fixtureMuts() []types.StateMutation {
	return []types.StateMutation{
		NewStateMutation( // IPMI discover initial
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(pb.Node_PHYS_UNKNOWN),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("IPMI"),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_CHILD,
			time.Second*10,
			[3]string{"", "", ""},
		),
		NewStateMutation( // IPMI power on
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(pb.Node_POWER_OFF),
					reflect.ValueOf(pb.Node_POWER_ON),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("IPMI"),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_CHILD,
			time.Second*10,
			[3]string{"", "", ""},
		),
		NewStateMutation( // IPMI power off
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(pb.Node_POWER_ON),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("IPMI"),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_CHILD,
			time.Second*10,
			[3]string{"", "", ""},
		),
		NewStateMutation( // Redfish discover initial
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(pb.Node_PHYS_UNKNOWN),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("Redfish"),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_CHILD,
			time.Second*10,
			[3]string{"", "", ""},
		),
		NewStateMutation( // Redfish power on
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(pb.Node_POWER_OFF),
					reflect.ValueOf(pb.Node_POWER_ON),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("Redfish"),
			},
			map[string]reflect.Value{},
			types.StateMutationContext_CHILD,
			time.Second*10,
			[3]string{"", "", ""},
		),
		NewStateMutation( // Redfish power off, sync exclude
			map[string][2]reflect.Value{
				"/PhysState": {
					reflect.ValueOf(pb.Node_POWER_ON),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("Redfish"),
			},
			map[string]reflect.Value{
				"/RunState": reflect.ValueOf(pb.Node_SYNC),
			},
			types.StateMutationContext_CHILD,
			time.Second*10,
			[3]string{"", "", ""},
		),
	}
}

func fixtureNodes() []struct {
	node  pb.Node
	count int
} {
	id := &ct.NodeID{uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426655440000"))}
	pbs := []struct {
		node  pb.Node
		count int
	}{
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "generic/unknown",
				RunState:  pb.Node_UNKNOWN,
				PhysState: pb.Node_PHYS_UNKNOWN,
				Arch:      "",
			},
			count: 1,
		},
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "ipmi/on",
				RunState:  pb.Node_INIT,
				PhysState: pb.Node_POWER_ON,
				Arch:      "IPMI",
			},
			count: 1,
		},
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "redfish/off",
				RunState:  pb.Node_INIT,
				PhysState: pb.Node_POWER_OFF,
				Arch:      "Redfish",
			},
			count: 1,
		},
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "redfish/off/sync",
				RunState:  pb.Node_SYNC,
				PhysState: pb.Node_POWER_OFF,
				Arch:      "Redfish",
			},
			count: 1,
		},
	}
	return pbs
}

func TestStateSpec_NodeMatch(t *testing.T) {
	id := &ct.NodeID{uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426655440000"))}

	nodes := []struct {
		node  pb.Node
		match bool
	}{
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "nomatch-noexclude",
				RunState:  pb.Node_UNKNOWN,
				PhysState: pb.Node_PHYS_UNKNOWN,
			},
			match: false,
		},
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "match-noexclude",
				RunState:  pb.Node_UNKNOWN,
				PhysState: pb.Node_POWER_OFF,
			},
			match: true,
		},
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "match-exclude",
				RunState:  pb.Node_SYNC,
				PhysState: pb.Node_POWER_OFF,
			},
			match: false,
		},
		{
			node: pb.Node{
				Id:        id,
				Nodename:  "nomatch-exclude",
				RunState:  pb.Node_SYNC,
				PhysState: pb.Node_POWER_ON,
			},
			match: false,
		},
	}
	req := map[string]reflect.Value{
		"/PhysState": reflect.ValueOf(pb.Node_POWER_OFF),
	}
	exc := map[string]reflect.Value{
		"/RunState": reflect.ValueOf(pb.Node_SYNC),
	}
	s := NewStateSpec(req, exc)
	for _, v := range nodes {
		t.Run(v.node.Nodename,
			func(t *testing.T) {
				n := NewNodeFromMessage(&v.node)
				if s.NodeMatch(n) != v.match {
					t.Errorf("NodeMatch incorrect")
				} else {
					t.Logf("NodeMatch correct")
				}
			})
	}
}

func TestStateSpec_SpecCompat(t *testing.T) {
	specs := []struct {
		name   string
		req    map[string]reflect.Value
		exc    map[string]reflect.Value
		compat bool
	}{
		{
			name:   "null",
			req:    map[string]reflect.Value{},
			exc:    map[string]reflect.Value{},
			compat: true,
		},
		{
			name: "equal",
			req: map[string]reflect.Value{
				"/PhysState": reflect.ValueOf(pb.Node_POWER_OFF),
			},
			exc: map[string]reflect.Value{
				"/RunState": reflect.ValueOf(pb.Node_SYNC),
			},
			compat: true,
		},
		{
			name: "req-noexclude",
			req: map[string]reflect.Value{
				"/PhysState": reflect.ValueOf(pb.Node_POWER_OFF),
			},
			exc:    map[string]reflect.Value{},
			compat: true,
		},
		{
			name: "req-mismatch",
			req: map[string]reflect.Value{
				"/PhysState": reflect.ValueOf(pb.Node_POWER_ON),
			},
			exc:    map[string]reflect.Value{},
			compat: false,
		},
		{
			name: "excluded",
			req: map[string]reflect.Value{
				"/PhysState": reflect.ValueOf(pb.Node_POWER_OFF),
				"/RunState":  reflect.ValueOf(pb.Node_SYNC),
			},
			exc:    map[string]reflect.Value{},
			compat: false,
		},
		{
			name: "excludes",
			req: map[string]reflect.Value{
				"/RunState": reflect.ValueOf(pb.Node_INIT),
			},
			exc: map[string]reflect.Value{
				"/PhysState": reflect.ValueOf(pb.Node_POWER_OFF),
			},
			compat: false,
		},
	}
	req := map[string]reflect.Value{
		"/PhysState": reflect.ValueOf(pb.Node_POWER_OFF),
	}
	exc := map[string]reflect.Value{
		"/RunState": reflect.ValueOf(pb.Node_SYNC),
	}
	s := NewStateSpec(req, exc)
	for _, v := range specs {
		t.Run(v.name,
			func(t *testing.T) {
				if s.SpecCompat(NewStateSpec(v.req, v.exc)) != v.compat {
					t.Errorf("SpecCompat incorrect")
				} else {
					t.Logf("SpecCompat correct")
				}
			})
	}
}

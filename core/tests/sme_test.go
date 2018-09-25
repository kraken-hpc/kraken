package core

import (
	"reflect"
	"testing"

	uuid "github.com/satori/go.uuid"
	. "github.com/hpc/kraken/core"
	pb "github.com/hpc/kraken/core/proto"
	"github.com/hpc/kraken/lib"
)

// fixture builds out a semi-realistic set of mutations
// for power control with IPMI vs Redfish
func fixtureMuts() []lib.StateMutation {
	return []lib.StateMutation{
		NewStateMutation( // IPMI discover initial
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
					reflect.ValueOf(pb.Node_PHYS_UNKNOWN),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("IPMI"),
			},
			map[string]reflect.Value{},
			func(...interface{}) {},
		),
		NewStateMutation( // IPMI power on
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
					reflect.ValueOf(pb.Node_POWER_OFF),
					reflect.ValueOf(pb.Node_POWER_ON),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("IPMI"),
			},
			map[string]reflect.Value{},
			func(...interface{}) {},
		),
		NewStateMutation( // IPMI power off
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
					reflect.ValueOf(pb.Node_POWER_ON),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("IPMI"),
			},
			map[string]reflect.Value{},
			func(...interface{}) {},
		),
		NewStateMutation( // Redfish discover initial
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
					reflect.ValueOf(pb.Node_PHYS_UNKNOWN),
					reflect.ValueOf(pb.Node_POWER_OFF),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("Redfish"),
			},
			map[string]reflect.Value{},
			func(...interface{}) {},
		),
		NewStateMutation( // Redfish power on
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
					reflect.ValueOf(pb.Node_POWER_OFF),
					reflect.ValueOf(pb.Node_POWER_ON),
				},
			},
			map[string]reflect.Value{
				"/Arch": reflect.ValueOf("Redfish"),
			},
			map[string]reflect.Value{},
			func(...interface{}) {},
		),
		NewStateMutation( // Redfish power off, sync exclude
			map[string][2]reflect.Value{
				"/PhysState": [2]reflect.Value{
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
			func(...interface{}) {},
		),
	}
}

func fixtureNodes() []struct {
	node  pb.Node
	count int
} {
	id := uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426655440000"))
	bid, _ := id.MarshalBinary()
	pbs := []struct {
		node  pb.Node
		count int
	}{
		{
			node: pb.Node{
				Id:        bid,
				Nodename:  "generic/unknown",
				RunState:  pb.Node_UNKNOWN,
				PhysState: pb.Node_PHYS_UNKNOWN,
				Arch:      "",
			},
			count: 1,
		},
		{
			node: pb.Node{
				Id:        bid,
				Nodename:  "ipmi/on",
				RunState:  pb.Node_INIT,
				PhysState: pb.Node_POWER_ON,
				Arch:      "IPMI",
			},
			count: 1,
		},
		{
			node: pb.Node{
				Id:        bid,
				Nodename:  "redfish/off",
				RunState:  pb.Node_INIT,
				PhysState: pb.Node_POWER_OFF,
				Arch:      "Redfish",
			},
			count: 1,
		},
		{
			node: pb.Node{
				Id:        bid,
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
	id := uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426655440000"))
	bid, _ := id.MarshalBinary()

	nodes := []struct {
		node  pb.Node
		match bool
	}{
		{
			node: pb.Node{
				Id:        bid,
				Nodename:  "nomatch-noexclude",
				RunState:  pb.Node_UNKNOWN,
				PhysState: pb.Node_PHYS_UNKNOWN,
			},
			match: false,
		},
		{
			node: pb.Node{
				Id:        bid,
				Nodename:  "match-noexclude",
				RunState:  pb.Node_UNKNOWN,
				PhysState: pb.Node_POWER_OFF,
			},
			match: true,
		},
		{
			node: pb.Node{
				Id:        bid,
				Nodename:  "match-exclude",
				RunState:  pb.Node_SYNC,
				PhysState: pb.Node_POWER_OFF,
			},
			match: false,
		},
		{
			node: pb.Node{
				Id:        bid,
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
	n := NewNode()
	for _, v := range nodes {
		t.Run(v.node.Nodename,
			func(t *testing.T) {
				n.SetMessage(&v.node)
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

func TestStateMutationEngine(t *testing.T) {
	sme := NewStateMutationEngine(NewStateSpec(map[string]reflect.Value{"/PhysState": reflect.ValueOf(pb.Node_PHYS_UNKNOWN)}, map[string]reflect.Value{}))
	sme.RegisterMutations(fixtureMuts())
	sme.DumpGraph()
	nodes := fixtureNodes()
	t.Run("NodeMatch", func(t *testing.T) {
		n := NewNode()
		for _, p := range nodes {
			t.Run(p.node.Nodename, func(t *testing.T) {
				n.SetMessage(&p.node)
				c := sme.NodeMatch(n)
				if c != p.count {
					t.Errorf("NodeMatch failed: %d != %d", c, p.count)
				} else {
					t.Logf("NodeMatch succeeded: %d == %d", c, p.count)
				}
			})
		}
	})

	t.Run("PathExists", func(t *testing.T) {
		// 0 1 -> true
		// 0 2 -> true
		// 1 2 -> false
		start := NewNode()
		end := NewNode()
		t.Run("UNKNOWN -> ON (IPMI)", func(t *testing.T) {
			start.SetMessage(&nodes[0].node)
			end.SetMessage(&nodes[1].node)
			b, e := sme.PathExists(start, end)
			if b {
				t.Log("existing path exists")
			} else {
				t.Errorf("existing path doesn't exist: %s", e.Error())
			}
		})
		t.Run("ON (IPMI) -> OFF (Redfish)", func(t *testing.T) {
			start.SetMessage(&nodes[1].node)
			end.SetMessage(&nodes[2].node)
			b, e := sme.PathExists(start, end)
			if b {
				t.Errorf("non-existent path exists")
			} else {
				t.Logf("non-existing path doesn't exist: %s", e.Error())
			}
		})
	})
}

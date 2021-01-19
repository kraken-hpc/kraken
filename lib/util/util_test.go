//go:generate protoc -I ../../core/proto/src -I . --gogo_out=grpc:. test.proto

package util

import (
	"reflect"
	"testing"
)

var n1 = &Fixture{
	Boolean:  true,
	UInt:     42,
	Slice:    []string{"this", "is", "a", "test"},
	Map:      map[string]uint32{"one": 1, "two": 2, "three": 3},
	Sub:      &SubFixture{A: "A", B: "B"},
	SliceSub: []*SubFixture{{A: "A", B: "B"}, {A: "C", B: "D"}},
	MapSub: map[uint32]*SubFixture{
		7: {
			A: "lucky",
			B: "prime",
		},
		13: {
			A: "unlucky",
			B: "also prime",
		},
		42: {
			A: "meaning",
			B: "life",
		},
	},
}
var n2 = &Fixture{
	Boolean:  true,
	UInt:     43,
	Slice:    []string{"this", "is", "the", "test"},
	Map:      map[string]uint32{"one": 1, "two": 3, "four": 4},
	Sub:      &SubFixture{A: "A", B: "C"},
	SliceSub: []*SubFixture{{A: "A", B: "D"}, {A: "C", B: "D"}},
	MapSub: map[uint32]*SubFixture{
		7: {
			A: "lucky",
			B: "prime",
		},
		13: {
			A: "very unlucky",
			B: "also prime",
		},
		11: {
			A: "also lucky",
			B: "also prime",
		},
	},
}

func TestDiff(t *testing.T) {

	d, e := MessageDiff(n1, n2, "")
	if e != nil {
		t.Error(e)
	}
	t.Logf("diff: %v", d)
	s := []string{
		"/UInt",
		"/Slice/2",
		"/Map/two",
		"/Map/three",
		"/Map/four",
		"/Sub/B",
		"/SliceSub/0/B",
		"/MapSub/13/A",
		"/MapSub/42",
		"/MapSub/11",
	}
	if !reflect.DeepEqual(d, s) {
		t.Error("Incorrect diff")
	}
}

func TestResolveURL(t *testing.T) {
	// do some lookups on n1
	tests := [][2]string{
		{"/UInt", "42"},
		{"/Slice/2", "a"},
		{"/Map/three", "3"},
		{"/Sub/B", "B"},
		{"/SliceSub/0/A", "A"},
		{"/MapSub/13/A", "unlucky"},
	}
	for _, test := range tests {
		test := test
		t.Run(test[0], func(t *testing.T) {
			v, e := ResolveURL(test[0], reflect.ValueOf(n1))
			if e != nil {
				t.Errorf("Unexected error: %v", e)
			}
			if ValueToString(v) != test[1] {
				t.Errorf("Result mismatch: %s != %s", ValueToString(v), test[1])
			}
		})
	}
}

func TestResolveOrMakeURL(t *testing.T) {
	// make more tests
	nv := reflect.ValueOf(n1)
	v, e := ResolveOrMakeURL("/MapSub/99/A", nv)
	if e != nil {
		t.Errorf("unexpected ResolvOrMakeURL failure: %v", e)
		return
	}
	v.SetString("testing")
	v2, e := ResolveURL("/MapSub/99/A", nv)
	if e != nil {
		t.Errorf("unexpected ResolveURL failure: %v", e)
		return
	}
	if v2.String() != v.String() {
		t.Errorf("second lookup failed: %s != %s", v2.String(), v.String())
	}
}

func TestURLShift(t *testing.T) {
	type test struct {
		url  string
		root string
		sub  string
	}
	tests := []test{
		{
			url:  "This/is/a/test",
			root: "This",
			sub:  "is/a/test",
		},
		{
			url:  "/This/is/a/test",
			root: "This",
			sub:  "is/a/test",
		},
		{
			url:  "",
			root: "",
			sub:  "",
		},
		{
			url:  "test",
			root: "test",
			sub:  "",
		},
		{
			url:  "test/",
			root: "test",
			sub:  "",
		},
	}
	for _, v := range tests {
		t.Run(v.url,
			func(t *testing.T) {
				root, sub := URLShift(v.url)
				t.Logf("url: %s, root: %s, sub: %s", v.url, root, sub)
				if root != v.root {
					t.Errorf("url: %s, root: %s != %s", v.url, root, v.root)
				}
				if sub != v.sub {
					t.Errorf("url: %s, sub: %s != %s", v.url, sub, v.sub)
				}
			})
	}
}

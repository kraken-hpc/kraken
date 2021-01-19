//go:generate protoc -I ../../core/proto/src -I . --gogo_out=grpc:. test.proto

package util

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	n1 := &Fixture{
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
	n2 := &Fixture{
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

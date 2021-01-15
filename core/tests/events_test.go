package core

import (
	"fmt"

	. "github.com/hpc/kraken/core"
	"github.com/hpc/kraken/lib/types"
)

// TestEventDispatchEngine tests that NewEventDispatchEngine initializes correctly
/* FIXME: broken test
func TestEventDispatchEngine(t *testing.T) {
	ed := NewEventDispatchEngine()
	if ed == nil {
		t.Fatal("failed to create EventDispatchEngine")
	}
	t.Run("valid echan", func(t *testing.T) {
		echan := ed.EventChan()
		if echan == nil {
			t.Fatal("echan was not succesfully created")
		}
	})
	t.Run("valid schan", func(t *testing.T) {
		schan := ed.SubscriptionChan()
		if schan == nil {
			t.Fatal("schan was not succesfully created")
		}
	})

}
*/

// ExampleFilterSimple shows how to use a filter generator
func ExampleFilterSimple() {
	list := []string{
		"/this/is",
		"/a/test",
	}
	el := NewEventListener("example",
		types.Event_STATE_CHANGE,
		func(ev types.Event) bool {
			return FilterSimple(ev, list)
		},
		nil)

	ev1 := NewEvent(
		types.Event_STATE_CHANGE,
		"/this/is",
		nil)
	ev2 := NewEvent(
		types.Event_STATE_CHANGE,
		"/blatz",
		nil)
	fmt.Printf("%s -> %v\n", ev1.URL(), el.Filter(ev1))
	fmt.Printf("%s -> %v\n", ev2.URL(), el.Filter(ev2))
	// Output:
	// /this/is -> true
	// /blatz -> false
}

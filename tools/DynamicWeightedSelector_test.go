package tools

import (
	"fmt"
	"testing"
)

func TestGetKey(t *testing.T) {
	keys := []WeightedKey{
		{"server1", 1},
		// {"server2", 1},
		// {"server3", 1},
		// {"server4", 1},
		// {"server5", 2},
	}
	selector := NewDynamicWeightedSelector(keys)

	for i := 0; i < 10000; i++ {
		selector.Select()
		// selected := selector.Select()
		// fmt.Println(selected)
	}
	// selector.SetWeight("server1", 0)
	// selector.RemoveKey("server1")
	for i := 0; i < 10000; i++ {
		selected := selector.Select()
		fmt.Println(selected)
	}
}

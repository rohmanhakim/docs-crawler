package frontier_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
)

func TestAddContains(t *testing.T) {
	set := frontier.NewSet[MySetItem]()
	size := set.Size()
	if size != 0 {
		t.Errorf("expected empty, got: %d", size)
	}

	set.Add(MySetItem{
		name:   "First Item",
		number: 1,
	})
	size = set.Size()
	if size != 1 {
		t.Errorf("expected size 1, got: %d", size)
	}

	// change value
	set.Add(MySetItem{
		name:   "First Item",
		number: 0,
	})
	size = set.Size()
	if size != 2 {
		t.Errorf("expected size 2, got: %d", size)
	}

	// add similar item
	set.Add(MySetItem{
		name:   "First Item",
		number: 0,
	})
	size = set.Size()
	if size != 2 {
		t.Errorf("expected size 2, got: %d", size)
	}
}

func TestAddRemove(t *testing.T) {
	set := frontier.NewSet[MySetItem]()
	size := set.Size()
	if size != 0 {
		t.Errorf("expected empty, got: %d", size)
	}

	firstItem := MySetItem{
		name:   "First Item",
		number: 1,
	}
	set.Add(firstItem)
	size = set.Size()
	if size != 1 {
		t.Errorf("expected size 1, got: %d", size)
	}

	secondItem := MySetItem{
		name:   "Second Item",
		number: 2,
	}
	// remove item that does not added yet
	set.Remove(secondItem)
	size = set.Size()
	if size != 1 {
		t.Errorf("expected size 1, got: %d", size)
	}

	// remove first item
	set.Remove(firstItem)
	size = set.Size()
	if size != 0 {
		t.Errorf("expected size 0, got: %d", size)
	}
}

func TestAddClear(t *testing.T) {
	set := frontier.NewSet[MySetItem]()
	size := set.Size()
	if size != 0 {
		t.Errorf("expected empty, got: %d", size)
	}

	firstItem := MySetItem{
		name:   "First Item",
		number: 1,
	}
	set.Add(firstItem)
	size = set.Size()
	if size != 1 {
		t.Errorf("expected size 1, got: %d", size)
	}

	secondItem := MySetItem{
		name:   "Second Item",
		number: 2,
	}
	set.Add(secondItem)
	size = set.Size()
	if size != 2 {
		t.Errorf("expected size 2, got: %d", size)
	}

	// clear all items
	set.Clear()
	size = set.Size()
	if size != 0 {
		t.Errorf("expected size 0, got: %d", size)
	}
}

type MySetItem struct {
	name   string
	number int
}

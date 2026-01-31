package frontier_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/frontier"
)

func TestEnqueueDequeue(t *testing.T) {
	queue := frontier.NewFIFOQueue[MyQueueItem]()

	firstItem := MyQueueItem{
		name: "First item",
	}

	secondItem := MyQueueItem{
		name: "Second item",
	}

	thirdItem := MyQueueItem{
		name: "Third item",
	}

	size := queue.Size()
	if size != 0 {
		t.Errorf("should have zero size, got: %d", size)
	}

	queue.Enqueue(firstItem)
	queue.Enqueue(secondItem)
	queue.Enqueue(thirdItem)

	size = queue.Size()
	if size != 3 {
		t.Errorf("should have size 3, got: %d", size)
	}

	output, ok := queue.Dequeue()
	if !ok {
		t.Error("should return ok")
	}
	if output != firstItem {
		t.Errorf("should dequeue %v, got: %v", firstItem, output)
	}

	size = queue.Size()
	if size != 2 {
		t.Errorf("should have size 2, got: %d", size)
	}

	output, ok = queue.Dequeue()
	if !ok {
		t.Error("should return ok")
	}
	if output != secondItem {
		t.Errorf("should dequeue %v, got: %v", secondItem, output)
	}

	size = queue.Size()
	if size != 1 {
		t.Errorf("should have size 1, got: %d", size)
	}

	output, ok = queue.Dequeue()
	if !ok {
		t.Error("should return ok")
	}
	if output != thirdItem {
		t.Errorf("should dequeue %v, got: %v", thirdItem, output)
	}

	size = queue.Size()
	if size != 0 {
		t.Errorf("should have zero size, got: %d", size)
	}

	output, ok = queue.Dequeue()
	if ok {
		t.Error("should not return ok")
	}
}

type MyQueueItem struct {
	name string
}

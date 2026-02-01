package frontier

// type QueueItem interface{}

type FIFOQueue[T any] []T

func NewFIFOQueue[T any]() *FIFOQueue[T] {
	return &FIFOQueue[T]{}
}

func (f *FIFOQueue[T]) Enqueue(item T) {
	*f = append(*f, item)
}

// return false on the second returned values if queue is empty
func (f *FIFOQueue[T]) Dequeue() (T, bool) {
	var zero T
	if len(*f) == 0 {
		return zero, false
	}
	first := (*f)[0]
	*f = (*f)[1:]
	return first, true
}

func (f *FIFOQueue[T]) Size() int {
	return len(*f)
}

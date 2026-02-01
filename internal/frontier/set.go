package frontier

type Set[T comparable] map[T]struct{}

func NewSet[T comparable]() Set[T] {
	return make(Set[T])
}

func (s Set[T]) Add(item T) {
	// a lightweight way to represent a placeholder or a signal in Go,
	// leveraging the fact that it consumes no memory.
	s[item] = struct{}{}
}

func (s Set[T]) Contains(item T) bool {
	// optional second return value when getting a value from a map
	// indicates if the key was present in the map
	_, exists := s[item]
	return exists
}

func (s Set[T]) Remove(element T) {
	delete(s, element)
}

func (s Set[T]) Clear() {
	clear(s)
}

func (s Set[T]) Size() int {
	return len(s)
}

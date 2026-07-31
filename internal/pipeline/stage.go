package pipeline

type Stage[T any, A any] interface {
	Enable() bool
	Process(input T) Result[A]
}

// type Step[T any] interface {
// 	Do(name string, task func() (T, error)) (T, error)
// }

type Journal interface {
	Load(name string) (any, bool)
	Save(name string, result any)
}

// This is what the runtime looks like conceptually
type StepRunner struct {
	journal Journal
}

func Do[T any](s *StepRunner, name string, task func() (T, error)) (T, error) {
	if result, ok := s.journal.Load(name); ok {
		return result.(T), nil // replay
	}
	result, err := task()
	if err != nil {
		return *new(T), err
	}
	s.journal.Save(name, result)
	return result, nil
}

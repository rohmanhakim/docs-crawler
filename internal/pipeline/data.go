package pipeline

type PipelineConfig struct {
}

type PipelineResult struct {
}

type Input[T any] struct {
	input   T
	options []any
}

type Result[O any] struct {
	output O
	err    StageError
}

// Decompose returns the result as a tuple (output, error).
// This provides idiomatic Go error handling for traditionalists:
//
//	output, err := Process().Decompose()
//	if err != nil {
//	    // handle error
//	}
func (r Result[O]) Decompose() (O, error) {
	return r.output, r.err
}

type StageError error

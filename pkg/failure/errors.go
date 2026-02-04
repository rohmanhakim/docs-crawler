package failure

type Severity string

// scheduler control flow
const (
	SeverityFatal       = "fatal"
	SeverityRecoverable = "recoverable"
)

type ClassifiedError interface {
	error
	Severity() Severity
}

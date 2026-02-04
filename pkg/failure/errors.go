package failure

type Severity int

// scheduler control flow
const (
	SeverityFatal Severity = iota
	SeverityRecoverable
)

type ClassifiedError interface {
	error
	Severity() Severity
}

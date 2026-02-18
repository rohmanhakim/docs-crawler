package failure

// RetryPolicy defines automatic retry behavior
// This controls whether retry.Retry() will attempt exponential backoff
type RetryPolicy int

const (
	RetryPolicyAuto   RetryPolicy = iota // Retry immediately with exponential backoff
	RetryPolicyManual                    // Do not auto-retry, but eligible for manual retry queue
	RetryPolicyNever                     // Permanent failure, do not track for retry
)

// ImpactLevel defines how the scheduler should respond
// This controls processing lifecycle decisions
type ImpactLevel int

const (
	ImpactLevelContinue ImpactLevel = iota // Continue to next item (default)
	ImpactLevelAbort                       // Abort entire operation (systemic failure)
)

// Severity provides observability and legacy compatibility
type Severity string

const (
	SeverityOK             Severity = "ok"
	SeverityRecoverable    Severity = "recoverable"
	SeverityFatal          Severity = "fatal"
	SeverityRetryExhausted Severity = "retry_exhausted" // Signals manual retry needed
)

// ClassifiedError is the primary error interface for the entire pipeline
type ClassifiedError interface {
	error

	// RetryPolicy controls automatic retry behavior
	// Used by retry handler
	RetryPolicy() RetryPolicy

	// Impact controls processing continuation/abortion
	// Used by scheduler
	Impact() ImpactLevel

	// Severity provides observability and legacy compatibility
	// Used by: metadata recording, logging, monitoring
	Severity() Severity
}

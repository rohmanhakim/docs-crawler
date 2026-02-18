package failurejournal

import "time"

// Stage represents a pipeline stage that can fail and be retried.
type Stage string

const (
	StageFetch   Stage = "fetch"
	StageAsset   Stage = "asset"
	StageStorage Stage = "storage"
)

// FailureRecord represents a single recoverable failure.
type FailureRecord struct {
	URL        string    `json:"url"`
	Stage      Stage     `json:"stage"`
	Error      string    `json:"error"`
	RetryCount int       `json:"retry_count"`
	Timestamp  time.Time `json:"timestamp"`
}

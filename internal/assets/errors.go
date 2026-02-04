package assets

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type AssetsErrorCause string

const (
	ErrCauseImageDownloadFailure = "failed to download image"
)

type AssetsError struct {
	Message   string
	Retryable bool
	Cause     AssetsErrorCause
}

func (e *AssetsError) Error() string {
	return fmt.Sprintf("assets error: %s", e.Cause)
}

func (e *AssetsError) Severity() failure.Severity {
	if e.Retryable {
		return failure.SeverityRecoverable
	}
	return failure.SeverityFatal
}

// mapAssetsErrorToMetadataCause maps assets-local error semantics
// to the canonical metadata.ErrorCause table.
//
// This mapping is observational only and MUST NOT be used
// to derive control-flow decisions.
func mapAssetsErrorToMetadataCause(err AssetsError) metadata.ErrorCause {
	switch err.Cause {
	case ErrCauseImageDownloadFailure:
		return metadata.CauseNetworkFailure
	default:
		return metadata.CauseUnknown
	}
}

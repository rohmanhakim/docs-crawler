package assets

import (
	"fmt"

	"github.com/rohmanhakim/docs-crawler/internal/metadata"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
)

type AssetsErrorCause string

const (
	ErrCausePathError             = "path error"
	ErrCauseDiskFull              = "disk is full"
	ErrCauseWriteFailure          = "write failed"
	ErrCauseTimeout               = "timeout"
	ErrCauseRequestTooMany        = "too many requests"
	ErrCauseNetworkFailure        = "network issues"
	ErrCauseRepeated403           = "repeated 403s"
	ErrCauseReadResponseBodyError = "failed to read response body"
	ErrCauseContentTypeInvalid    = "non-HTML content"
	ErrCauseRedirectLimitExceeded = "reached redirect limit"
	ErrCauseRequestPageForbidden  = "forbidden"
	ErrCauseRequest5xx            = "5xx"
	ErrCauseAssetTooLarge         = "asset too large"
)

type AssetsError struct {
	Message   string
	Retryable bool
	Cause     AssetsErrorCause
}

func (e *AssetsError) Error() string {
	return fmt.Sprintf("assets error: %s, message: %s", e.Cause, e.Message)
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
	case ErrCausePathError:
		return metadata.CauseStorageFailure
	case ErrCauseDiskFull:
		return metadata.CauseStorageFailure
	case ErrCauseWriteFailure:
		return metadata.CauseStorageFailure
	case ErrCauseTimeout:
		return metadata.CauseNetworkFailure
	case ErrCauseRequestTooMany:
		return metadata.CausePolicyDisallow
	case ErrCauseRepeated403:
		return metadata.CausePolicyDisallow
	case ErrCauseReadResponseBodyError:
		return metadata.CauseContentInvalid
	case ErrCauseContentTypeInvalid:
		return metadata.CauseContentInvalid
	case ErrCauseRedirectLimitExceeded:
		return metadata.CauseUnknown
	case ErrCauseRequestPageForbidden:
		return metadata.CausePolicyDisallow
	case ErrCauseRequest5xx:
		return metadata.CauseUnknown
	default:
		return metadata.CauseUnknown
	}
}

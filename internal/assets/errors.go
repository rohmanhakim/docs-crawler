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
	ErrCauseHashError             = "hash error"
)

// assetsErrorClassifications provides explicit retry policy and impact level
// for each AssetsErrorCause. This replaces the old Retryable boolean field
// with explicit two-dimensional classification.
var assetsErrorClassifications = map[AssetsErrorCause]struct {
	Policy failure.RetryPolicy
	Impact failure.ImpactLevel
}{
	ErrCauseTimeout:               {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseNetworkFailure:        {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseReadResponseBodyError: {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseRequest5xx:            {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseRequestTooMany:        {failure.RetryPolicyAuto, failure.ImpactLevelContinue},
	ErrCauseRequestPageForbidden:  {failure.RetryPolicyManual, failure.ImpactLevelContinue},
	ErrCauseDiskFull:              {failure.RetryPolicyManual, failure.ImpactLevelContinue},
	ErrCauseRepeated403:           {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseAssetTooLarge:         {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseRedirectLimitExceeded: {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseContentTypeInvalid:    {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseWriteFailure:          {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCausePathError:             {failure.RetryPolicyNever, failure.ImpactLevelContinue},
	ErrCauseHashError:             {failure.RetryPolicyNever, failure.ImpactLevelContinue},
}

// AssetsError represents an error that occurred during asset resolution.
// It implements failure.ClassifiedError interface with explicit retry policy
// and impact level based on the error cause.
type AssetsError struct {
	Message string
	Cause   AssetsErrorCause
	policy  failure.RetryPolicy
	impact  failure.ImpactLevel
}

// NewAssetsError creates a new AssetsError with explicit classification based on cause.
// The retry policy and crawl impact are determined by the error cause classification map.
func NewAssetsError(cause AssetsErrorCause, message string) *AssetsError {
	classification := assetsErrorClassifications[cause]
	return &AssetsError{
		Message: message,
		Cause:   cause,
		policy:  classification.Policy,
		impact:  classification.Impact,
	}
}

func (e *AssetsError) Error() string {
	return fmt.Sprintf("assets error: %s, message: %s", e.Cause, e.Message)
}

func (e *AssetsError) Severity() failure.Severity {
	if e.impact == failure.ImpactLevelAbort {
		return failure.SeverityFatal
	}
	switch e.policy {
	case failure.RetryPolicyAuto:
		return failure.SeverityRecoverable
	case failure.RetryPolicyManual:
		return failure.SeverityRetryExhausted
	case failure.RetryPolicyNever:
		return failure.SeverityRecoverable
	default:
		return failure.SeverityRecoverable
	}
}

// RetryPolicy returns the automatic retry behavior for this error.
// This is now explicitly set based on the error cause, not derived from a boolean.
func (e *AssetsError) RetryPolicy() failure.RetryPolicy {
	return e.policy
}

// Impact returns how the scheduler should respond to this error.
// Asset errors never abort the crawl - they are per-URL failures.
func (e *AssetsError) Impact() failure.ImpactLevel {
	return e.impact
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
	case ErrCauseAssetTooLarge:
		return metadata.CausePolicyDisallow
	case ErrCauseHashError:
		return metadata.CauseContentInvalid
	default:
		return metadata.CauseUnknown
	}
}

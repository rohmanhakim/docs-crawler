package scheduler_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/assets"
	"github.com/rohmanhakim/docs-crawler/internal/mdconvert"
	"github.com/rohmanhakim/docs-crawler/pkg/failure"
	"github.com/rohmanhakim/docs-crawler/pkg/retry"
	"github.com/stretchr/testify/mock"
)

// resolverMock is a testify mock for the assets.Resolver
type resolverMock struct {
	mock.Mock
}

// Resolve mocks the Resolve method
func (r *resolverMock) Resolve(
	ctx context.Context,
	pageUrl url.URL,
	host string,
	scheme string,
	conversionResult mdconvert.ConversionResult,
	resolveParam assets.ResolveParam,
	retryParam retry.RetryParam,
) (assets.AssetfulMarkdownDoc, failure.ClassifiedError) {
	args := r.Called(ctx, pageUrl, host, scheme, conversionResult, resolveParam, retryParam)
	doc := args.Get(0).(assets.AssetfulMarkdownDoc)
	var err failure.ClassifiedError
	if args.Get(1) != nil {
		err = args.Get(1).(failure.ClassifiedError)
	}
	return doc, err
}

// newResolverMockForTest creates a properly configured resolver mock for tests
func newResolverMockForTest(t *testing.T) *resolverMock {
	t.Helper()
	m := new(resolverMock)
	return m
}

// setupResolverMockWithSuccess sets up the resolver mock to return a successful result
func setupResolverMockWithSuccess(m *resolverMock) {
	m.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assets.AssetfulMarkdownDoc{}, nil)
}

// setupResolverMockWithError sets up the resolver mock to return an error
func setupResolverMockWithError(m *resolverMock, err failure.ClassifiedError) {
	m.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assets.AssetfulMarkdownDoc{}, err)
}

// setupResolverMockWithFatalError sets up the resolver mock to return a fatal error
func setupResolverMockWithFatalError(m *resolverMock) {
	resolverErr := &assets.AssetsError{
		Message:   "fatal asset error: disk full",
		Retryable: false,
		Cause:     assets.ErrCauseDiskFull,
	}
	m.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assets.AssetfulMarkdownDoc{}, resolverErr)
}

// setupResolverMockWithRecoverableError sets up the resolver mock to return a recoverable error
func setupResolverMockWithRecoverableError(m *resolverMock) {
	resolverErr := &assets.AssetsError{
		Message:   "recoverable asset error: network timeout",
		Retryable: true,
		Cause:     assets.ErrCauseNetworkFailure,
	}
	m.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(assets.AssetfulMarkdownDoc{}, resolverErr)
}

// setupResolverMockWithCustomResult sets up the resolver mock to return a custom result
func setupResolverMockWithCustomResult(m *resolverMock, doc assets.AssetfulMarkdownDoc) {
	m.On("Resolve", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(doc, nil)
}

// createAssetfulMarkdownDocForTest creates an AssetfulMarkdownDoc for testing
func createAssetfulMarkdownDocForTest(content string, localAssets []string) assets.AssetfulMarkdownDoc {
	if localAssets == nil {
		localAssets = []string{}
	}
	return assets.NewAssetfulMarkdownDoc(
		[]byte(content),
		[]url.URL{},
		[]string{},
		localAssets,
	)
}

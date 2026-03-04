package scheduler_test

import (
	"context"
	"testing"
	"time"

	ratelimiter "github.com/rohmanhakim/rate-limiter"
	"github.com/stretchr/testify/mock"
)

// rateLimiterMock is a testify mock for the RateLimiter interface from
// github.com/rohmanhakim/rate-limiter
type rateLimiterMock struct {
	mock.Mock
}

// newRateLimiterMockForTest creates a properly configured rate limiter mock for crawl tests
func newRateLimiterMockForTest(t *testing.T) *rateLimiterMock {
	t.Helper()
	m := new(rateLimiterMock)
	// Set up default expectations for crawl tests
	m.On("SetBaseDelay", mock.Anything).Return()
	m.On("SetJitter", mock.Anything).Return()
	m.On("SetResourceDelay", mock.Anything, mock.Anything).Return()
	m.On("Backoff", mock.Anything, mock.Anything).Return()
	m.On("ResetBackoff", mock.Anything).Return()
	m.On("Wait", mock.Anything, mock.Anything).Return(nil)
	return m
}

func (m *rateLimiterMock) SetBaseDelay(baseDelay time.Duration) {
	m.Called(baseDelay)
}

func (m *rateLimiterMock) SetJitter(jitter time.Duration) {
	m.Called(jitter)
}

func (m *rateLimiterMock) SetResourceDelay(resource string, delay time.Duration) {
	m.Called(resource, delay)
}

func (m *rateLimiterMock) Backoff(ctx context.Context, resource string, opts ...ratelimiter.BackoffOptions) {
	m.Called(ctx, resource)
}

func (m *rateLimiterMock) ResetBackoff(resource string) {
	m.Called(resource)
}

func (m *rateLimiterMock) Wait(ctx context.Context, resource string) error {
	args := m.Called(ctx, resource)
	return args.Error(0)
}

func (m *rateLimiterMock) ResolveDelay(ctx context.Context, resource string) time.Duration {
	args := m.Called(ctx, resource)
	return args.Get(0).(time.Duration)
}

func (m *rateLimiterMock) SetDebugLogger(logger ratelimiter.DebugLogger) {
	m.Called(logger)
}

package scheduler_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// rateLimiterMock is a testify mock for the RateLimiter
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
	m.On("SetRandomSeed", mock.Anything).Return()
	m.On("SetCrawlDelay", mock.Anything, mock.Anything).Return()
	m.On("Backoff", mock.Anything).Return()
	m.On("ResetBackoff", mock.Anything).Return()
	m.On("ResolveDelay", mock.Anything).Return(time.Duration(0))
	return m
}

func (m *rateLimiterMock) SetBaseDelay(baseDelay time.Duration) {
	m.Called(baseDelay)
}

func (m *rateLimiterMock) SetJitter(jitter time.Duration) {
	m.Called(jitter)
}

func (m *rateLimiterMock) SetRandomSeed(randomSeed int64) {
	m.Called(randomSeed)
}

func (m *rateLimiterMock) SetCrawlDelay(host string, delay time.Duration) {
	m.Called(host, delay)
}

func (m *rateLimiterMock) Backoff(host string) {
	m.Called(host)
}

func (m *rateLimiterMock) ResetBackoff(host string) {
	m.Called(host)
}

func (m *rateLimiterMock) MarkLastFetchAsNow(host string) {
	m.Called(host)
}

func (m *rateLimiterMock) Jitter(base time.Duration) time.Duration {
	args := m.Called(base)
	return args.Get(0).(time.Duration)
}

func (m *rateLimiterMock) SetRNG(rng interface{}) {
	m.Called(rng)
}

func (m *rateLimiterMock) ResolveDelay(host string) time.Duration {
	args := m.Called(host)
	return args.Get(0).(time.Duration)
}

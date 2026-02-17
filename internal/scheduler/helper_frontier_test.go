package scheduler_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/config"
	"github.com/rohmanhakim/docs-crawler/internal/frontier"
	"github.com/stretchr/testify/mock"
)

type frontierMock struct {
	mock.Mock
	// Fields to track submissions and enqueues for automatic Dequeue behavior
	submittedCandidates []frontier.CrawlAdmissionCandidate
	enqueuedTokens      []frontier.CrawlToken
	// disableAutoEnqueue prevents automatic enqueueing of tokens on Submit()
	// This allows tests to explicitly control Dequeue return values via OnDequeue()
	disableAutoEnqueue bool
}

func (f *frontierMock) Init(cfg config.Config) {
	// Note: We don't require f.Called() here to avoid panic when no expectation is set
	// Tests that need to verify Init was called can set up explicit expectations
}

func (f *frontierMock) Submit(admission frontier.CrawlAdmissionCandidate) {
	// Track submitted candidate for later use (always do this)
	f.submittedCandidates = append(f.submittedCandidates, admission)
	// Simulate the real frontier: after submitting, a token is enqueued
	// Unless disableAutoEnqueue is set (for tests that need explicit Dequeue control)
	if !f.disableAutoEnqueue {
		token := frontier.NewCrawlToken(
			admission.TargetURL(),
			admission.DiscoveryMetadata().Depth(),
		)
		f.enqueuedTokens = append(f.enqueuedTokens, token)
	}
	// Note: We don't call f.Called() here to avoid requiring mock expectations
	// Tests that need to verify Submit was called can use explicit expectations
	// by setting f.disableAutoEnqueue = true and using On("Submit")
}

func (f *frontierMock) Enqueue(incomingToken frontier.CrawlToken) {
	// Track enqueued token for automatic Dequeue
	f.enqueuedTokens = append(f.enqueuedTokens, incomingToken)
	// Note: We don't call f.Called() here to avoid requiring mock expectations
}

func (f *frontierMock) IsDepthExhausted(depth int) bool {
	args := f.Called(depth)

	value, ok := args.Get(0).(bool)
	if !ok {
		return false
	}

	return value
}

func (f *frontierMock) CurrentMinDepth() int {
	args := f.Called()

	value, ok := args.Get(0).(int)
	if !ok {
		return -1
	}

	return value
}

func (f *frontierMock) VisitedCount() int {
	// Return the actual count of submitted candidates
	// This makes the mock behave like the real frontier
	return len(f.submittedCandidates)
}

func (f *frontierMock) Dequeue() (frontier.CrawlToken, bool) {
	// When auto-enqueue is disabled, use mock expectations
	if f.disableAutoEnqueue {
		args := f.Called()
		token, _ := args.Get(0).(frontier.CrawlToken)
		ok := args.Bool(1)
		return token, ok
	}

	// Normal FIFO behavior when auto-enqueue is enabled
	if len(f.enqueuedTokens) > 0 {
		token := f.enqueuedTokens[0]
		f.enqueuedTokens = f.enqueuedTokens[1:]
		return token, true
	}

	// No tokens available - return empty
	// Tests should use Enqueue() or set disableAutoEnqueue=false to get tokens
	return frontier.CrawlToken{}, false
}

// OnDequeue sets up the mock to return a specific token and ok value when Dequeue is called.
// The returned mock.Call can be chained with Once(), Times(n), etc.
func (f *frontierMock) OnDequeue(token frontier.CrawlToken, ok bool) *mock.Call {
	return f.On("Dequeue").Return(token, ok)
}

// SetupDequeueToReturn seeds the mock to return token, ok for the next Dequeue call.
// This is a convenience wrapper around OnDequeue().Once().
func (f *frontierMock) SetupDequeueToReturn(token frontier.CrawlToken, ok bool) {
	f.OnDequeue(token, ok).Once()
}

func newFrontierMockForTest(t *testing.T) *frontierMock {
	t.Helper()
	m := new(frontierMock)
	// Default to enabling auto-enqueue for backward compatibility
	// Tests that need explicit Dequeue control can set disableAutoEnqueue = true
	m.disableAutoEnqueue = false
	return m
}

package scheduler_test

import (
	"net/url"
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/robots"
	"github.com/stretchr/testify/mock"
)

type robotsMock struct {
	mock.Mock
}

func (r *robotsMock) Init(userAgent string) {
	r.Called(userAgent)
}

func (r *robotsMock) Decide(targetURL url.URL) (robots.Decision, *robots.RobotsError) {
	args := r.Called(targetURL)

	// Extract decision
	decision, ok := args.Get(0).(robots.Decision)
	if !ok {
		// Return default decision if type assertion fails
		return robots.Decision{}, nil
	}

	// Ensure the decision contains the target URL if not already set
	if decision.Url == (url.URL{}) {
		decision.Url = targetURL
	}

	// Extract error (may be nil)
	err, _ := args.Get(1).(*robots.RobotsError)
	return decision, err
}

// OnDecide sets up the mock to return a specific decision and error for any URL.
// Use mock.Anything for targetURL to match any URL.
// It returns the mock.Call so you can chain .Once(), .Times(n), etc.
func (r *robotsMock) OnDecide(targetURL interface{}, decision robots.Decision, err *robots.RobotsError) *mock.Call {
	return r.On("Decide", targetURL).Return(decision, err)
}

// NewRobotsMockForTest creates a properly configured robots mock for tests.
// It does NOT set any default expectation - tests must call OnDecide to configure behavior.
func NewRobotsMockForTest(t *testing.T) *robotsMock {
	t.Helper()
	m := new(robotsMock)
	// Do NOT set any default expectation here - tests must be explicit
	return m
}

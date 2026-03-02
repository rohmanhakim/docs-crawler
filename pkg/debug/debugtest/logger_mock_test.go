package debugtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/debug"
)

func TestLoggerMock_Enabled(t *testing.T) {
	mock := NewLoggerMock()
	if !mock.Enabled() {
		t.Error("NewLoggerMock().Enabled() should return true by default")
	}

	mock.SetEnabled(false)
	if mock.Enabled() {
		t.Error("Enabled() should return false after SetEnabled(false)")
	}

	mock.SetEnabled(true)
	if !mock.Enabled() {
		t.Error("Enabled() should return true after SetEnabled(true)")
	}
}

func TestLoggerMock_LogStage(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	// Log a stage event
	mock.LogStage(ctx, "fetcher", debug.StageEvent{
		Type: debug.EventTypeStart,
		URL:  "https://example.com",
	})

	if !mock.LogStageCalled {
		t.Error("LogStageCalled should be true")
	}

	if len(mock.StageEntries) != 1 {
		t.Fatalf("expected 1 stage entry, got %d", len(mock.StageEntries))
	}

	entry := mock.StageEntries[0]
	if entry.Stage != "fetcher" {
		t.Errorf("stage = %q, want %q", entry.Stage, "fetcher")
	}
	if entry.Event.Type != debug.EventTypeStart {
		t.Errorf("event type = %q, want %q", entry.Event.Type, debug.EventTypeStart)
	}
	if entry.Event.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", entry.Event.URL, "https://example.com")
	}
}

func TestLoggerMock_LogRetry(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()
	testErr := errors.New("timeout")

	mock.LogRetry(ctx, 2, 10, 100*time.Millisecond, testErr, debug.FieldMap{"url": "https://example.com", "stage": "fetcher"})

	if !mock.LogRetryCalled {
		t.Error("LogRetryCalled should be true")
	}

	if len(mock.RetryEntries) != 1 {
		t.Fatalf("expected 1 retry entry, got %d", len(mock.RetryEntries))
	}

	entry := mock.RetryEntries[0]
	if entry.Attempt != 2 {
		t.Errorf("attempt = %d, want 2", entry.Attempt)
	}
	if entry.MaxAttempts != 10 {
		t.Errorf("maxAttempts = %d, want 10", entry.MaxAttempts)
	}
	if entry.Backoff != 100*time.Millisecond {
		t.Errorf("backoff = %v, want 100ms", entry.Backoff)
	}
	if entry.Err != testErr {
		t.Errorf("err = %v, want %v", entry.Err, testErr)
	}
}

func TestLoggerMock_LogRateLimit(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	mock.LogRateLimit(ctx, "example.com", 1500*time.Millisecond, debug.RateLimitReasonBaseDelay)

	if !mock.LogRateLimitCalled {
		t.Error("LogRateLimitCalled should be true")
	}

	if len(mock.RateLimitEntries) != 1 {
		t.Fatalf("expected 1 rate limit entry, got %d", len(mock.RateLimitEntries))
	}

	entry := mock.RateLimitEntries[0]
	if entry.Host != "example.com" {
		t.Errorf("host = %q, want %q", entry.Host, "example.com")
	}
	if entry.Delay != 1500*time.Millisecond {
		t.Errorf("delay = %v, want 1500ms", entry.Delay)
	}
	if entry.Reason != debug.RateLimitReasonBaseDelay {
		t.Errorf("reason = %q, want %q", entry.Reason, debug.RateLimitReasonBaseDelay)
	}
}

func TestLoggerMock_LogStep(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	mock.LogStep(ctx, "fetcher", "http_request", debug.FieldMap{
		"method": "GET",
		"url":    "https://example.com",
	})

	if !mock.LogStepCalled {
		t.Error("LogStepCalled should be true")
	}

	if len(mock.StepEntries) != 1 {
		t.Fatalf("expected 1 step entry, got %d", len(mock.StepEntries))
	}

	entry := mock.StepEntries[0]
	if entry.Stage != "fetcher" {
		t.Errorf("stage = %q, want %q", entry.Stage, "fetcher")
	}
	if entry.Step != "http_request" {
		t.Errorf("step = %q, want %q", entry.Step, "http_request")
	}
	if entry.Fields["method"] != "GET" {
		t.Errorf("method = %v, want GET", entry.Fields["method"])
	}
}

func TestLoggerMock_LogError(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()
	testErr := errors.New("connection refused")

	mock.LogError(ctx, "fetcher", testErr, debug.FieldMap{"retry_count": 3})

	if !mock.LogErrorCalled {
		t.Error("LogErrorCalled should be true")
	}

	if len(mock.ErrorEntries) != 1 {
		t.Fatalf("expected 1 error entry, got %d", len(mock.ErrorEntries))
	}

	entry := mock.ErrorEntries[0]
	if entry.Stage != "fetcher" {
		t.Errorf("stage = %q, want %q", entry.Stage, "fetcher")
	}
	if entry.Err != testErr {
		t.Errorf("err = %v, want %v", entry.Err, testErr)
	}
	if entry.Fields["retry_count"] != 3 {
		t.Errorf("retry_count = %v, want 3", entry.Fields["retry_count"])
	}
}

func TestLoggerMock_WithFields(t *testing.T) {
	mock := NewLoggerMock()

	childLogger := mock.WithFields(debug.FieldMap{"crawl_id": "crawl-123"})

	if !mock.WithFieldsCalled {
		t.Error("WithFieldsCalled should be true")
	}

	if len(mock.WithFieldsRecords) != 1 {
		t.Fatalf("expected 1 with fields record, got %d", len(mock.WithFieldsRecords))
	}

	if mock.WithFieldsRecords[0]["crawl_id"] != "crawl-123" {
		t.Errorf("crawl_id = %v, want crawl-123", mock.WithFieldsRecords[0]["crawl_id"])
	}

	// Verify child logger is also a LoggerMock
	if childLogger == nil {
		t.Fatal("WithFields should return non-nil logger")
	}
}

func TestLoggerMock_Close(t *testing.T) {
	mock := NewLoggerMock()

	err := mock.Close()

	if err != nil {
		t.Errorf("Close() should return nil, got %v", err)
	}

	if !mock.CloseCalled {
		t.Error("CloseCalled should be true")
	}
}

func TestLoggerMock_Reset(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	// Record some events
	mock.LogStage(ctx, "fetcher", debug.StageEvent{Type: debug.EventTypeStart})
	mock.LogRetry(ctx, 1, 10, 100*time.Millisecond, errors.New("test"), nil)
	mock.LogRateLimit(ctx, "example.com", 1*time.Second, debug.RateLimitReasonBaseDelay)
	mock.LogStep(ctx, "fetcher", "test", nil)
	mock.LogError(ctx, "fetcher", errors.New("test"), nil)
	mock.WithFields(debug.FieldMap{"key": "value"})
	mock.Close()

	// Reset
	mock.Reset()

	// Verify all state is cleared
	if mock.LogStageCalled {
		t.Error("LogStageCalled should be false after reset")
	}
	if mock.LogRetryCalled {
		t.Error("LogRetryCalled should be false after reset")
	}
	if mock.LogRateLimitCalled {
		t.Error("LogRateLimitCalled should be false after reset")
	}
	if mock.LogStepCalled {
		t.Error("LogStepCalled should be false after reset")
	}
	if mock.LogErrorCalled {
		t.Error("LogErrorCalled should be false after reset")
	}
	if mock.WithFieldsCalled {
		t.Error("WithFieldsCalled should be false after reset")
	}
	if mock.CloseCalled {
		t.Error("CloseCalled should be false after reset")
	}
	if len(mock.StageEntries) != 0 {
		t.Errorf("StageEntries should be empty, got %d", len(mock.StageEntries))
	}
	if !mock.enabled {
		t.Error("enabled should be true after reset")
	}
}

func TestLoggerMock_LastMethods(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	// Test Last* methods return nil when empty
	if mock.LastStage() != nil {
		t.Error("LastStage() should return nil when empty")
	}
	if mock.LastRetry() != nil {
		t.Error("LastRetry() should return nil when empty")
	}
	if mock.LastRateLimit() != nil {
		t.Error("LastRateLimit() should return nil when empty")
	}
	if mock.LastStep() != nil {
		t.Error("LastStep() should return nil when empty")
	}
	if mock.LastError() != nil {
		t.Error("LastError() should return nil when empty")
	}

	// Add multiple entries
	mock.LogStage(ctx, "fetcher", debug.StageEvent{Type: debug.EventTypeStart})
	mock.LogStage(ctx, "extractor", debug.StageEvent{Type: debug.EventTypeStart})

	mock.LogRetry(ctx, 1, 10, 100*time.Millisecond, nil, nil)
	mock.LogRetry(ctx, 2, 10, 200*time.Millisecond, nil, nil)

	mock.LogRateLimit(ctx, "a.com", 1*time.Second, debug.RateLimitReasonBaseDelay)
	mock.LogRateLimit(ctx, "b.com", 2*time.Second, debug.RateLimitReasonCrawlDelay)

	mock.LogStep(ctx, "fetcher", "step1", nil)
	mock.LogStep(ctx, "fetcher", "step2", nil)

	mock.LogError(ctx, "fetcher", errors.New("err1"), nil)
	mock.LogError(ctx, "extractor", errors.New("err2"), nil)

	// Verify Last* returns most recent
	if mock.LastStage().Stage != "extractor" {
		t.Errorf("LastStage().Stage = %q, want %q", mock.LastStage().Stage, "extractor")
	}
	if mock.LastRetry().Attempt != 2 {
		t.Errorf("LastRetry().Attempt = %d, want 2", mock.LastRetry().Attempt)
	}
	if mock.LastRateLimit().Host != "b.com" {
		t.Errorf("LastRateLimit().Host = %q, want %q", mock.LastRateLimit().Host, "b.com")
	}
	if mock.LastStep().Step != "step2" {
		t.Errorf("LastStep().Step = %q, want %q", mock.LastStep().Step, "step2")
	}
	if mock.LastError().Stage != "extractor" {
		t.Errorf("LastError().Stage = %q, want %q", mock.LastError().Stage, "extractor")
	}
}

func TestLoggerMock_GetMethods(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	// Add entries
	mock.LogStage(ctx, "fetcher", debug.StageEvent{Type: debug.EventTypeStart})
	mock.LogRetry(ctx, 1, 10, 100*time.Millisecond, nil, nil)
	mock.LogRateLimit(ctx, "a.com", 1*time.Second, debug.RateLimitReasonBaseDelay)
	mock.LogStep(ctx, "fetcher", "step1", nil)
	mock.LogError(ctx, "fetcher", errors.New("err"), nil)

	// Get methods should return copies (safe to modify)
	stages := mock.GetStageEntries()
	_ = stages // Verify we can access without panic

	retries := mock.GetRetryEntries()
	_ = retries

	rateLimits := mock.GetRateLimitEntries()
	_ = rateLimits

	steps := mock.GetStepEntries()
	_ = steps

	errors := mock.GetErrorEntries()
	_ = errors
}

func TestLoggerMock_StagesByType(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	mock.LogStage(ctx, "fetcher", debug.StageEvent{Type: debug.EventTypeStart})
	mock.LogStage(ctx, "fetcher", debug.StageEvent{Type: debug.EventTypeComplete})
	mock.LogStage(ctx, "extractor", debug.StageEvent{Type: debug.EventTypeStart})
	mock.LogStage(ctx, "extractor", debug.StageEvent{Type: debug.EventTypeError})

	startEntries := mock.StagesByType(debug.EventTypeStart)
	if len(startEntries) != 2 {
		t.Errorf("StagesByType(start) = %d entries, want 2", len(startEntries))
	}

	completeEntries := mock.StagesByType(debug.EventTypeComplete)
	if len(completeEntries) != 1 {
		t.Errorf("StagesByType(complete) = %d entries, want 1", len(completeEntries))
	}

	errorEntries := mock.StagesByType(debug.EventTypeError)
	if len(errorEntries) != 1 {
		t.Errorf("StagesByType(error) = %d entries, want 1", len(errorEntries))
	}
}

func TestLoggerMock_StepsByStage(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	mock.LogStep(ctx, "fetcher", "step1", nil)
	mock.LogStep(ctx, "fetcher", "step2", nil)
	mock.LogStep(ctx, "extractor", "step3", nil)

	fetcherSteps := mock.StepsByStage("fetcher")
	if len(fetcherSteps) != 2 {
		t.Errorf("StepsByStage(fetcher) = %d entries, want 2", len(fetcherSteps))
	}

	extractorSteps := mock.StepsByStage("extractor")
	if len(extractorSteps) != 1 {
		t.Errorf("StepsByStage(extractor) = %d entries, want 1", len(extractorSteps))
	}
}

func TestLoggerMock_StepsByName(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	mock.LogStep(ctx, "fetcher", "http_request", nil)
	mock.LogStep(ctx, "fetcher", "body_read", nil)
	mock.LogStep(ctx, "extractor", "http_request", nil)

	httpSteps := mock.StepsByName("http_request")
	if len(httpSteps) != 2 {
		t.Errorf("StepsByName(http_request) = %d entries, want 2", len(httpSteps))
	}
}

func TestLoggerMock_TotalCalls(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()

	if mock.TotalCalls() != 0 {
		t.Errorf("TotalCalls() = %d, want 0", mock.TotalCalls())
	}

	mock.LogStage(ctx, "fetcher", debug.StageEvent{})
	mock.LogRetry(ctx, 1, 10, 0, nil, nil)
	mock.LogRateLimit(ctx, "a.com", 0, debug.RateLimitReasonBaseDelay)
	mock.LogStep(ctx, "fetcher", "step", nil)
	mock.LogError(ctx, "fetcher", errors.New("err"), nil)

	if mock.TotalCalls() != 5 {
		t.Errorf("TotalCalls() = %d, want 5", mock.TotalCalls())
	}
}

func TestLoggerMock_Interface(t *testing.T) {
	// Compile-time check that LoggerMock implements debug.DebugLogger
	var _ debug.DebugLogger = (*LoggerMock)(nil)
}

func TestLoggerMock_ConcurrentAccess(t *testing.T) {
	mock := NewLoggerMock()
	ctx := context.Background()
	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				mock.LogStage(ctx, "stage", debug.StageEvent{
					URL: "https://example.com",
				})
				mock.LogStep(ctx, "stage", "step", debug.FieldMap{
					"id": id,
				})
				_ = mock.Enabled()
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = mock.TotalCalls()
				_ = mock.LastStage()
				_ = mock.GetStageEntries()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}

	// Verify all writes were recorded
	if mock.TotalCalls() != 2000 {
		t.Errorf("TotalCalls() = %d, want 2000", mock.TotalCalls())
	}
}

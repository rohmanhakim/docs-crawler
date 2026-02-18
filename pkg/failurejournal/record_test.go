package failurejournal

import (
	"testing"
	"time"
)

func TestStageConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      Stage
		expected Stage
	}{
		{"StageFetch", StageFetch, Stage("fetch")},
		{"StageAsset", StageAsset, Stage("asset")},
		{"StageStorage", StageStorage, Stage("storage")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

func TestFailureRecord_Creation(t *testing.T) {
	now := time.Now()
	record := FailureRecord{
		URL:        "http://example.com/page",
		Stage:      StageFetch,
		Error:      "connection refused",
		RetryCount: 3,
		Timestamp:  now,
	}

	if record.URL != "http://example.com/page" {
		t.Errorf("URL = %q, want %q", record.URL, "http://example.com/page")
	}
	if record.Stage != StageFetch {
		t.Errorf("Stage = %q, want %q", record.Stage, StageFetch)
	}
	if record.Error != "connection refused" {
		t.Errorf("Error = %q, want %q", record.Error, "connection refused")
	}
	if record.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want %d", record.RetryCount, 3)
	}
	if record.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", record.Timestamp, now)
	}
}

func TestFailureRecord_AllStages(t *testing.T) {
	stages := []Stage{StageFetch, StageAsset, StageStorage}
	urls := []string{"http://fetch.com", "http://asset.com", "http://storage.com"}

	for i, stage := range stages {
		record := FailureRecord{
			URL:        urls[i],
			Stage:      stage,
			Error:      "test error",
			RetryCount: 1,
			Timestamp:  time.Now(),
		}

		if record.Stage != stage {
			t.Errorf("Stage = %q, want %q", record.Stage, stage)
		}
	}
}

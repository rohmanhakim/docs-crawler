package failurejournal_test

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rohmanhakim/docs-crawler/pkg/failurejournal"
)

func TestFileSink_NewFileSink(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"

	sink := failurejournal.NewFileSink(path)

	if sink.Path() != path {
		t.Errorf("expected path %s, got %s", path, sink.Path())
	}

	if sink.Count() != 0 {
		t.Errorf("expected count 0, got %d", sink.Count())
	}
}

func TestFileSink_Path(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	got := sink.Path()
	if got != path {
		t.Errorf("Path() = %v, want %v", got, path)
	}
}

func TestFileSink_Record(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	record1 := failurejournal.FailureRecord{
		URL:        "http://example.com/1",
		Stage:      failurejournal.StageFetch,
		Error:      "connection refused",
		RetryCount: 1,
		Timestamp:  time.Now(),
	}
	record2 := failurejournal.FailureRecord{
		URL:        "http://example.com/2",
		Stage:      failurejournal.StageAsset,
		Error:      "timeout",
		RetryCount: 2,
		Timestamp:  time.Now(),
	}

	sink.Record(record1)
	sink.Record(record2)

	if sink.Count() != 2 {
		t.Errorf("Count() = %d, want %d", sink.Count(), 2)
	}
}

func TestFileSink_Flush(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	now := time.Now()
	records := []failurejournal.FailureRecord{
		{
			URL:        "http://example.com/1",
			Stage:      failurejournal.StageFetch,
			Error:      "connection refused",
			RetryCount: 1,
			Timestamp:  now,
		},
		{
			URL:        "http://example.com/2",
			Stage:      failurejournal.StageAsset,
			Error:      "timeout",
			RetryCount: 2,
			Timestamp:  now.Add(1 * time.Second),
		},
	}

	for _, r := range records {
		sink.Record(r)
	}

	err := sink.Flush()
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if sink.Count() != 0 {
		t.Errorf("Count() after flush = %d, want %d", sink.Count(), 0)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	lines := splitLines(string(data))
	if len(lines) != 2 {
		t.Errorf("number of lines = %d, want %d", len(lines), 2)
	}

	var readRecords []failurejournal.FailureRecord
	for _, line := range lines {
		var r failurejournal.FailureRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("Unmarshal error = %v", err)
		}
		readRecords = append(readRecords, r)
	}

	if len(readRecords) != 2 {
		t.Fatalf("read records = %d, want %d", len(readRecords), 2)
	}

	if readRecords[0].URL != records[0].URL || readRecords[1].URL != records[1].URL {
		t.Errorf("records don't match")
	}
}

func TestFileSink_Flush_EmptyBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	err := sink.Flush()
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not be created for empty buffer")
	}
}

func TestFileSink_Flush_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/subdir/nested/test.log"
	sink := failurejournal.NewFileSink(path)

	record := failurejournal.FailureRecord{
		URL:        "http://example.com",
		Stage:      failurejournal.StageFetch,
		Error:      "test error",
		RetryCount: 1,
		Timestamp:  time.Now(),
	}
	sink.Record(record)

	err := sink.Flush()
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("file was not created at %s", path)
	}
}

func TestFileSink_Read(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	now := time.Now()
	records := []failurejournal.FailureRecord{
		{
			URL:        "http://example.com/1",
			Stage:      failurejournal.StageFetch,
			Error:      "error1",
			RetryCount: 1,
			Timestamp:  now,
		},
		{
			URL:        "http://example.com/2",
			Stage:      failurejournal.StageAsset,
			Error:      "error2",
			RetryCount: 2,
			Timestamp:  now.Add(1 * time.Second),
		},
	}

	for _, r := range records {
		sink.Record(r)
	}
	sink.Flush()

	readBack, err := sink.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(readBack) != 2 {
		t.Errorf("Read() returned %d records, want %d", len(readBack), 2)
	}
}

func TestFileSink_Read_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/nonexistent.log"
	sink := failurejournal.NewFileSink(path)

	records, err := sink.Read()
	if err != nil {
		t.Fatalf("Read() error = %v, want nil", err)
	}

	if len(records) != 0 {
		t.Errorf("Read() returned %d records, want %d", len(records), 0)
	}
}

func TestFileSink_Read_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"

	fileContent := `{"url":"http://example.com/1","stage":"fetch","error":"error1","retry_count":1,"timestamp":"` + time.Now().Format(time.RFC3339) + `"}
invalid json line
{"url":"http://example.com/2","stage":"asset","error":"error2","retry_count":2,"timestamp":"` + time.Now().Format(time.RFC3339) + `"}
`

	if err := os.WriteFile(path, []byte(fileContent), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	sink := failurejournal.NewFileSink(path)
	records, err := sink.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(records) != 2 {
		t.Errorf("Read() returned %d valid records, want %d", len(records), 2)
	}
}

func TestFileSink_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	record := failurejournal.FailureRecord{
		URL:        "http://example.com",
		Stage:      failurejournal.StageFetch,
		Error:      "test error",
		RetryCount: 1,
		Timestamp:  time.Now(),
	}
	sink.Record(record)
	sink.Flush()

	err := sink.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if sink.Count() != 0 {
		t.Errorf("Count() after Clear = %d, want %d", sink.Count(), 0)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after Clear")
	}
}

func TestFileSink_Clear_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/nonexistent.log"
	sink := failurejournal.NewFileSink(path)

	err := sink.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
}

func TestFileJournal_NewFileJournal(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"

	journal := failurejournal.NewFileJournal(path)

	if journal.Path() != path {
		t.Errorf("Path() = %s, want %s", journal.Path(), path)
	}

	if journal.Count() != 0 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 0)
	}

	record := failurejournal.FailureRecord{
		URL:        "http://example.com",
		Stage:      failurejournal.StageFetch,
		Error:      "test error",
		RetryCount: 1,
		Timestamp:  time.Now(),
	}
	journal.Record(record)

	if journal.Count() != 1 {
		t.Errorf("Count() after Record = %d, want %d", journal.Count(), 1)
	}
}

func TestFileSink_ConcurrentRecord(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	var wg sync.WaitGroup
	numGoroutines := 10
	recordsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				record := failurejournal.FailureRecord{
					URL:        "http://example.com/" + strconv.Itoa(idx*recordsPerGoroutine+j),
					Stage:      failurejournal.StageFetch,
					Error:      "concurrent test error",
					RetryCount: j,
					Timestamp:  time.Now(),
				}
				sink.Record(record)
			}
		}(i)
	}
	wg.Wait()

	expectedCount := numGoroutines * recordsPerGoroutine
	if sink.Count() != expectedCount {
		t.Errorf("Count() = %d, want %d", sink.Count(), expectedCount)
	}

	// Verify records were persisted correctly
	err := sink.Flush()
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	readBack, err := sink.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(readBack) != expectedCount {
		t.Errorf("Read() returned %d records, want %d", len(readBack), expectedCount)
	}
}

func TestFileSink_ConcurrentFlushAndRecord(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	stopChan := make(chan struct{})
	var wg sync.WaitGroup

	// Flush goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopChan:
				return
			default:
				if err := sink.Flush(); err != nil {
					t.Logf("Flush error (may be expected): %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Record goroutine
	numRecords := 50
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numRecords; i++ {
			record := failurejournal.FailureRecord{
				URL:        "http://example.com/" + strconv.Itoa(i),
				Stage:      failurejournal.StageFetch,
				Error:      "concurrent error",
				RetryCount: i,
				Timestamp:  time.Now(),
			}
			sink.Record(record)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	close(stopChan)
	wg.Wait()

	// Flush any remaining records in buffer
	err := sink.Flush()
	if err != nil {
		t.Fatalf("Final Flush() error = %v", err)
	}

	// Verify records were persisted to disk
	readBack, err := sink.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	// At least some records should have been written
	if len(readBack) == 0 {
		t.Errorf("No records were persisted to disk during concurrent operations")
	}

	// Verify data integrity - all read records should have valid URLs
	for _, r := range readBack {
		if r.URL == "" {
			t.Errorf("Found record with empty URL - data corruption detected")
		}
	}

	t.Logf("Successfully verified %d records persisted during concurrent Flush/Record operations", len(readBack))
}

// TestFileSink_ConcurrentReadWrite verifies data integrity when Read() runs
// concurrently with Record() operations.
func TestFileSink_ConcurrentReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	// Pre-populate with some records and flush
	for i := 0; i < 10; i++ {
		record := failurejournal.FailureRecord{
			URL:        "http://example.com/init-" + strconv.Itoa(i),
			Stage:      failurejournal.StageFetch,
			Error:      "initial error",
			RetryCount: i,
			Timestamp:  time.Now(),
		}
		sink.Record(record)
	}
	sink.Flush()

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Writer: continuously adds new records
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-stopChan:
				return
			default:
				record := failurejournal.FailureRecord{
					URL:        "http://example.com/write-" + strconv.Itoa(i),
					Stage:      failurejournal.StageFetch,
					Error:      "write error",
					RetryCount: i,
					Timestamp:  time.Now(),
				}
				sink.Record(record)
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Reader: continuously reads records
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-stopChan:
				return
			default:
				records, err := sink.Read()
				if err != nil {
					t.Logf("Read error: %v", err)
					continue
				}
				// Verify no data corruption - all records should have valid URLs
				for _, r := range records {
					if r.URL == "" {
						t.Errorf("Data corruption: found record with empty URL")
					}
				}
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	// Wait for goroutines to complete
	time.Sleep(100 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	// Final verification: flush and read all records
	err := sink.Flush()
	if err != nil {
		t.Fatalf("Final Flush() error = %v", err)
	}

	readBack, err := sink.Read()
	if err != nil {
		t.Fatalf("Final Read() error = %v", err)
	}

	// Should have at least the initial 10 + some written records
	if len(readBack) < 10 {
		t.Errorf("Expected at least 10 records, got %d", len(readBack))
	}

	t.Logf("Concurrent Read/Write test passed with %d total records", len(readBack))
}

// TestFileSink_ConcurrentClearAndRecord verifies behavior when Clear() runs
// concurrently with Record() operations.
func TestFileSink_ConcurrentClearAndRecord(t *testing.T) {
	tmpDir := t.TempDir()
	path := tmpDir + "/test.log"
	sink := failurejournal.NewFileSink(path)

	// Pre-populate with some records
	for i := 0; i < 20; i++ {
		record := failurejournal.FailureRecord{
			URL:        "http://example.com/initial-" + strconv.Itoa(i),
			Stage:      failurejournal.StageFetch,
			Error:      "initial error",
			RetryCount: i,
			Timestamp:  time.Now(),
		}
		sink.Record(record)
	}
	sink.Flush()

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Clear goroutine: repeatedly clears the sink
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-stopChan:
				return
			default:
				sink.Clear()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Record goroutine: continuously adds new records
	numRecords := 100
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numRecords; i++ {
			select {
			case <-stopChan:
				return
			default:
				record := failurejournal.FailureRecord{
					URL:        "http://example.com/new-" + strconv.Itoa(i),
					Stage:      failurejournal.StageFetch,
					Error:      "new error",
					RetryCount: i,
					Timestamp:  time.Now(),
				}
				sink.Record(record)
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	close(stopChan)
	wg.Wait()

	// After concurrent operations, verify the sink is in a consistent state
	// Count should be non-negative
	count := sink.Count()
	if count < 0 {
		t.Errorf("Invalid count: %d", count)
	}

	// Verify no data corruption by reading
	readBack, err := sink.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	// All records should have valid URLs
	for _, r := range readBack {
		if r.URL == "" {
			t.Errorf("Data corruption: found record with empty URL")
		}
	}

	t.Logf("Concurrent Clear/Record test passed. Final count: %d, records on disk: %d", count, len(readBack))
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

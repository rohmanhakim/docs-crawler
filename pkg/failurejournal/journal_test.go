package failurejournal

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNewInMemoryJournal(t *testing.T) {
	journal := NewInMemoryJournal()

	if journal == nil {
		t.Fatal("NewInMemoryJournal returned nil")
	}

	if journal.Count() != 0 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 0)
	}

	if journal.Path() != "" {
		t.Errorf("Path() = %q, want %q", journal.Path(), "")
	}
}

func TestInMemoryJournal_Record(t *testing.T) {
	journal := NewInMemoryJournal()

	record := FailureRecord{
		URL:        "http://example.com/page1",
		Stage:      StageFetch,
		Error:      "connection refused",
		RetryCount: 1,
		Timestamp:  time.Now(),
	}

	journal.Record(record)

	if journal.Count() != 1 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 1)
	}
}

func TestInMemoryJournal_Record_Multiple(t *testing.T) {
	journal := NewInMemoryJournal()

	records := []FailureRecord{
		{
			URL:        "http://example.com/page1",
			Stage:      StageFetch,
			Error:      "error1",
			RetryCount: 1,
			Timestamp:  time.Now(),
		},
		{
			URL:        "http://example.com/page2",
			Stage:      StageAsset,
			Error:      "error2",
			RetryCount: 2,
			Timestamp:  time.Now(),
		},
		{
			URL:        "http://example.com/page3",
			Stage:      StageStorage,
			Error:      "error3",
			RetryCount: 3,
			Timestamp:  time.Now(),
		},
	}

	for _, r := range records {
		journal.Record(r)
	}

	if journal.Count() != 3 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 3)
	}
}

func TestInMemoryJournal_Flush(t *testing.T) {
	journal := NewInMemoryJournal()

	// Flush should be a no-op and return nil
	err := journal.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v, want nil", err)
	}

	// Flush after recording should also work
	journal.Record(FailureRecord{
		URL:        "http://example.com/page",
		Stage:      StageFetch,
		Error:      "error",
		RetryCount: 1,
		Timestamp:  time.Now(),
	})

	err = journal.Flush()
	if err != nil {
		t.Errorf("Flush() error after Record = %v, want nil", err)
	}
}

func TestInMemoryJournal_Path(t *testing.T) {
	journal := NewInMemoryJournal()

	path := journal.Path()
	if path != "" {
		t.Errorf("Path() = %q, want %q", path, "")
	}
}

func TestInMemoryJournal_Read(t *testing.T) {
	journal := NewInMemoryJournal()

	records := []FailureRecord{
		{
			URL:        "http://example.com/page1",
			Stage:      StageFetch,
			Error:      "error1",
			RetryCount: 1,
			Timestamp:  time.Now(),
		},
		{
			URL:        "http://example.com/page2",
			Stage:      StageAsset,
			Error:      "error2",
			RetryCount: 2,
			Timestamp:  time.Now(),
		},
	}

	for _, r := range records {
		journal.Record(r)
	}

	readRecords, err := journal.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(readRecords) != 2 {
		t.Errorf("Read() returned %d records, want %d", len(readRecords), 2)
	}

	// Verify content
	if readRecords[0].URL != records[0].URL {
		t.Errorf("URL = %q, want %q", readRecords[0].URL, records[0].URL)
	}
	if readRecords[1].URL != records[1].URL {
		t.Errorf("URL = %q, want %q", readRecords[1].URL, records[1].URL)
	}
}

func TestInMemoryJournal_Read_ReturnsCopy(t *testing.T) {
	journal := NewInMemoryJournal()

	record := FailureRecord{
		URL:        "http://example.com/page",
		Stage:      StageFetch,
		Error:      "error",
		RetryCount: 1,
		Timestamp:  time.Now(),
	}
	journal.Record(record)

	// Read returns a copy
	readRecords, _ := journal.Read()

	// Modify the returned slice - should not affect internal state
	readRecords[0].URL = "http://modified.com"

	// Read again - should return original
	readRecords2, _ := journal.Read()
	if readRecords2[0].URL != "http://example.com/page" {
		t.Errorf("Read() did not return a copy, got %q", readRecords2[0].URL)
	}
}

func TestInMemoryJournal_Read_Empty(t *testing.T) {
	journal := NewInMemoryJournal()

	readRecords, err := journal.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(readRecords) != 0 {
		t.Errorf("Read() returned %d records, want %d", len(readRecords), 0)
	}
}

func TestInMemoryJournal_Clear(t *testing.T) {
	journal := NewInMemoryJournal()

	// Add some records
	journal.Record(FailureRecord{
		URL:        "http://example.com/page1",
		Stage:      StageFetch,
		Error:      "error1",
		RetryCount: 1,
		Timestamp:  time.Now(),
	})
	journal.Record(FailureRecord{
		URL:        "http://example.com/page2",
		Stage:      StageAsset,
		Error:      "error2",
		RetryCount: 2,
		Timestamp:  time.Now(),
	})

	if journal.Count() != 2 {
		t.Fatalf("Count() = %d, want %d", journal.Count(), 2)
	}

	// Clear
	err := journal.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if journal.Count() != 0 {
		t.Errorf("Count() after Clear = %d, want %d", journal.Count(), 0)
	}
}

func TestInMemoryJournal_Clear_Empty(t *testing.T) {
	journal := NewInMemoryJournal()

	// Clear empty journal should not error
	err := journal.Clear()
	if err != nil {
		t.Fatalf("Clear() error on empty journal = %v", err)
	}

	if journal.Count() != 0 {
		t.Errorf("Count() after Clear = %d, want %d", journal.Count(), 0)
	}
}

func TestInMemoryJournal_Count(t *testing.T) {
	journal := NewInMemoryJournal()

	// Empty count
	if journal.Count() != 0 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 0)
	}

	// Add one record
	journal.Record(FailureRecord{
		URL:        "http://example.com/page",
		Stage:      StageFetch,
		Error:      "error",
		RetryCount: 1,
		Timestamp:  time.Now(),
	})

	if journal.Count() != 1 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 1)
	}

	// Add more records
	for i := 0; i < 5; i++ {
		journal.Record(FailureRecord{
			URL:        "http://example.com/page" + strconv.Itoa(i),
			Stage:      StageFetch,
			Error:      "error",
			RetryCount: i,
			Timestamp:  time.Now(),
		})
	}

	if journal.Count() != 6 {
		t.Errorf("Count() = %d, want %d", journal.Count(), 6)
	}

	// After clear
	journal.Clear()
	if journal.Count() != 0 {
		t.Errorf("Count() after Clear = %d, want %d", journal.Count(), 0)
	}
}

// ========== Mutex/Concurrency Tests ==========

func TestInMemoryJournal_ConcurrentRecord(t *testing.T) {
	journal := NewInMemoryJournal()

	var wg sync.WaitGroup
	numGoroutines := 10
	recordsPerGoroutine := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < recordsPerGoroutine; j++ {
				record := FailureRecord{
					URL:        "http://example.com/" + strconv.Itoa(idx*recordsPerGoroutine+j),
					Stage:      StageFetch,
					Error:      "concurrent error",
					RetryCount: j,
					Timestamp:  time.Now(),
				}
				journal.Record(record)
			}
		}(i)
	}
	wg.Wait()

	expectedCount := numGoroutines * recordsPerGoroutine
	if journal.Count() != expectedCount {
		t.Errorf("Count() = %d, want %d", journal.Count(), expectedCount)
	}

	// Verify all records were added correctly
	readRecords, err := journal.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(readRecords) != expectedCount {
		t.Errorf("Read() returned %d records, want %d", len(readRecords), expectedCount)
	}
}

func TestInMemoryJournal_ConcurrentReadAndRecord(t *testing.T) {
	journal := NewInMemoryJournal()

	// Pre-populate
	for i := 0; i < 10; i++ {
		journal.Record(FailureRecord{
			URL:        "http://example.com/init-" + strconv.Itoa(i),
			Stage:      StageFetch,
			Error:      "initial error",
			RetryCount: i,
			Timestamp:  time.Now(),
		})
	}

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
				journal.Record(FailureRecord{
					URL:        "http://example.com/write-" + strconv.Itoa(i),
					Stage:      StageFetch,
					Error:      "write error",
					RetryCount: i,
					Timestamp:  time.Now(),
				})
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
				records, err := journal.Read()
				if err != nil {
					t.Logf("Read error: %v", err)
					continue
				}
				// Verify no data corruption
				for _, r := range records {
					if r.URL == "" {
						t.Errorf("Data corruption: found record with empty URL")
					}
				}
			}
		}
	}()

	// Wait for goroutines to complete
	time.Sleep(50 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	// Final verification
	count := journal.Count()
	if count < 10 {
		t.Errorf("Expected at least 10 records, got %d", count)
	}

	t.Logf("Concurrent Read/Record test passed. Final count: %d", count)
}

func TestInMemoryJournal_ConcurrentReadAndClear(t *testing.T) {
	journal := NewInMemoryJournal()

	// Pre-populate
	for i := 0; i < 50; i++ {
		journal.Record(FailureRecord{
			URL:        "http://example.com/initial-" + strconv.Itoa(i),
			Stage:      StageFetch,
			Error:      "initial error",
			RetryCount: i,
			Timestamp:  time.Now(),
		})
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Clear goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Clear()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-stopChan:
				return
			default:
				records, err := journal.Read()
				if err != nil {
					t.Logf("Read error: %v", err)
					continue
				}
				// Verify no data corruption
				for _, r := range records {
					if r.URL == "" {
						t.Errorf("Data corruption: found record with empty URL")
					}
				}
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	// Count should be non-negative
	count := journal.Count()
	if count < 0 {
		t.Errorf("Invalid count: %d", count)
	}

	t.Logf("Concurrent Read/Clear test passed. Final count: %d", count)
}

func TestInMemoryJournal_ConcurrentRecordAndClear(t *testing.T) {
	journal := NewInMemoryJournal()

	// Pre-populate
	for i := 0; i < 20; i++ {
		journal.Record(FailureRecord{
			URL:        "http://example.com/initial-" + strconv.Itoa(i),
			Stage:      StageFetch,
			Error:      "initial error",
			RetryCount: i,
			Timestamp:  time.Now(),
		})
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Clear goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Clear()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Record goroutine
	numRecords := 100
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numRecords; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Record(FailureRecord{
					URL:        "http://example.com/new-" + strconv.Itoa(i),
					Stage:      StageFetch,
					Error:      "new error",
					RetryCount: i,
					Timestamp:  time.Now(),
				})
			}
		}
	}()

	close(stopChan)
	wg.Wait()

	// Count should be non-negative
	count := journal.Count()
	if count < 0 {
		t.Errorf("Invalid count: %d", count)
	}

	// Verify no data corruption
	records, err := journal.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	for _, r := range records {
		if r.URL == "" {
			t.Errorf("Data corruption: found record with empty URL")
		}
	}

	t.Logf("Concurrent Record/Clear test passed. Final count: %d", count)
}

func TestInMemoryJournal_ConcurrentAllOperations(t *testing.T) {
	journal := NewInMemoryJournal()

	var wg sync.WaitGroup
	stopChan := make(chan struct{})

	// Record goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Record(FailureRecord{
					URL:        "http://example.com/record-" + strconv.Itoa(i),
					Stage:      StageFetch,
					Error:      "error",
					RetryCount: i,
					Timestamp:  time.Now(),
				})
			}
		}
	}()

	// Read goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Read()
			}
		}
	}()

	// Count goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Count()
			}
		}
	}()

	// Clear goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			select {
			case <-stopChan:
				return
			default:
				journal.Clear()
				time.Sleep(3 * time.Millisecond)
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	// Final state should be consistent
	count := journal.Count()
	if count < 0 {
		t.Errorf("Invalid count: %d", count)
	}

	records, err := journal.Read()
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if len(records) != count {
		t.Errorf("Read() length = %d, Count() = %d", len(records), count)
	}

	t.Logf("Concurrent All Operations test passed. Final count: %d", count)
}

package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openwebui-content-sync/internal/adapter"
	"github.com/openwebui-content-sync/internal/mocks"
)

// MockSyncManager is a simple mock for testing
type MockSyncManager struct{}

func (m *MockSyncManager) SyncFiles(ctx context.Context, adapters []adapter.Adapter) error {
	return nil
}

func (m *MockSyncManager) SetKnowledgeID(knowledgeID string) {
	// Mock implementation
}

func (m *MockSyncManager) InitializeFileIndex(ctx context.Context, adapters []adapter.Adapter) error {
	// Mock implementation
	return nil
}

func TestNew(t *testing.T) {
	interval := 1 * time.Hour
	adapters := []adapter.Adapter{}
	syncManager := &MockSyncManager{}

	scheduler := New(interval, adapters, syncManager)
	if scheduler == nil {
		t.Fatal("Expected scheduler to be created")
	}
	if scheduler.interval != interval {
		t.Errorf("Expected interval %v, got %v", interval, scheduler.interval)
	}
	if len(scheduler.adapters) != len(adapters) {
		t.Errorf("Expected %d adapters, got %d", len(adapters), len(scheduler.adapters))
	}
}

func TestScheduler_RunSync(t *testing.T) {
	// Create mock sync manager
	syncManager := &MockSyncManager{}

	// Create mock adapters
	adapters := []adapter.Adapter{
		&mocks.MockAdapter{},
		&mocks.MockAdapter{},
	}

	scheduler := New(1*time.Hour, adapters, syncManager)

	// Test RunSync
	err := scheduler.RunSync()
	if err != nil {
		t.Errorf("RunSync failed: %v", err)
	}
}

func TestScheduler_Start(t *testing.T) {
	// Create mock sync manager
	syncManager := &MockSyncManager{}

	// Create mock adapters
	adapters := []adapter.Adapter{
		&mocks.MockAdapter{},
	}

	scheduler := New(100*time.Millisecond, adapters, syncManager)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Start scheduler in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scheduler.Start(ctx)
	}()

	// Wait for context to be cancelled
	<-ctx.Done()
	wg.Wait()
}

func TestScheduler_Interval(t *testing.T) {
	interval := 2 * time.Hour
	scheduler := New(interval, []adapter.Adapter{}, &MockSyncManager{})

	if scheduler.interval != interval {
		t.Errorf("Expected interval %v, got %v", interval, scheduler.interval)
	}
}

func TestScheduler_Adapters(t *testing.T) {
	adapters := []adapter.Adapter{
		&mocks.MockAdapter{},
		&mocks.MockAdapter{},
	}
	scheduler := New(1*time.Hour, adapters, &MockSyncManager{})

	if len(scheduler.adapters) != len(adapters) {
		t.Errorf("Expected %d adapters, got %d", len(adapters), len(scheduler.adapters))
	}

	for i, expected := range adapters {
		if scheduler.adapters[i] != expected {
			t.Errorf("Expected adapter %d to be %v, got %v", i, expected, scheduler.adapters[i])
		}
	}
}

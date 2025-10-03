package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/openwebui-content-sync/internal/adapter"
	"github.com/openwebui-content-sync/internal/sync"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

// Scheduler manages periodic synchronization
type Scheduler struct {
	cron        *cron.Cron
	interval    time.Duration
	adapters    []adapter.Adapter
	syncManager sync.ManagerInterface
}

// New creates a new scheduler
func New(interval time.Duration, adapters []adapter.Adapter, syncManager sync.ManagerInterface) *Scheduler {
	return &Scheduler{
		cron:        cron.New(cron.WithSeconds()),
		interval:    interval,
		adapters:    adapters,
		syncManager: syncManager,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) {
	logrus.Infof("Starting scheduler with interval: %v", s.interval)

	// Schedule the sync job
	cronSpec := fmt.Sprintf("@every %v", s.interval)
	_, err := s.cron.AddFunc(cronSpec, func() {
		logrus.Info("Running scheduled sync")
		if err := s.RunSyncWithContext(ctx); err != nil {
			logrus.Errorf("Scheduled sync failed: %v", err)
		}
	})
	if err != nil {
		logrus.Errorf("Failed to schedule sync job: %v", err)
		return
	}

	s.cron.Start()

	// Wait for context cancellation
	<-ctx.Done()
	logrus.Info("Stopping scheduler...")
	s.cron.Stop()
}

// RunSyncWithContext runs a synchronization cycle with the provided context
func (s *Scheduler) RunSyncWithContext(ctx context.Context) error {
	// Create a timeout context, but make it respect the parent context cancellation
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	return s.syncManager.SyncFiles(syncCtx, s.adapters)
}

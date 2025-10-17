// OpenWebUI Content Sync
// Copyright (C) 2025  OpenWebUI Content Sync Contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openwebui-content-sync/internal/adapter"
	"github.com/openwebui-content-sync/internal/config"
	"github.com/openwebui-content-sync/internal/health"
	"github.com/openwebui-content-sync/internal/scheduler"
	"github.com/openwebui-content-sync/internal/sync"
	"github.com/sirupsen/logrus"
)

func main() {
	var configPath = flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	// Set log level
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		logrus.Fatalf("Invalid log level: %v", err)
	}
	logrus.SetLevel(level)

	logrus.Info("Starting OpenWebUI Content Sync")

	// Initialize adapters
	adapters := make([]adapter.Adapter, 0)

	// Add GitHub adapter if configured
	if cfg.GitHub.Enabled {
		githubAdapter, err := adapter.NewGitHubAdapter(cfg.GitHub)
		if err != nil {
			logrus.Fatalf("Failed to create GitHub adapter: %v", err)
		}
		adapters = append(adapters, githubAdapter)
	}

	// Add Confluence adapter if configured
	if cfg.Confluence.Enabled {
		confluenceAdapter, err := adapter.NewConfluenceAdapter(cfg.Confluence)
		if err != nil {
			logrus.Fatalf("Failed to create Confluence adapter: %v", err)
		}
		adapters = append(adapters, confluenceAdapter)
	}

	// Add Local Folders adapter if configured
	if cfg.LocalFolders.Enabled {
		localAdapter, err := adapter.NewLocalFolderAdapter(cfg.LocalFolders)
		if err != nil {
			logrus.Fatalf("Failed to create Local Folders adapter: %v", err)
		}
		adapters = append(adapters, localAdapter)
	}

	// Add Slack adapter if configured
	if cfg.Slack.Enabled {
		slackAdapter, err := adapter.NewSlackAdapter(cfg.Slack, cfg.Storage.Path)
		if err != nil {
			logrus.Fatalf("Failed to create Slack adapter: %v", err)
		}
		adapters = append(adapters, slackAdapter)
	}

	// Initialize sync manager
	syncManager, err := sync.NewManager(cfg.OpenWebUI, cfg.Storage)
	if err != nil {
		logrus.Fatalf("Failed to create sync manager: %v", err)
	}

	// Note: With the mapping system, individual files will have their own knowledge IDs
	logrus.Infof("Using mapping-based knowledge ID assignment - files will use their individual knowledge IDs from mappings")

	// Initialize scheduler
	sched := scheduler.New(cfg.Schedule.Interval, adapters, syncManager)

	// Start health check server
	healthServer := health.NewServer(8080)
	go func() {
		if err := healthServer.Start(); err != nil {
			logrus.Errorf("Health server error: %v", err)
		}
	}()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler
	go sched.Start(ctx)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize file index from OpenWebUI
	logrus.Info("Initializing file index from OpenWebUI...")
	if err := syncManager.InitializeFileIndex(ctx, adapters); err != nil {
		logrus.Errorf("Failed to initialize file index: %v", err)
		// Continue even if initialization fails
	}

	// Run initial sync
	logrus.Info("Running initial sync...")
	if err := sched.RunSyncWithContext(ctx); err != nil {
		logrus.Errorf("Initial sync failed: %v", err)
	}

	// Wait for shutdown signal
	<-sigChan
	logrus.Info("Shutting down gracefully... (press CTRL+C again to force)")
	cancel()

	// Create a channel for forced shutdown
	forceChan := make(chan os.Signal, 1)
	signal.Notify(forceChan, syscall.SIGINT, syscall.SIGTERM)

	// Stop health server with timeout
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer healthCancel()

	// Run shutdown in a goroutine so we can detect double CTRL+C
	shutdownDone := make(chan bool, 1)
	go func() {
		healthServer.Stop(healthCtx)
		// Give some time for graceful shutdown
		time.Sleep(5 * time.Second)
		shutdownDone <- true
	}()

	// Wait for either shutdown completion or forced termination
	select {
	case <-shutdownDone:
		logrus.Info("Graceful shutdown completed")
	case <-forceChan:
		logrus.Warn("Force shutdown requested, exiting immediately")
		os.Exit(1)
	}
}

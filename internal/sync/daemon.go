// ABOUTME: Daemon process orchestrating fsnotify watcher, periodic reconciler, and single-writer sync loop.
// ABOUTME: Watches agentsview SQLite for changes and triggers sync engine with debounce and status reporting.
package sync

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	gosync "sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/clkao/agentstrove/internal/config"
	"github.com/clkao/agentstrove/internal/reader"
	"github.com/clkao/agentstrove/internal/store"
)

// DaemonStatus reports the current state of the daemon.
type DaemonStatus struct {
	LastSyncAt           time.Time
	TotalSessionsSynced  int64
	TotalSecretsDetected int64
	LastError            string
	Running              bool
}

// Daemon watches for agentsview SQLite changes and triggers sync.
type Daemon struct {
	engine *Engine
	config *config.Config
	store  store.Store
	reader *reader.Reader
	syncCh chan struct{}
	mu     gosync.Mutex
	status DaemonStatus
}

// NewDaemon creates a daemon from config, initializing reader, store, and sync engine.
func NewDaemon(cfg *config.Config) (*Daemon, error) {
	r, err := reader.NewReader(cfg.AgentsviewDBPath)
	if err != nil {
		return nil, fmt.Errorf("create reader: %w", err)
	}

	addr := cfg.ClickHouseAddr
	if addr == "" {
		addr = "localhost:9000"
	}

	var s *store.ClickHouseStore
	if cfg.ClickHouseUser != "" || cfg.ClickHousePassword != "" {
		s, err = store.NewClickHouseStoreWithAuth(addr, "agentstrove", cfg.ClickHouseUser, cfg.ClickHousePassword)
	} else {
		s, err = store.NewClickHouseStore(addr, "agentstrove")
	}
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("create store: %w", err)
	}

	if err := s.EnsureSchema(context.Background()); err != nil {
		s.Close()
		r.Close()
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	d, err := NewDaemonWithDeps(cfg, s, r)
	if err != nil {
		s.Close()
		r.Close()
		return nil, err
	}

	return d, nil
}

// NewDaemonWithDeps creates a daemon using a pre-built store and reader.
// The caller is responsible for closing the store and reader if this returns an error.
func NewDaemonWithDeps(cfg *config.Config, s store.Store, r *reader.Reader) (*Daemon, error) {
	engine, err := NewEngine(cfg, r, s)
	if err != nil {
		return nil, fmt.Errorf("create engine: %w", err)
	}

	return &Daemon{
		engine: engine,
		config: cfg,
		store:  s,
		reader: r,
		syncCh: make(chan struct{}, 1),
	}, nil
}

// Run starts the watcher, reconciler, and writer goroutines and blocks until
// the context is cancelled. On cancellation, the writer loop finishes its
// current sync cycle before returning.
func (d *Daemon) Run(ctx context.Context) error {
	d.mu.Lock()
	d.status.Running = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.status.Running = false
		d.mu.Unlock()
	}()

	var wg gosync.WaitGroup

	wg.Add(3)
	go func() { defer wg.Done(); d.watchLoop(ctx) }()
	go func() { defer wg.Done(); d.reconcileLoop(ctx) }()
	go func() { defer wg.Done(); d.writerLoop(ctx) }()

	<-ctx.Done()
	wg.Wait()

	return nil
}

// watchLoop uses fsnotify to watch the agentsview SQLite file for changes.
// On write/create events, it debounces 500ms then sends a sync request.
func (d *Daemon) watchLoop(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("daemon: failed to create watcher: %v", err)
		return
	}
	defer watcher.Close()

	dbPath := d.config.AgentsviewDBPath

	// Watch the directory containing the SQLite file so we also catch
	// WAL file creation/modification events.
	watchDir := filepath.Dir(dbPath)
	if err := watcher.Add(watchDir); err != nil {
		log.Printf("daemon: failed to watch %s: %v", watchDir, err)
		return
	}

	dbBase := filepath.Base(dbPath)
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// Only react to changes to the SQLite DB file or its WAL/SHM files
			base := filepath.Base(event.Name)
			if base != dbBase && base != dbBase+"-wal" && base != dbBase+"-shm" {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce: reset timer on each event, fire 500ms after last
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				select {
				case d.syncCh <- struct{}{}:
				default:
				}
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("daemon: watcher error: %v", err)
		}
	}
}

// reconcileLoop sends a sync request every 5 minutes to catch any events that
// fsnotify may have missed.
func (d *Daemon) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case d.syncCh <- struct{}{}:
			default:
			}
		}
	}
}

// writerLoop processes sync requests from syncCh by calling Engine.RunOnce.
// It updates daemon status after each sync and logs the result. On context
// cancellation, it finishes the current RunOnce before returning.
func (d *Daemon) writerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.syncCh:
			d.doSync(ctx)
		}
	}
}

// doSync executes one sync cycle and updates status.
func (d *Daemon) doSync(ctx context.Context) {
	result, err := d.engine.RunOnce(ctx)

	d.mu.Lock()
	if err != nil {
		d.status.LastError = err.Error()
		d.mu.Unlock()
		log.Printf("daemon: sync error: %v", err)
		return
	}

	d.status.LastSyncAt = time.Now().UTC()
	d.status.TotalSessionsSynced += int64(result.SessionsSynced)
	d.status.TotalSecretsDetected += int64(result.SecretsDetected)

	if len(result.Errors) > 0 {
		for sessID, sessionErr := range result.Errors {
			log.Printf("daemon: sync error for session %s: %v", sessID, sessionErr)
			d.status.LastError = sessionErr.Error()
		}
	} else {
		d.status.LastError = ""
	}
	d.mu.Unlock()

	log.Printf("daemon: synced %d sessions (%d secrets masked, %d errors)",
		result.SessionsSynced, result.SecretsDetected, len(result.Errors))
}

// Status returns a snapshot of the current daemon status.
func (d *Daemon) Status() DaemonStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

// Close releases daemon resources (reader and store).
func (d *Daemon) Close() error {
	if d.reader != nil {
		d.reader.Close()
	}
	if d.store != nil {
		d.store.Close()
	}
	return nil
}

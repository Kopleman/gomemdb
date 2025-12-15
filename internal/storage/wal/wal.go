package wal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

const (
	walFilePrefix = "wal_"
	walFileSuffix = ".log"
	uint32Size    = 4
)

var (
	ErrWALDisabled = errors.New("WAL is disabled")
	ErrWALClosed   = errors.New("WAL is closed")
)

// pendingCommand represents a command waiting to be flushed.
type pendingCommand struct {
	doneCh chan struct{}
	errCh  chan error
	cmd    command.Command
}

// WAL represents Write-Ahead Log implementation.
type WAL struct {
	flushWorkerDone chan struct{}
	logger          *zap.Logger
	currentSegment  *segment
	flushCh         chan struct{}
	flushTimer      *time.Timer
	cancel          context.CancelFunc
	pending         []pendingCommand
	config          Config
	wg              sync.WaitGroup
	segmentCounter  uint64
	mu              sync.RWMutex
	batchMu         sync.Mutex
	closed          bool
}

// Config represents WAL configuration.
type Config struct {
	DataDirectory        string
	FlushingBatchSize    int
	FlushingBatchTimeout time.Duration
	MaxSegmentSize       int64
	Enabled              bool
}

// New creates a new WAL instance.
func New(config Config, logger *zap.Logger) (*WAL, error) {
	if !config.Enabled {
		return nil, ErrWALDisabled
	}

	const dirPerm = 0o750
	if err := os.MkdirAll(config.DataDirectory, dirPerm); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Find maximum segment number to continue from
	maxSegmentNum, err := findMaxSegmentNumber(config.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to find max segment number: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &WAL{
		config:          config,
		logger:          logger,
		segmentCounter:  maxSegmentNum,
		pending:         make([]pendingCommand, 0, config.FlushingBatchSize),
		flushCh:         make(chan struct{}, 1),
		flushWorkerDone: make(chan struct{}),
		cancel:          cancel,
	}

	// Start flush worker
	w.wg.Add(1)
	go w.flushWorker(ctx)

	return w, nil
}

// findMaxSegmentNumber finds the maximum segment number in the data directory.
func findMaxSegmentNumber(dataDirectory string) (uint64, error) {
	entries, err := os.ReadDir(dataDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read directory: %w", err)
	}

	maxNum := uint64(0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), walFilePrefix) || !strings.HasSuffix(entry.Name(), walFileSuffix) {
			continue
		}
		seqStr := strings.TrimPrefix(entry.Name(), walFilePrefix)
		seqStr = strings.TrimSuffix(seqStr, walFileSuffix)
		seq, err := strconv.ParseUint(seqStr, 10, 64)
		if err != nil {
			continue
		}
		if seq > maxNum {
			maxNum = seq
		}
	}

	return maxNum, nil
}

// Write appends a command to the WAL batch.
// It blocks until the command is flushed to disk.
func (w *WAL) Write(ctx context.Context, cmd command.Command) error {
	if w == nil {
		return nil // WAL disabled
	}

	w.mu.RLock()
	if w.closed {
		w.mu.RUnlock()
		return ErrWALClosed
	}
	w.mu.RUnlock()

	// Create channels for notification
	doneCh := make(chan struct{})
	errCh := make(chan error, 1)

	pending := pendingCommand{
		cmd:    cmd,
		doneCh: doneCh,
		errCh:  errCh,
	}

	w.batchMu.Lock()
	w.pending = append(w.pending, pending)
	batchSize := len(w.pending)
	shouldFlush := batchSize >= w.config.FlushingBatchSize

	// Reset timer if this is the first command in a new batch
	if batchSize == 1 {
		if w.flushTimer != nil {
			w.flushTimer.Stop()
		}
		w.flushTimer = time.AfterFunc(w.config.FlushingBatchTimeout, func() {
			select {
			case w.flushCh <- struct{}{}:
			default:
			}
		})
	}

	w.batchMu.Unlock()

	if shouldFlush {
		select {
		case w.flushCh <- struct{}{}:
		default:
		}
	}

	// Wait for flush to complete
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	case err := <-errCh:
		return fmt.Errorf("WAL write error: %w", err)
	case <-doneCh:
		return nil
	}
}

// flushWorker periodically flushes batches to disk.
func (w *WAL) flushWorker(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-ctx.Done():
			// Flush remaining batch before exit
			w.batchMu.Lock()
			if len(w.pending) > 0 {
				w.flushBatch()
			}
			w.batchMu.Unlock()
			close(w.flushWorkerDone)
			return
		case <-w.flushCh:
			w.batchMu.Lock()
			if len(w.pending) > 0 {
				w.flushBatch()
			}
			w.batchMu.Unlock()
		}
	}
}

// flushBatch writes the current batch to disk and clears it.
func (w *WAL) flushBatch() {
	if len(w.pending) == 0 {
		return
	}

	batch := make([]pendingCommand, len(w.pending))
	copy(batch, w.pending)
	w.pending = w.pending[:0]

	if w.flushTimer != nil {
		w.flushTimer.Stop()
		w.flushTimer = nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		// Notify all pending commands about error
		for _, p := range batch {
			select {
			case p.errCh <- ErrWALClosed:
			default:
			}
		}
		return
	}

	// Ensure we have a current segment
	if w.currentSegment == nil {
		seg, err := w.createNewSegment()
		if err != nil {
			w.logger.Error("failed to create segment", zap.Error(err))
			// Notify all pending commands about error
			for _, p := range batch {
				select {
				case p.errCh <- err:
				default:
				}
			}
			return
		}
		w.currentSegment = seg
	}

	// Write batch to segment
	var flushErr error
	for _, p := range batch {
		if err := w.writeCommand(p.cmd); err != nil {
			w.logger.Error("failed to write command to WAL", zap.Error(err))
			flushErr = err
			break
		}
	}

	// Sync to disk
	if flushErr == nil {
		if err := w.currentSegment.Sync(); err != nil {
			w.logger.Error("failed to sync WAL segment", zap.Error(err))
			flushErr = err
		}
	}

	// Notify all commands in batch
	if flushErr != nil {
		for _, p := range batch {
			select {
			case p.errCh <- flushErr:
			default:
			}
		}
	} else {
		for _, p := range batch {
			close(p.doneCh)
		}
	}

	w.logger.Debug("flushed batch to WAL", zap.Int("size", len(batch)))
}

// writeCommand writes a single command to the current segment.
func (w *WAL) writeCommand(cmd command.Command) error {
	// Check if we need to rotate segment
	if w.currentSegment.Size() >= w.config.MaxSegmentSize {
		if err := w.currentSegment.Close(); err != nil {
			return fmt.Errorf("failed to close segment: %w", err)
		}

		seg, err := w.createNewSegment()
		if err != nil {
			return fmt.Errorf("failed to create new segment: %w", err)
		}
		w.currentSegment = seg
	}

	return w.currentSegment.WriteCommand(cmd)
}

// createNewSegment creates a new WAL segment file.
func (w *WAL) createNewSegment() (*segment, error) {
	w.segmentCounter++
	segmentName := fmt.Sprintf("%s%016d%s", walFilePrefix, w.segmentCounter, walFileSuffix)
	segmentPath := filepath.Join(w.config.DataDirectory, segmentName)
	return newSegment(segmentPath, w.logger)
}

// Close closes the WAL and flushes any remaining data.
func (w *WAL) Close() error {
	if w == nil {
		return nil
	}

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	// Cancel context to stop flush worker
	w.cancel()

	// Wait for flush worker to finish
	<-w.flushWorkerDone

	// Flush remaining batch
	w.batchMu.Lock()
	if len(w.pending) > 0 {
		w.flushBatch()
	}
	w.batchMu.Unlock()

	// Close current segment
	w.mu.Lock()
	if w.currentSegment != nil {
		if err := w.currentSegment.Close(); err != nil {
			w.mu.Unlock()
			return fmt.Errorf("failed to close segment: %w", err)
		}
	}
	w.mu.Unlock()

	w.wg.Wait()

	return nil
}

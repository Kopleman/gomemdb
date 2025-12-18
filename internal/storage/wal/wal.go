package wal

import (
	"context"
	"errors"
	"fmt"
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
	resultCh chan error
	cmd      command.Command
}

// WAL represents Write-Ahead Log implementation.
type WAL struct {
	logger         *zap.Logger
	currentSegment *segment
	timer          *time.Timer
	flushCh        chan struct{}
	done           chan struct{}
	pending        []pendingCommand
	config         Config
	wg             sync.WaitGroup
	segmentCounter uint64
	closeOnce      sync.Once
	mu             sync.Mutex
	closed         bool
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

	if err := MkDir(config.DataDirectory); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Find maximum segment number to continue from
	maxSegmentNum, err := FindMaxSegmentNumber(config.DataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to find max segment number: %w", err)
	}

	w := &WAL{
		config:         config,
		logger:         logger,
		segmentCounter: maxSegmentNum,
		pending:        make([]pendingCommand, 0, config.FlushingBatchSize),
		flushCh:        make(chan struct{}, 1),
		done:           make(chan struct{}),
	}

	// Start flush worker
	w.wg.Add(1)
	go w.flushWorker()

	return w, nil
}

// Write appends a command to the WAL batch.
// It blocks until the command is flushed to disk.
func (w *WAL) Write(ctx context.Context, cmd command.Command) error {
	if w == nil {
		return nil // WAL disabled
	}

	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrWALClosed
	}

	// Create channel for notification
	resultCh := make(chan error, 1)

	pending := pendingCommand{
		cmd:      cmd,
		resultCh: resultCh,
	}

	w.pending = append(w.pending, pending)
	batchSize := len(w.pending)
	shouldFlush := batchSize >= w.config.FlushingBatchSize

	// Start or reset timeout timer if this is the first command in a new batch
	if batchSize == 1 {
		if w.timer != nil {
			w.timer.Stop()
		}
		w.timer = time.AfterFunc(w.config.FlushingBatchTimeout, func() {
			select {
			case w.flushCh <- struct{}{}:
			default:
			}
		})
	}

	w.mu.Unlock()

	// Trigger flush if batch is full
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
	case err := <-resultCh:
		if err != nil {
			return fmt.Errorf("WAL write error: %w", err)
		}
		return nil
	}
}

// flushWorker periodically flushes batches to disk.
func (w *WAL) flushWorker() {
	defer w.wg.Done()

	for {
		select {
		case <-w.done:
			// Flush remaining batch before exit
			w.mu.Lock()
			if len(w.pending) > 0 {
				w.flushBatch()
			}
			w.mu.Unlock()
			return
		case <-w.flushCh:
			w.mu.Lock()
			if len(w.pending) > 0 {
				w.flushBatch()
			}
			w.mu.Unlock()
		}
	}
}

// flushBatch writes the current batch to disk and clears it.
// Must be called with mu locked.
func (w *WAL) flushBatch() {
	if len(w.pending) == 0 {
		return
	}

	batch := make([]pendingCommand, len(w.pending))
	copy(batch, w.pending)
	w.pending = w.pending[:0]

	// Stop timer if it's running
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}

	if w.closed {
		// Notify all pending commands about error
		for _, p := range batch {
			select {
			case p.resultCh <- ErrWALClosed:
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
				case p.resultCh <- err:
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
	for _, p := range batch {
		select {
		case p.resultCh <- flushErr:
		default:
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
	segmentPath := BuildSegmentPath(w.config.DataDirectory, w.segmentCounter)
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

	// Flush remaining batch before closing
	if len(w.pending) > 0 {
		w.flushBatch()
	}

	// Close current segment
	var closeErr error
	if w.currentSegment != nil {
		if err := w.currentSegment.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close segment: %w", err)
		}
	}
	w.mu.Unlock()

	// Signal flush worker to stop (safe to call multiple times)
	w.closeOnce.Do(func() {
		close(w.done)
	})

	// Wait for flush worker to finish
	w.wg.Wait()

	return closeErr
}

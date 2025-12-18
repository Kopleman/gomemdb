package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// SegmentInfo represents metadata about a WAL segment file.
type SegmentInfo struct {
	Path        string
	Filename    string
	SequenceNum uint64
}

// FindSegments finds all WAL segment files in the given directory.
// Returns segments sorted by sequence number.
func FindSegments(dataDirectory string) ([]SegmentInfo, error) {
	// Check if directory exists
	if _, err := os.Stat(dataDirectory); os.IsNotExist(err) {
		return nil, nil
	}

	// List all files in directory
	entries, err := os.ReadDir(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL directory: %w", err)
	}

	// Filter and collect segment files
	segments := make([]SegmentInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !IsSegmentFile(filename) {
			continue
		}

		seqNum := ExtractSequenceNumber(filename)
		segPath := filepath.Join(dataDirectory, filename)

		segments = append(segments, SegmentInfo{
			Path:        segPath,
			Filename:    filename,
			SequenceNum: seqNum,
		})
	}

	// Sort segments by sequence number
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].SequenceNum < segments[j].SequenceNum
	})

	return segments, nil
}

// FindMaxSegmentNumber finds the maximum segment number in the data directory.
func FindMaxSegmentNumber(dataDirectory string) (uint64, error) {
	segments, err := FindSegments(dataDirectory)
	if err != nil {
		return 0, err
	}

	if len(segments) == 0 {
		return 0, nil
	}

	// Segments are already sorted, so the last one has the max number
	return segments[len(segments)-1].SequenceNum, nil
}

// IsSegmentFile checks if a filename matches the WAL segment file pattern.
func IsSegmentFile(filename string) bool {
	return strings.HasPrefix(filename, walFilePrefix) && strings.HasSuffix(filename, walFileSuffix)
}

// ExtractSequenceNumber extracts sequence number from segment filename.
// Format: wal_0000000000000001.log -> 1.
func ExtractSequenceNumber(filename string) uint64 {
	// Remove prefix "wal_" and suffix ".log"
	seqStr := strings.TrimPrefix(filename, walFilePrefix)
	seqStr = strings.TrimSuffix(seqStr, walFileSuffix)
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return 0
	}
	return seq
}

// BuildSegmentPath constructs the full path to a segment file.
func BuildSegmentPath(dataDirectory string, sequenceNum uint64) string {
	filename := FormatSegmentFilename(sequenceNum)
	return filepath.Join(dataDirectory, filename)
}

// FormatSegmentFilename formats a sequence number into a segment filename.
// Format: wal_0000000000000001.log.
func FormatSegmentFilename(sequenceNum uint64) string {
	return fmt.Sprintf("%s%016d%s", walFilePrefix, sequenceNum, walFileSuffix)
}

// SegmentFile represents an open WAL segment file with logging capabilities.
// It wraps os.File to provide consistent error handling and logging for segment operations.
type SegmentFile struct {
	file   *os.File
	logger *zap.Logger
	path   string
}

// Close closes the segment file and logs any errors that occur during closing.
// This method is safe to call multiple times.
func (f *SegmentFile) Close() {
	if closeErr := f.file.Close(); closeErr != nil {
		f.logger.Error("failed to close segment file", zap.Error(closeErr), zap.String("path", f.path))
	}
}

// Read reads data from the segment file into the provided buffer.
// It implements the io.Reader interface.
// Returns the number of bytes read and any error encountered.
func (f *SegmentFile) Read(p []byte) (n int, err error) {
	if f.file == nil {
		return 0, nil
	}
	n, err = f.file.Read(p)
	if err != nil {
		return n, fmt.Errorf("failed to read segment file: %w", err)
	}
	return n, nil
}

// OpenFile opens a WAL segment file at the given path and returns a SegmentFile wrapper.
// The returned SegmentFile should be closed after use to release resources.
func OpenFile(path string, logger *zap.Logger) (*SegmentFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open segment file: %w", err)
	}

	return &SegmentFile{
		path:   path,
		file:   file,
		logger: logger,
	}, nil
}

func MkDir(path string) error {
	const dirPerm = 0o750
	if err := os.MkdirAll(path, dirPerm); err != nil {
		return fmt.Errorf("failed to create WAL directory: %w", err)
	}

	return nil
}

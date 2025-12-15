package wal

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

const (
	// Command type bytes.
	cmdTypeSET byte = 1
	cmdTypeDEL byte = 2
	filePerm        = 0o600
)

// segment represents a single WAL segment file.
type segment struct {
	file   *os.File
	logger *zap.Logger
	path   string
	size   int64
}

// newSegment creates a new segment file.
func newSegment(path string, logger *zap.Logger) (*segment, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open segment file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			logger.Error("failed to close file after stat error", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to stat segment file: %w", err)
	}

	return &segment{
		file:   file,
		path:   path,
		size:   stat.Size(),
		logger: logger,
	}, nil
}

// WriteCommand writes a command to the segment file.
// Format: [type:1 byte][name_len:4 bytes][name:name_len bytes][value_len:4 bytes][value:value_len bytes]
// For DEL commands, value_len is 0.
func (s *segment) WriteCommand(cmd command.Command) error {
	var cmdType byte
	var value string

	switch cmd.Type {
	case command.CommandSET:
		cmdType = cmdTypeSET
		value = cmd.Set.Value
	case command.CommandDEL:
		cmdType = cmdTypeDEL
		value = ""
	default:
		return fmt.Errorf("unsupported command type: %s", cmd.Type)
	}

	nameBytes := []byte(cmd.Name)
	valueBytes := []byte(value)

	const (
		cmdTypeSize = 1
	)

	// Calculate total size: 1 (type) + 4 (name_len) + name_len + 4 (value_len) + value_len.
	totalSize := cmdTypeSize + uint32Size + len(nameBytes) + uint32Size + len(valueBytes)

	// Write command type
	if err := binary.Write(s.file, binary.LittleEndian, cmdType); err != nil {
		return fmt.Errorf("failed to write command type: %w", err)
	}

	// Write name length and name
	nameLen := uint32(len(nameBytes))
	if err := binary.Write(s.file, binary.LittleEndian, nameLen); err != nil {
		return fmt.Errorf("failed to write name length: %w", err)
	}
	if _, err := s.file.Write(nameBytes); err != nil {
		return fmt.Errorf("failed to write name: %w", err)
	}

	// Write value length and value
	valueLen := uint32(len(valueBytes))
	if err := binary.Write(s.file, binary.LittleEndian, valueLen); err != nil {
		return fmt.Errorf("failed to write value length: %w", err)
	}
	if len(valueBytes) > 0 {
		if _, err := s.file.Write(valueBytes); err != nil {
			return fmt.Errorf("failed to write value: %w", err)
		}
	}

	s.size += int64(totalSize)

	return nil
}

// Sync flushes the segment file to disk.
func (s *segment) Sync() error {
	if err := s.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync segment file: %w", err)
	}
	return nil
}

// Size returns the current size of the segment file.
func (s *segment) Size() int64 {
	return s.size
}

// Close closes the segment file.
func (s *segment) Close() error {
	if s.file == nil {
		return nil
	}
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("failed to close segment file: %w", err)
	}
	return nil
}

// Path returns the path of the segment file.
func (s *segment) Path() string {
	return s.path
}

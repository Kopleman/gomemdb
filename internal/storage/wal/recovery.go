package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

// Recover reads all WAL segments and returns commands in order.
func Recover(dataDirectory string, logger *zap.Logger) ([]command.Command, error) {
	// Find all segment files using filesystem component
	segments, err := FindSegments(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to find WAL segments: %w", err)
	}

	if len(segments) == 0 {
		logger.Info("no WAL segments found, skipping recovery")
		return nil, nil
	}

	logger.Info("recovering from WAL segments", zap.Int("count", len(segments)))

	// Read all segments
	var allCommands []command.Command
	for _, seg := range segments {
		commands, err := readSegment(seg.Path, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to read segment %s: %w", seg.Filename, err)
		}
		allCommands = append(allCommands, commands...)
	}

	logger.Info("recovered commands from WAL", zap.Int("count", len(allCommands)))
	return allCommands, nil
}

// readSegment reads all commands from a segment file.
func readSegment(path string, logger *zap.Logger) ([]command.Command, error) {
	file, err := OpenFile(path, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to open segment file: %w", err)
	}
	defer file.Close()

	var commands []command.Command

	for {
		cmd, err := readCommand(file)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read command: %w", err)
		}
		commands = append(commands, cmd)
	}

	logger.Debug("read segment", zap.String("path", path), zap.Int("commands", len(commands)))
	return commands, nil
}

// readCommand reads a single command from the file.
func readCommand(r io.Reader) (command.Command, error) {
	var cmdType byte
	if err := binary.Read(r, binary.LittleEndian, &cmdType); err != nil {
		return command.Command{}, fmt.Errorf("failed to read command type: %w", err)
	}

	// Read name length
	var nameLen uint32
	if err := binary.Read(r, binary.LittleEndian, &nameLen); err != nil {
		return command.Command{}, fmt.Errorf("failed to read name length: %w", err)
	}

	// Read name
	nameBytes := make([]byte, nameLen)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return command.Command{}, fmt.Errorf("failed to read name: %w", err)
	}

	// Read value length
	var valueLen uint32
	if err := binary.Read(r, binary.LittleEndian, &valueLen); err != nil {
		return command.Command{}, fmt.Errorf("failed to read value length: %w", err)
	}

	// Read value
	var valueBytes []byte
	if valueLen > 0 {
		valueBytes = make([]byte, valueLen)
		if _, err := io.ReadFull(r, valueBytes); err != nil {
			return command.Command{}, fmt.Errorf("failed to read value: %w", err)
		}
	}

	// Construct command
	cmd := command.Command{
		Name: string(nameBytes),
	}

	switch cmdType {
	case cmdTypeSET:
		cmd.Type = command.CommandSET
		cmd.Set = command.SetArgs{
			Value: string(valueBytes),
		}
	case cmdTypeDEL:
		cmd.Type = command.CommandDEL
	default:
		return command.Command{}, fmt.Errorf("unknown command type: %d", cmdType)
	}

	return cmd, nil
}

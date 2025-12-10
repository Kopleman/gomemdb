package engine

import (
	"context"
)

const (
	// DefaultStorageCapacity is the default capacity for the storage map.
	defaultStorageCapacity = 50
)

type storage struct {
	data map[string]string
}

func newStorage() *storage {
	return &storage{
		data: make(map[string]string, defaultStorageCapacity),
	}
}

func (s *storage) Set(ctx context.Context, name string, value string) error {
	s.data[name] = value
	return nil
}

func (s *storage) Get(ctx context.Context, name string) (string, error) {
	v, ok := s.data[name]
	if !ok {
		return "", ErrNotFound
	}

	return v, nil
}

func (s *storage) Del(ctx context.Context, name string) error {
	delete(s.data, name)

	return nil
}

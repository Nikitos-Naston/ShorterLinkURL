package repository

import (
	"context"
	"sync"

	"LinkShrink/internal/service"
)

type Memory struct {
	mu     sync.RWMutex
	byCode map[string]service.Link
	byURL  map[string]string
}

var _ service.Repository = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{byCode: make(map[string]service.Link), byURL: make(map[string]string)}
}

func (s *Memory) FindByURL(ctx context.Context, originalURL string) (service.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return service.Link{}, err
	}
	code, ok := s.byURL[originalURL]
	if !ok {
		return service.Link{}, service.ErrNotFound
	}
	return s.byCode[code], nil
}

func (s *Memory) FindByCode(ctx context.Context, code string) (service.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return service.Link{}, err
	}
	link, ok := s.byCode[code]
	if !ok {
		return service.Link{}, service.ErrNotFound
	}
	return link, nil
}

func (s *Memory) Create(ctx context.Context, link service.Link) (service.Link, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return service.Link{}, false, err
	}
	// Both maps must be checked and updated under the SAME lock.
	if code, ok := s.byURL[link.URL]; ok {
		return s.byCode[code], false, nil
	}
	if _, ok := s.byCode[link.Code]; ok {
		return service.Link{}, false, service.ErrCodeConflict
	}
	s.byCode[link.Code] = link
	s.byURL[link.URL] = link.Code
	return link, true, nil
}

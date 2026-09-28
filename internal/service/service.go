package service

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrNotFound            = errors.New("link not found")
	ErrCodeConflict        = errors.New("code already exists")
	ErrInvalidURL          = errors.New("invalid URL")
	ErrInvalidCode         = errors.New("invalid code")
	ErrGenerationExhausted = errors.New("could not allocate a unique code")
)

const maxAttempts = 10

type Link struct {
	Code string
	URL  string
}
type Repository interface {
	FindByURL(context.Context, string) (Link, error)
	FindByCode(context.Context, string) (Link, error)
	Create(context.Context, Link) (link Link, created bool, err error)
}

type Service struct {
	repo     Repository
	generate func() (string, error)
}

func New(repo Repository) *Service {
	return &Service{repo: repo, generate: GenerateCode}
}

func (s *Service) Shorten(ctx context.Context, originalURL string) (Link, bool, error) {
	if err := ValidateURL(originalURL); err != nil {
		return Link{}, false, err
	}
	link, err := s.repo.FindByURL(ctx, originalURL)
	if err == nil {
		return link, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Link{}, false, fmt.Errorf("find original URL: %w", err)
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return Link{}, false, err
		}
		code, err := s.generate()
		if err != nil {
			return Link{}, false, fmt.Errorf("generate code: %w", err)
		}
		link, created, err := s.repo.Create(ctx, Link{Code: code, URL: originalURL})
		if errors.Is(err, ErrCodeConflict) {
			continue
		}
		if err != nil {
			return Link{}, false, fmt.Errorf("create link: %w", err)
		}
		return link, created, nil
	}
	return Link{}, false, ErrGenerationExhausted
}

func (s *Service) Resolve(ctx context.Context, code string) (Link, error) {
	if !ValidCode(code) {
		return Link{}, ErrInvalidCode
	}
	link, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		return Link{}, fmt.Errorf("find code: %w", err)
	}
	return link, nil
}

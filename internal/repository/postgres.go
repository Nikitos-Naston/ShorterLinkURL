package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"LinkShrink/internal/service"
)

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Postgres struct{ db querier }

var _ service.Repository = (*Postgres)(nil)

func NewPostgres(db querier) *Postgres { return &Postgres{db: db} }

func scanLink(row pgx.Row) (service.Link, error) {
	var link service.Link
	err := row.Scan(&link.Code, &link.URL)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.Link{}, service.ErrNotFound
	}
	return link, err
}

func (r *Postgres) FindByURL(ctx context.Context, originalURL string) (service.Link, error) {
	return scanLink(r.db.QueryRow(ctx, "SELECT code, original_url FROM links WHERE original_url = $1", originalURL))
}

func (r *Postgres) FindByCode(ctx context.Context, code string) (service.Link, error) {
	return scanLink(r.db.QueryRow(ctx, "SELECT code, original_url FROM links WHERE code = $1", code))
}

func (r *Postgres) Create(ctx context.Context, candidate service.Link) (service.Link, bool, error) {
	link, err := scanLink(r.db.QueryRow(ctx, `
  INSERT INTO links (code, original_url) VALUES ($1, $2)
  ON CONFLICT DO NOTHING RETURNING code, original_url`, candidate.Code, candidate.URL))
	if err == nil {
		return link, true, nil
	}
	if !errors.Is(err, service.ErrNotFound) {
		return service.Link{}, false, err
	}

	link, err = r.FindByURL(ctx, candidate.URL)
	if err == nil {
		return link, false, nil
	}
	if errors.Is(err, service.ErrNotFound) {
		return service.Link{}, false, service.ErrCodeConflict
	}
	return service.Link{}, false, err
}

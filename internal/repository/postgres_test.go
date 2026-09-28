package repository

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"LinkShrink/internal/service"

	"github.com/jackc/pgx/v5"
)

type rowStub struct {
	link service.Link
	err  error
}

func (r rowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*string)) = r.link.Code
	*(dest[1].(*string)) = r.link.URL
	return nil
}

type queryStep struct {
	args []any
	row  rowStub
}
type dbStub struct {
	t     *testing.T
	steps []queryStep
	calls int
}

func (d *dbStub) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	d.t.Helper()
	if d.calls >= len(d.steps) {
		d.t.Fatal("unexpected query")
	}
	step := d.steps[d.calls]
	d.calls++
	if !reflect.DeepEqual(args, step.args) {
		d.t.Fatalf("parameters %v, want %v", args, step.args)
	}
	for _, v := range args {
		if str, ok := v.(string); ok && strings.Contains(sql, str) {
			d.t.Fatal("input interpolated into SQL")
		}
	}
	return step.row
}

func TestCreate(t *testing.T) {
	candidate := service.Link{Code: "0123456789", URL: "https://example.org/'?x=1"}
	existing := service.Link{Code: "Existing_1", URL: candidate.URL}
	dbErr := errors.New("database failure")
	tests := []struct {
		name    string
		rows    []rowStub
		want    service.Link
		created bool
		err     error
	}{
		{"insert", []rowStub{{link: candidate}}, candidate, true, nil},
		{"same URL", []rowStub{{err: pgx.ErrNoRows}, {link: existing}}, existing, false, nil},
		{"code collision", []rowStub{{err: pgx.ErrNoRows}, {err: pgx.ErrNoRows}}, service.Link{}, false, service.ErrCodeConflict},
		{"insert error", []rowStub{{err: dbErr}}, service.Link{}, false, dbErr},
		{"lookup error", []rowStub{{err: pgx.ErrNoRows}, {err: dbErr}}, service.Link{}, false, dbErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &dbStub{t: t, steps: []queryStep{{args: []any{candidate.Code, candidate.URL}, row: tt.rows[0]}}}
			if len(tt.rows) > 1 {
				db.steps = append(db.steps, queryStep{args: []any{candidate.URL}, row: tt.rows[1]})
			}
			s := NewPostgres(db)
			got, created, err := s.Create(context.Background(), candidate)
			if got != tt.want || created != tt.created || !errors.Is(err, tt.err) {
				t.Fatalf("%+v %v %v", got, created, err)
			}
			if db.calls != len(db.steps) {
				t.Fatal("missing query")
			}
		})
	}
}

func TestFind(t *testing.T) {
	link := service.Link{Code: "0123456789", URL: "https://example.org"}
	db := &dbStub{t: t, steps: []queryStep{
		{args: []any{link.Code}, row: rowStub{link: link}},
		{args: []any{link.URL}, row: rowStub{err: pgx.ErrNoRows}},
	}}
	s := NewPostgres(db)
	if got, err := s.FindByCode(context.Background(), link.Code); got != link || err != nil {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := s.FindByURL(context.Background(), link.URL); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
}

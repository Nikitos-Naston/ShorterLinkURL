package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"LinkShrink/internal/service"
)

func TestStoreContract(t *testing.T) {
	ctx := context.Background()
	s := NewMemory()
	link := service.Link{Code: "0123456789", URL: "https://example.org"}
	if _, err := s.FindByCode(ctx, link.Code); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.FindByURL(ctx, link.URL); !errors.Is(err, service.ErrNotFound) {
		t.Fatal(err)
	}
	if got, created, err := s.Create(ctx, link); got != link || !created || err != nil {
		t.Fatalf("%+v %v %v", got, created, err)
	}
	for _, candidate := range []service.Link{link, {Code: "OtherCode1", URL: link.URL}} {
		if got, created, err := s.Create(ctx, candidate); got != link || created || err != nil {
			t.Fatalf("%+v %v %v", got, created, err)
		}
	}
	if _, _, err := s.Create(ctx, service.Link{Code: link.Code, URL: "https://other.org"}); !errors.Is(err, service.ErrCodeConflict) {
		t.Fatal(err)
	}
	if _, created, err := s.Create(ctx, service.Link{Code: "OtherCode1", URL: "https://other.org"}); err != nil || !created {
		t.Fatal(err)
	}
	if got, err := s.FindByURL(ctx, link.URL); got != link || err != nil {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err := s.FindByCode(ctx, link.Code); got != link || err != nil {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestConcurrentSameURL(t *testing.T) {
	const count = 500
	s := NewMemory()
	ctx := context.Background()
	var wg sync.WaitGroup
	var created atomic.Int32
	results := make(chan service.Link, count)
	errs := make(chan error, count)
	start := make(chan struct{})
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			link, isNew, err := s.Create(ctx, service.Link{Code: fmt.Sprintf("%010d", i), URL: "https://example.org/same"})
			if err != nil {
				errs <- err
				return
			}
			if isNew {
				created.Add(1)
			}
			results <- link
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if created.Load() != 1 {
		t.Fatalf("created %d mappings", created.Load())
	}
	var first service.Link
	for link := range results {
		if first.Code == "" {
			first = link
		}
		if first != link {
			t.Fatalf("different mappings: %+v %+v", first, link)
		}
	}
}

func TestConcurrentDistinctURLsAndReads(t *testing.T) {
	const count = 500
	s := NewMemory()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := range count {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			link := service.Link{Code: fmt.Sprintf("%010d", i), URL: fmt.Sprintf("https://example.org/%d", i)}
			if _, created, err := s.Create(ctx, link); err != nil || !created {
				t.Errorf("create %v %v", created, err)
				return
			}
			if got, err := s.FindByURL(ctx, link.URL); err != nil || got != link {
				t.Errorf("lookup %v %v", got, err)
			}
			if got, err := s.FindByCode(ctx, link.Code); err != nil || got != link {
				t.Errorf("lookup %v %v", got, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewMemory()
	if _, err := s.FindByCode(ctx, "0123456789"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.FindByURL(ctx, "https://example.org"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := s.Create(ctx, service.Link{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

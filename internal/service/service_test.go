package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type repositoryStub struct {
	findURL  func(context.Context, string) (Link, error)
	findCode func(context.Context, string) (Link, error)
	create   func(context.Context, Link) (Link, bool, error)
}

func (r repositoryStub) FindByURL(ctx context.Context, u string) (Link, error) {
	if r.findURL == nil {
		return Link{}, ErrNotFound
	}
	return r.findURL(ctx, u)
}
func (r repositoryStub) FindByCode(ctx context.Context, c string) (Link, error) {
	if r.findCode == nil {
		return Link{}, ErrNotFound
	}
	return r.findCode(ctx, c)
}
func (r repositoryStub) Create(ctx context.Context, l Link) (Link, bool, error) {
	if r.create == nil {
		return l, true, nil
	}
	return r.create(ctx, l)
}

func TestShorten(t *testing.T) {
	ctx := context.Background()
	const original = "https://example.org/a?x=1#part"
	existing := Link{Code: "Existing_1", URL: original}
	backendErr := errors.New("database unavailable")
	tests := []struct {
		name     string
		repo     repositoryStub
		generate func() (string, error)
		want     Link
		created  bool
		wantErr  error
	}{
		{name: "new", want: Link{Code: "0123456789", URL: original}, created: true},
		{name: "existing", repo: repositoryStub{findURL: func(context.Context, string) (Link, error) { return existing, nil }},
			generate: func() (string, error) { t.Fatal("existing URL must not need a new code"); return "", nil }, want: existing},
		{name: "concurrent winner", repo: repositoryStub{create: func(context.Context, Link) (Link, bool, error) { return existing, false, nil }}, want: existing},
		{name: "lookup error", repo: repositoryStub{findURL: func(context.Context, string) (Link, error) { return Link{}, backendErr }}, wantErr: backendErr},
		{name: "write error", repo: repositoryStub{create: func(context.Context, Link) (Link, bool, error) { return Link{}, false, backendErr }}, wantErr: backendErr},
		{name: "randomness error", generate: func() (string, error) { return "", backendErr }, wantErr: backendErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.repo)
			s.generate = func() (string, error) { return "0123456789", nil }
			if tt.generate != nil {
				s.generate = tt.generate
			}
			got, created, err := s.Shorten(ctx, original)
			if !errors.Is(err, tt.wantErr) || got != tt.want || created != tt.created {
				t.Fatalf("got (%+v,%v,%v), want (%+v,%v,%v)", got, created, err, tt.want, tt.created, tt.wantErr)
			}
		})
	}
}

func TestShortenCollision(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		t.Run(fmt.Sprint(exhausted), func(t *testing.T) {
			calls := 0
			s := New(repositoryStub{create: func(_ context.Context, l Link) (Link, bool, error) {
				calls++
				if exhausted || calls == 1 {
					return Link{}, false, ErrCodeConflict
				}
				return l, true, nil
			}})
			s.generate = func() (string, error) { return fmt.Sprintf("%010d", calls), nil }
			link, created, err := s.Shorten(context.Background(), "https://example.org")
			if exhausted {
				if !errors.Is(err, ErrGenerationExhausted) || calls != maxAttempts {
					t.Fatalf("calls=%d err=%v", calls, err)
				}
			} else if err != nil || !created || link.Code != "0000000001" || calls != 2 {
				t.Fatalf("link=%+v calls=%d err=%v", link, calls, err)
			}
		})
	}
}

func TestShortenInvalidAndCanceled(t *testing.T) {
	s := New(repositoryStub{})
	if _, _, err := s.Shorten(context.Background(), "javascript:alert(1)"); !errors.Is(err, ErrInvalidURL) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := s.Shorten(ctx, "https://example.org"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestResolve(t *testing.T) {
	expected := Link{Code: "0123456789", URL: "https://example.org"}
	s := New(repositoryStub{findCode: func(_ context.Context, code string) (Link, error) {
		if code == expected.Code {
			return expected, nil
		}
		return Link{}, ErrNotFound
	}})
	if got, err := s.Resolve(context.Background(), expected.Code); got != expected || err != nil {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := s.Resolve(context.Background(), "unknown___"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.Resolve(context.Background(), "bad"); !errors.Is(err, ErrInvalidCode) {
		t.Fatal(err)
	}
}

func TestGenerateCode(t *testing.T) {
	for range 1000 {
		code, err := GenerateCode()
		if err != nil || !ValidCode(code) {
			t.Fatalf("%q %v", code, err)
		}
	}
	// Every alphabet symbol can be selected, including uppercase and underscore.
	for i, c := range []byte(Alphabet) {
		got, err := generateCode(bytes.NewReader(bytes.Repeat([]byte{byte(i)}, CodeLength)))
		if err != nil || got != strings.Repeat(string(c), CodeLength) {
			t.Fatalf("%q %v", got, err)
		}
	}
	if _, err := generateCode(bytes.NewReader(nil)); err == nil {
		t.Fatal("expected entropy reader error")
	}
	for _, c := range []string{"", "123", "12345678901", "012345678-", "abcdefghЯ"} {
		if ValidCode(c) {
			t.Fatalf("accepted %q", c)
		}
	}
}

func TestValidateURL(t *testing.T) {
	valid := []string{"https://example.org", "http://localhost:8080/a?x=1&y=2#frag", "http://[::1]:8080", "https://example.org/путь", "https://example.org/a%20b", "https://example.org/" + strings.Repeat("x", MaxURLBytes-len("https://example.org/"))}
	for _, u := range valid {
		if err := ValidateURL(u); err != nil {
			t.Errorf("rejected %q: %v", u, err)
		}
	}
	invalid := []string{"", "example.org", "//example.org", "ftp://example.org", "javascript:alert(1)", "https:///path", "https://", "https://:80", "https://user:pass@example.org", "https://example.org:abc", "https://example.org:0", "https://example.org:65536", "https://example.org:", "https://example.org/a b", "https://example.org/\n", "https://example.org/\x00", "https://example.org/\xff", "https://example.org/%zz", "https://example.org/" + strings.Repeat("я", 1024)}
	for _, u := range invalid {
		if err := ValidateURL(u); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("accepted %q: %v", u, err)
		}
	}
}

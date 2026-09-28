package service

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxURLBytes = 2048

func ValidateURL(raw string) error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidURL, reason) }
	if len(raw) == 0 || len(raw) > MaxURLBytes || !utf8.ValidString(raw) {
		return invalid("expected 1..2048 bytes of UTF-8")
	}
	if strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return invalid("whitespace and control characters must be percent-encoded")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.Opaque != "" {
		return invalid("expected an absolute HTTP(S) URL with a host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalid("only http and https are supported")
	}
	if u.User != nil {
		return invalid("embedded credentials are not supported")
	}
	if strings.HasSuffix(u.Host, ":") {
		return invalid("port must not be empty")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid("port must be in the range 1..65535")
		}
	}
	return nil
}

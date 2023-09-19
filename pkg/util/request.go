package util

import (
	"fmt"
	"io"
	"net/http"
)

// NewRequest returns a http.Request object with kots defaults set, including a User-Agent header.
func NewRequest(method string, url string, body io.Reader, userAgent string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to call newrequest: %w", err)
	}

	req.Header.Add("User-Agent", userAgent)
	return req, nil
}

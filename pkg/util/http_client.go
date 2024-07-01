package util

import (
	"net/http"
	"time"
)

func HttpClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

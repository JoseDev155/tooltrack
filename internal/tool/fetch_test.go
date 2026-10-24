package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetcherReadsBody(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		w.Write([]byte("version = '1.2.3'"))
	}))
	defer srv.Close()
	f := newTestFetcher(srv, time.Second)
	body, err := f(context.Background(), srv.URL)
	if err != nil || string(body) != "version = '1.2.3'" {
		t.Errorf("got %q, %v", body, err)
	}
}

func TestFetcherHTTPError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := newTestFetcher(srv, time.Second)(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v, want HTTP 404", err)
	}
}

func TestFetcherRejectsPlainHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("plain http server must not be contacted")
	}))
	defer srv.Close()
	if _, err := NewFetcher(time.Second)(context.Background(), srv.URL); err == nil {
		t.Error("expected an error for an http URL")
	}
}

func TestFetcherLimitsBody(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", maxFetchBytes+10)))
	}))
	defer srv.Close()
	body, err := newTestFetcher(srv, time.Second)(context.Background(), srv.URL)
	if err != nil || len(body) != maxFetchBytes {
		t.Errorf("len = %d, err = %v", len(body), err)
	}
}

// newTestFetcher is NewFetcher trusting the test server's certificate.
func newTestFetcher(srv *httptest.Server, timeout time.Duration) Fetcher {
	return newFetcher(srv.Client(), timeout)
}

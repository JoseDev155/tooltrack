package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T, h http.HandlerFunc) *GitHubClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &GitHubClient{BaseURL: srv.URL}
}

func TestLatestReleaseSendsRequiredHeaders(t *testing.T) {
	var got *http.Request
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Write([]byte(`{"tag_name":"v0.5.1"}`))
	})
	c.Token = "secret-token"

	tag, err := c.LatestRelease(context.Background(), "astral-sh/uv")
	if err != nil || tag != "v0.5.1" {
		t.Fatalf("got %q, %v", tag, err)
	}
	if got.Method != http.MethodGet || got.URL.Path != "/repos/astral-sh/uv/releases/latest" {
		t.Errorf("request = %s %s", got.Method, got.URL.Path)
	}
	if v := got.Header.Get("User-Agent"); v != "Updater-CLI-App" {
		t.Errorf("User-Agent = %q", v)
	}
	if v := got.Header.Get("Accept"); v != "application/vnd.github.v3+json" {
		t.Errorf("Accept = %q", v)
	}
	if v := got.Header.Get("Authorization"); v != "Bearer secret-token" {
		t.Errorf("Authorization = %q", v)
	}
}

func TestLatestReleaseOmitsAuthWithoutToken(t *testing.T) {
	var auth string
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"tag_name":"v1.0.0"}`))
	})
	if _, err := c.LatestRelease(context.Background(), "a/b"); err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		t.Errorf("Authorization = %q, want none", auth)
	}
}

func TestLatestReleaseErrors(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{"not found", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }, "no published release"},
		{"rate limited 403", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(403)
		}, "rate limit exceeded"},
		{"rate limited 429", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }, "rate limit exceeded"},
		{"forbidden", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }, "access denied"},
		{"server error", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }, "HTTP status 500"},
		{"bad json", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{`)) }, "invalid response"},
		{"no tag", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }, "no tag_name"},
	}
	for _, c := range cases {
		client := newServer(t, c.handler)
		client.Token = "secret-token"
		_, err := client.LatestRelease(context.Background(), "a/b")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want containing %q", c.name, err, c.want)
			continue
		}
		if strings.Contains(err.Error(), "secret-token") {
			t.Errorf("%s: error leaks the token: %v", c.name, err)
		}
	}
}

func TestLatestReleaseRejectsBadRepo(t *testing.T) {
	c := &GitHubClient{BaseURL: "http://127.0.0.1:1"}
	for _, repo := range []string{"", "noslash", "a/b/c", "a/b?x=1", "../x", "a/..", "./b"} {
		if _, err := c.LatestRelease(context.Background(), repo); err == nil ||
			!strings.Contains(err.Error(), "invalid GitHub repository") {
			t.Errorf("repo %q: err = %v", repo, err)
		}
	}
}

func TestLatestReleaseHonoursContext(t *testing.T) {
	c := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v1"}`))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.LatestRelease(ctx, "a/b"); err == nil {
		t.Error("expected an error with a cancelled context")
	}
}

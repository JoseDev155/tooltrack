package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxFetchBytes caps the body read from a version URL.
const maxFetchBytes = 1 << 20

// Fetcher downloads a small text document, such as a release manifest.
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// NewFetcher returns a Fetcher that only accepts https URLs, so a catalog
// entry cannot make the tool talk to a plain-text endpoint.
func NewFetcher(timeout time.Duration) Fetcher {
	return newFetcher(&http.Client{}, timeout)
}

func newFetcher(client *http.Client, timeout time.Duration) Fetcher {
	client.Timeout = timeout
	return func(ctx context.Context, url string) ([]byte, error) {
		if !strings.HasPrefix(url, "https://") {
			return nil, fmt.Errorf("remote_url %q must use https", url)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Updater-CLI-App")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
	}
}

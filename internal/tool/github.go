package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DefaultGitHubURL is the GitHub REST API root.
const DefaultGitHubURL = "https://api.github.com"

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// validRepo accepts "owner/name" and rejects dot segments, which would let the
// request path escape /repos/.
func validRepo(repo string) bool {
	if !repoPattern.MatchString(repo) {
		return false
	}
	for _, part := range strings.Split(repo, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// GitHubClient reads the latest release of a repository.
type GitHubClient struct {
	// BaseURL defaults to DefaultGitHubURL; tests point it at httptest.
	BaseURL string
	// Token is an optional personal access token (raises the rate limit from
	// 60 to 5,000 requests per hour). It is only sent as a header and never
	// included in errors.
	Token string
	// HTTPClient defaults to a client with a 30 second timeout.
	HTTPClient *http.Client
}

// NewGitHubClient returns a client using the given token and timeout.
func NewGitHubClient(token string, timeout time.Duration) *GitHubClient {
	return &GitHubClient{Token: token, HTTPClient: &http.Client{Timeout: timeout}}
}

// LatestRelease returns the tag_name of the latest published release of repo
// ("owner/name").
func (c *GitHubClient) LatestRelease(ctx context.Context, repo string) (string, error) {
	if !validRepo(repo) {
		return "", fmt.Errorf("invalid GitHub repository %q (want owner/name)", repo)
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultGitHubURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Updater-CLI-App")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github %s: %w", repo, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("github %s: %w", repo, err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound:
		return "", fmt.Errorf("github %s: no published release found", repo)
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.StatusCode == http.StatusTooManyRequests {
			return "", fmt.Errorf("github %s: rate limit exceeded (set github_token or GITHUB_TOKEN to raise it)", repo)
		}
		return "", fmt.Errorf("github %s: access denied (HTTP 403)", repo)
	default:
		return "", fmt.Errorf("github %s: unexpected HTTP status %d", repo, resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("github %s: invalid response: %w", repo, err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("github %s: release has no tag_name", repo)
	}
	return release.TagName, nil
}

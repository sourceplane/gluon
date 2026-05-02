// Package remotestate provides the HTTP client, token resolution, plan
// conversion, and run-ID derivation for orun-backend remote state.
package remotestate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// TokenSource resolves the bearer token for backend requests.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// OIDCTokenSource requests a GitHub Actions OIDC token from the GitHub
// Actions token endpoint with the configured audience.
type OIDCTokenSource struct {
	Audience   string
	httpClient *http.Client
}

// NewOIDCTokenSource returns an OIDCTokenSource using the given audience
// (default "orun" if empty).
func NewOIDCTokenSource(audience string) *OIDCTokenSource {
	if audience == "" {
		audience = "orun"
	}
	return &OIDCTokenSource{
		Audience:   audience,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Token fetches a fresh OIDC token from the GitHub Actions token endpoint.
func (o *OIDCTokenSource) Token(ctx context.Context) (string, error) {
	requestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	requestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if requestURL == "" || requestToken == "" {
		return "", fmt.Errorf(
			"GitHub Actions OIDC token not available: " +
				"ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN must be set; " +
				"add `id-token: write` to your workflow permissions")
	}

	u, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("invalid ACTIONS_ID_TOKEN_REQUEST_URL: %w", err)
	}
	q := u.Query()
	q.Set("audience", o.Audience)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("building OIDC token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+requestToken)
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting OIDC token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OIDC token request returned status %d", resp.StatusCode)
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding OIDC token response: %w", err)
	}
	if result.Value == "" {
		return "", fmt.Errorf("OIDC token response missing value field")
	}
	return result.Value, nil
}

// StaticTokenSource returns a pre-configured bearer token as-is.
type StaticTokenSource struct {
	token string
}

// NewStaticTokenSource wraps a fixed bearer token (e.g. from ORUN_TOKEN).
func NewStaticTokenSource(token string) *StaticTokenSource {
	return &StaticTokenSource{token: token}
}

// Token returns the static token or an error if it is empty.
func (s *StaticTokenSource) Token(_ context.Context) (string, error) {
	if s.token == "" {
		return "", fmt.Errorf("ORUN_TOKEN is not set")
	}
	return s.token, nil
}

// ResolveTokenSource returns the appropriate TokenSource based on the current
// environment:
//
//   - GitHub Actions with id-token:write → OIDCTokenSource (audience "orun")
//   - ORUN_TOKEN set → StaticTokenSource
//   - Otherwise → error
func ResolveTokenSource() (TokenSource, error) {
	if isGitHubActionsOIDC() {
		return NewOIDCTokenSource("orun"), nil
	}
	if token := os.Getenv("ORUN_TOKEN"); token != "" {
		return NewStaticTokenSource(token), nil
	}
	return nil, fmt.Errorf(
		"no authentication token available: " +
			"in GitHub Actions add `id-token: write` to workflow permissions; " +
			"outside GitHub Actions set ORUN_TOKEN")
}

// isGitHubActionsOIDC reports whether OIDC token acquisition is possible.
func isGitHubActionsOIDC() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true" &&
		os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL") != "" &&
		os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN") != ""
}

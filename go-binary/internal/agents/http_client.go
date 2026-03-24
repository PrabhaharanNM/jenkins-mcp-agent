package agents

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"
)

const (
	httpTimeout = 30 * time.Second
	maxRetries  = 3
	baseBackoff = 500 * time.Millisecond
)

var sharedClient = &http.Client{Timeout: httpTimeout}

// doRequest executes an HTTP GET with retry logic and exponential backoff.
// authHeader is the header name (e.g. "Authorization", "X-JFrog-Art-Api")
// and authValue is the corresponding value. If authHeader is empty, no auth
// header is added.
func doRequest(ctx context.Context, url, authHeader, authValue string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(float64(baseBackoff) * math.Pow(2, float64(attempt-1)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request for %s: %w", url, err)
		}
		if authHeader != "" {
			req.Header.Set(authHeader, authValue)
		}

		resp, err := sharedClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("GET %s: %w", url, err)
			log.Printf("[agents] attempt %d failed for %s: %v", attempt+1, url, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response from %s: %w", url, err)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("GET %s returned status %d", url, resp.StatusCode)
			log.Printf("[agents] attempt %d: server error %d for %s", attempt+1, resp.StatusCode, url)
			continue
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("GET %s returned status %d: %s", url, resp.StatusCode, string(body))
		}

		return body, nil
	}
	return nil, fmt.Errorf("all %d retries exhausted for %s: %w", maxRetries, url, lastErr)
}

// basicAuthValue builds the value for an Authorization: Basic header.
func basicAuthValue(username, password string) string {
	creds := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

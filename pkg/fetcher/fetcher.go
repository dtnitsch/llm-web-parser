package fetcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	// Custom User-Agent to avoid being blocked by default.
	userAgent = "Gemini-WebParser/1.0"
)

type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a new Fetcher with a default HTTP client.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{},
	}
}

// GetHtml performs a robust two-phase fetch: HEAD then GET.
func (f *Fetcher) GetHtml(url string) (*bytes.Buffer, error) {
	// Phase 1: HEAD request with a short timeout to fail fast.
	headTimeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), headTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD request failed for %s: %w", url, err)
	}
	resp.Body.Close() // Close body even for HEAD request

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status for %s: %s", url, resp.Status)
	}

	// Phase 2: GET request with a longer timeout.
	getTimeout := 10 * time.Second
	ctx, cancel = context.WithTimeout(context.Background(), getTimeout)
	defer cancel()

	req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request for %s: %w", url, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err = f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET request failed for %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status for %s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body for %s: %w", url, err)
	}

	return bytes.NewBuffer(body), nil
}

func (f *Fetcher) ParseHtml(html *bytes.Buffer) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w", err)
	}
	return doc, nil
}

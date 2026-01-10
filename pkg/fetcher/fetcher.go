package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// FetchResponse contains enriched HTTP metadata from fetch
type FetchResponse struct {
	HTML          []byte
	StatusCode    int
	ContentType   string
	FinalURL      string // URL after following redirects
	RedirectChain []string
	Headers       http.Header
}

type Fetcher struct {
	client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{},
	}
}

func (f *Fetcher) GetHtml(url string) (*goquery.Document, error) {
    bodyBytes, err := f.GetHtmlBytes(url)
    if err != nil {
        return nil, err
    }
    
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(bodyBytes)))
    if err != nil {
        return nil, fmt.Errorf("failed to parse HTML: %w", err)
    }
    return doc, nil
}

func (f *Fetcher) GetHtmlBytes(url string) ([]byte, error) {
    resp, err := f.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to make HTTP request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("failed to fetch HTML, status code: %d", resp.StatusCode)
    }

    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response body: %w", err)
    }
    return bodyBytes, nil
}

// Fetch performs enriched HTTP fetch with metadata capture
func (f *Fetcher) Fetch(url string) (*FetchResponse, error) {
	// Track redirects
	var redirectChain []string

	// Create client with redirect tracking
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			redirectChain = append(redirectChain, req.URL.String())
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Build response
	fetchResp := &FetchResponse{
		HTML:          bodyBytes,
		StatusCode:    resp.StatusCode,
		ContentType:   resp.Header.Get("Content-Type"),
		FinalURL:      resp.Request.URL.String(),
		RedirectChain: redirectChain,
		Headers:       resp.Header,
	}

	return fetchResp, nil
}
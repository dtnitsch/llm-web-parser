package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

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
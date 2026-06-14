// Package bible is the library behind the bible command line:
// the HTTP client, request shaping, and the typed data models for bible-api.com.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
package bible

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to bible-api.com.
const DefaultUserAgent = "bible/dev (+https://github.com/tamnd/bible-cli)"

// Host is the site this client talks to.
const Host = "bible-api.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Verse is a single verse from the Bible.
type Verse struct {
	BookID   string `json:"book_id"`
	BookName string `json:"book_name"`
	Chapter  int    `json:"chapter"`
	Verse    int    `json:"verse"`
	Text     string `json:"text"`
}

// Passage is a passage (one or more verses) returned by the API.
type Passage struct {
	Reference       string  `kit:"id" json:"reference"`
	Verses          []Verse `json:"verses"`
	TranslationName string  `json:"translation_name"`
}

// Book is a single book of the Bible.
type Book struct {
	ID        string `kit:"id" json:"id"`
	Name      string `json:"name"`
	Testament string `json:"testament"`
	Chapters  int    `json:"chapters"`
}

// Client talks to bible-api.com over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 200ms
// minimum gap between requests, and five retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   5,
	}
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// GetVerses fetches one or more verses by reference (e.g. "john 3:16" or
// "romans 8:28-30"). If translation is non-empty it is passed as a query param.
func (c *Client) GetVerses(ctx context.Context, reference, translation string) (*Passage, error) {
	// bible-api.com uses + for spaces in the reference path segment.
	ref := strings.ReplaceAll(strings.TrimSpace(reference), " ", "+")
	u := BaseURL + "/" + ref
	if translation != "" {
		u += "?translation=" + translation
	}
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	var p Passage
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("decode passage: %w", err)
	}
	return &p, nil
}

// ListBooks returns all books of the Bible from the /data/web/books.json endpoint.
func (c *Client) ListBooks(ctx context.Context) ([]Book, error) {
	u := BaseURL + "/data/web/books.json"
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	var books []Book
	if err := json.Unmarshal(body, &books); err != nil {
		return nil, fmt.Errorf("decode books: %w", err)
	}
	return books, nil
}

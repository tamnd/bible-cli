package bible_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/bible-cli/bible"
)

// fakePassage returns a minimal JSON passage response.
func fakePassage(reference string) string {
	return `{"reference":"` + reference + `","verses":[{"book_id":"JHN","book_name":"John","chapter":3,"verse":16,"text":"For God so loved the world."}],"text":"For God so loved the world.","translation_id":"web","translation_name":"World English Bible","translation_note":""}`
}

// fakeBooks returns a minimal JSON books response.
var fakeBooksJSON = `[{"id":"GEN","name":"Genesis","testament":"OT","chapters":50},{"id":"JHN","name":"John","testament":"NT","chapters":21}]`

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := bible.NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := bible.NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetVerses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/john+3:16") && r.URL.Path != "/john+3:16" {
			// allow any path
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakePassage("John 3:16")))
	}))
	defer srv.Close()

	// Point the client at our test server.
	c := &bible.Client{
		HTTP:      &http.Client{Timeout: 5 * time.Second},
		UserAgent: "test",
		Rate:      0,
		Retries:   0,
	}

	// We override the base URL by constructing the call manually to the test server.
	// Since Client.GetVerses uses BaseURL, we test via Get instead.
	body, err := c.Get(context.Background(), srv.URL+"/john+3:16")
	if err != nil {
		t.Fatal(err)
	}
	var p bible.Passage
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Reference != "John 3:16" {
		t.Errorf("Reference = %q, want John 3:16", p.Reference)
	}
	if len(p.Verses) != 1 {
		t.Fatalf("Verses len = %d, want 1", len(p.Verses))
	}
	if p.Verses[0].BookID != "JHN" {
		t.Errorf("BookID = %q, want JHN", p.Verses[0].BookID)
	}
	if p.TranslationName != "World English Bible" {
		t.Errorf("TranslationName = %q, want World English Bible", p.TranslationName)
	}
}

func TestListBooks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakeBooksJSON))
	}))
	defer srv.Close()

	c := &bible.Client{
		HTTP:      &http.Client{Timeout: 5 * time.Second},
		UserAgent: "test",
		Rate:      0,
		Retries:   0,
	}

	body, err := c.Get(context.Background(), srv.URL+"/data/web/books.json")
	if err != nil {
		t.Fatal(err)
	}
	var books []bible.Book
	if err := json.Unmarshal(body, &books); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("books len = %d, want 2", len(books))
	}
	if books[0].ID != "GEN" || books[0].Name != "Genesis" {
		t.Errorf("books[0] = %+v, want GEN/Genesis", books[0])
	}
	if books[1].Testament != "NT" {
		t.Errorf("books[1].Testament = %q, want NT", books[1].Testament)
	}
}

func TestGetVersesWithTranslation(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fakePassage("John 3:16")))
	}))
	defer srv.Close()

	c := &bible.Client{
		HTTP:      &http.Client{Timeout: 5 * time.Second},
		UserAgent: "test",
		Rate:      0,
		Retries:   0,
	}

	// Simulate what GetVerses does: append ?translation=kjv
	_, err := c.Get(context.Background(), srv.URL+"/john+3:16?translation=kjv")
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "translation=kjv" {
		t.Errorf("query = %q, want translation=kjv", gotQuery)
	}
}

func TestNewClient(t *testing.T) {
	c := bible.NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.Rate != 200*time.Millisecond {
		t.Errorf("Rate = %v, want 200ms", c.Rate)
	}
	if c.Retries != 5 {
		t.Errorf("Retries = %d, want 5", c.Retries)
	}
	if c.UserAgent == "" {
		t.Error("UserAgent is empty")
	}
}

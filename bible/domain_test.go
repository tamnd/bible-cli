package bible

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "bible" {
		t.Errorf("Scheme = %q, want bible", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "bible" {
		t.Errorf("Identity.Binary = %q, want bible", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"john 3:16", "reference", "john 3:16"},
		{"romans 8:28-30", "reference", "romans 8:28-30"},
		{"Genesis", "book", "Genesis"},
		{"1 John", "book", "1 John"},
		{"Revelation 22:21", "reference", "Revelation 22:21"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil {
			t.Errorf("Classify(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)",
				tc.in, typ, id, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		uriType string
		id      string
		want    string
	}{
		{"reference", "john 3:16", "https://bible-api.com/john+3:16"},
		{"reference", "romans 8:28-30", "https://bible-api.com/romans+8:28-30"},
		{"book", "Genesis", "https://bible-api.com/Genesis+1:1"},
		{"book", "1 John", "https://bible-api.com/1+John+1:1"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil {
			t.Errorf("Locate(%q, %q) unexpected error: %v", tc.uriType, tc.id, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Locate(%q, %q) = %q, want %q", tc.uriType, tc.id, got, tc.want)
		}
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate(unknown) expected error, got nil")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Passage{Reference: "john 3:16", TranslationName: "World English Bible"}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	// kit URL-encodes spaces in URIs.
	if want := "bible://reference/john%203:16"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("bible", "genesis 1:1")
	if err != nil {
		t.Fatalf("ResolveOn: %v", err)
	}
	if got.String() != "bible://reference/genesis%201:1" {
		t.Errorf("ResolveOn = %q, want bible://reference/genesis%%201:1", got.String())
	}
}

package bible

import (
	"context"
	"regexp"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes bible as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/bible-cli/bible"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// bible:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone bible binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the bible driver. It carries no state.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "bible",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "bible",
			Short:  "A command line for the Bible.",
			Long: `A command line for the Bible.

bible reads public Bible data from bible-api.com over plain HTTPS, shapes it
into clean records, and prints output that pipes into the rest of your tools.
No API key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/bible-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// verse: fetch one or more verses by reference.
	kit.Handle(app, kit.OpMeta{
		Name:    "verse",
		Group:   "read",
		Single:  true,
		Summary: "Get verses by Bible reference",
		URIType: "reference",
		Resolver: true,
		Args:    []kit.Arg{{Name: "reference", Help: "Bible reference (e.g. 'john 3:16' or 'romans 8:28-30')"}},
	}, getVerses)

	// books: list all books of the Bible.
	kit.Handle(app, kit.OpMeta{
		Name:    "books",
		Group:   "read",
		List:    true,
		Summary: "List all books of the Bible",
		URIType: "book",
	}, listBooks)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type versesInput struct {
	Reference   string  `kit:"arg" help:"Bible reference (e.g. 'john 3:16' or 'romans 8:28-30')"`
	Translation string  `kit:"flag" help:"translation: web,kjv,asv,dra,ylt,bbe,darby" default:"web"`
	Client      *Client `kit:"inject"`
}

type booksInput struct {
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getVerses(ctx context.Context, in versesInput, emit func(*Passage) error) error {
	p, err := in.Client.GetVerses(ctx, in.Reference, in.Translation)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

func listBooks(ctx context.Context, in booksInput, emit func(*Book) error) error {
	books, err := in.Client.ListBooks(ctx)
	if err != nil {
		return mapErr(err)
	}
	for i := range books {
		if err := emit(&books[i]); err != nil {
			return err
		}
	}
	return nil
}

// bookNameRE matches a bare book name (letters, spaces, digits like "1 John").
var bookNameRE = regexp.MustCompile(`(?i)^(\d\s+)?[a-z][a-z\s]*$`)

// referenceRE matches a Bible reference: book name + chapter:verse.
var referenceRE = regexp.MustCompile(`(?i)^[\da-z][a-z\s]*\d+:\d+`)

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", "", errs.Usage("empty bible reference")
	}
	if referenceRE.MatchString(s) {
		return "reference", s, nil
	}
	if bookNameRE.MatchString(s) {
		return "book", s, nil
	}
	// Fall back to treating it as a query/reference.
	return "reference", s, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "reference":
		ref := strings.ReplaceAll(id, " ", "+")
		return BaseURL + "/" + ref, nil
	case "book":
		ref := strings.ReplaceAll(id, " ", "+") + "+1:1"
		return BaseURL + "/" + ref, nil
	default:
		return "", errs.Usage("bible has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind that carries the right
// exit code.
func mapErr(err error) error {
	return err
}

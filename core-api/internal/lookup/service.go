package lookup

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Cache lifetimes: catalogue data barely changes, but a miss may be filled
// in by the community, so it is retried sooner.
const (
	hitTTL       = 30 * 24 * time.Hour
	missTTL      = 24 * time.Hour
	lookupBudget = 2500 * time.Millisecond
	sourceNone   = "none"
)

// ErrInvalidCode marks a code that is not a plausible barcode or ISBN.
var ErrInvalidCode = errors.New("not a valid barcode or ISBN")

// Store is the slice of the generated queries this package needs. Both are
// on the family-scoped allowlist: the cache holds public catalogue data.
type Store interface {
	GetProductCache(ctx context.Context, barcode string) (db.ProductCache, error)
	UpsertProductCache(ctx context.Context, arg db.UpsertProductCacheParams) error
}

// Service answers barcode lookups from the cache or the catalogues.
type Service struct {
	store     Store
	providers []Provider
	log       *slog.Logger
	now       func() time.Time
}

// NewService builds the lookup service.
func NewService(store Store, providers []Provider, log *slog.Logger) *Service {
	return &Service{store: store, providers: providers, log: log.With(slog.String("component", "lookup")), now: time.Now}
}

// Lookup resolves one code. It never fails because a catalogue did: if every
// provider errors or knows nothing, the answer is "not found", and only a
// real answer or a confirmed miss is cached — a catalogue outage is not.
func (s *Service) Lookup(ctx context.Context, rawCode string) (Product, error) {
	code, ok := normalizeCode(rawCode)
	if !ok {
		return Product{}, ErrInvalidCode
	}

	if p, ok := s.cached(ctx, code); ok {
		return p, nil
	}

	product, allAnswered := s.ask(ctx, code)
	if product == nil {
		if allAnswered {
			s.remember(ctx, code, sourceNone, &Product{Code: code})
		}

		return Product{Code: code}, nil
	}

	s.remember(ctx, code, product.Source, product)

	return *product, nil
}

func (s *Service) cached(ctx context.Context, code string) (Product, bool) {
	row, err := s.store.GetProductCache(ctx, code)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.WarnContext(ctx, "read product cache", slog.Any("error", err))
		}

		return Product{}, false
	}

	ttl := hitTTL
	if row.Source == sourceNone {
		ttl = missTTL
	}

	if s.now().Sub(pgconv.ToTime(row.FetchedAt)) > ttl {
		return Product{}, false
	}

	var p Product
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return Product{}, false
	}

	p.Code = code

	return p, true
}

func (s *Service) remember(ctx context.Context, code, source string, p *Product) {
	payload, err := json.Marshal(p)
	if err != nil {
		return
	}

	// The caller may already have its answer and be gone; the cache write
	// should still happen.
	if err := s.store.UpsertProductCache(context.WithoutCancel(ctx), db.UpsertProductCacheParams{
		Barcode: code, Source: source, Payload: payload,
	}); err != nil {
		s.log.WarnContext(ctx, "write product cache", slog.Any("error", err))
	}
}

type answer struct {
	product *Product
	err     error
}

// ask queries every provider at once and returns the first hit. allAnswered
// is true when no provider failed, so a nil product is a true miss.
func (s *Service) ask(ctx context.Context, code string) (product *Product, allAnswered bool) {
	ctx, cancel := context.WithTimeout(ctx, lookupBudget)
	defer cancel()

	results := make(chan answer, len(s.providers))

	for _, p := range s.providers {
		go func() {
			product, err := p.Lookup(ctx, code)
			if err != nil && !errors.Is(err, errNotISBN) {
				s.log.DebugContext(ctx, "catalogue lookup failed", slog.String("provider", p.Name()), slog.Any("error", err))
			}

			results <- answer{product: product, err: err}
		}()
	}

	allAnswered = true

	for range s.providers {
		select {
		case r := <-results:
			if r.product != nil {
				return r.product, true
			}

			if r.err != nil && !errors.Is(r.err, errNotISBN) {
				allAnswered = false
			}
		case <-ctx.Done():
			return nil, false
		}
	}

	return nil, allAnswered
}

// normalizeCode strips spaces and hyphens and accepts 8–14 digits, or an
// ISBN-10 whose check digit is X.
func normalizeCode(raw string) (string, bool) {
	out := make([]byte, 0, len(raw))

	for i := 0; i < len(raw); i++ {
		c := raw[i]

		switch {
		case c == ' ' || c == '-':
		case c >= '0' && c <= '9':
			out = append(out, c)
		case (c == 'X' || c == 'x') && i == len(raw)-1 && len(out) == 9:
			out = append(out, 'X')
		default:
			return "", false
		}
	}

	if len(out) < 8 || len(out) > 14 {
		return "", false
	}

	return string(out), true
}

func isISBN(code string) bool {
	switch len(code) {
	case 10:
		return true
	case 13:
		return code[:3] == "978" || code[:3] == "979"
	default:
		return false
	}
}

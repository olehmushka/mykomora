package lookup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/olehmushka/mykomora/core-api/internal/config"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

type fakeStore struct {
	rows map[string]db.ProductCache
}

func (f *fakeStore) GetProductCache(_ context.Context, barcode string) (db.ProductCache, error) {
	row, ok := f.rows[barcode]
	if !ok {
		return db.ProductCache{}, pgx.ErrNoRows
	}

	return row, nil
}

func (f *fakeStore) UpsertProductCache(_ context.Context, arg db.UpsertProductCacheParams) error {
	f.rows[arg.Barcode] = db.ProductCache{
		Barcode: arg.Barcode, Source: arg.Source, Payload: arg.Payload, FetchedAt: pgconv.Timestamptz(time.Now()),
	}

	return nil
}

func newTestService(t *testing.T, cfg *config.Config) (*Service, *fakeStore) {
	t.Helper()

	store := &fakeStore{rows: map[string]db.ProductCache{}}
	svc := NewService(store, Providers(&http.Client{Timeout: time.Second}, cfg), slog.New(slog.NewTextHandler(io.Discard, nil)))

	return svc, store
}

func facts(t *testing.T, body string, hits *atomic.Int32) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

const miss = `{"status":0}`

func TestLookupTakesTheFirstHitAndCachesIt(t *testing.T) {
	var foodHits, other atomic.Int32

	food := facts(t, `{"status":1,"product":{"product_name":"Oat milk","brands":"Alpro, Other","categories":"Drinks, Plant milks"}}`, &foodHits)
	beauty := facts(t, miss, &other)
	products := facts(t, miss, &other)
	books := facts(t, `{}`, &other)

	svc, store := newTestService(t, &config.Config{
		OpenFoodFactsURL: food.URL, OpenBeautyFactsURL: beauty.URL, OpenProductsFactsURL: products.URL, GoogleBooksURL: books.URL,
	})

	got, err := svc.Lookup(context.Background(), "5411188119098")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if !got.Found || got.Name != "Oat milk" || got.Brand != "Alpro" || got.CategoryHint != "Plant milks" || got.Source != "openfoodfacts" {
		t.Fatalf("Lookup = %+v", got)
	}

	if _, ok := store.rows["5411188119098"]; !ok {
		t.Fatal("hit was not cached")
	}

	if _, err := svc.Lookup(context.Background(), "5411188119098"); err != nil {
		t.Fatalf("second Lookup: %v", err)
	}

	if foodHits.Load() != 1 {
		t.Errorf("catalogue was asked %d times, want 1 (second answer from cache)", foodHits.Load())
	}
}

func TestLookupFindsABookByISBN(t *testing.T) {
	var hits atomic.Int32

	none := facts(t, miss, &hits)
	books := facts(t, `{"items":[{"volumeInfo":{"title":"Kobzar","authors":["Taras Shevchenko"],"categories":["Poetry"],"imageLinks":{"thumbnail":"http://img/x"}}}]}`, &hits)

	svc, _ := newTestService(t, &config.Config{
		OpenFoodFactsURL: none.URL, OpenBeautyFactsURL: none.URL, OpenProductsFactsURL: none.URL, GoogleBooksURL: books.URL,
	})

	got, err := svc.Lookup(context.Background(), "978-966-03-1234-5")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}

	if got.Name != "Kobzar" || got.Brand != "Taras Shevchenko" || got.ImageURL != "https://img/x" || got.Source != "googlebooks" {
		t.Fatalf("Lookup = %+v", got)
	}
}

func TestLookupRemembersAConfirmedMissButNotAnOutage(t *testing.T) {
	var hits atomic.Int32

	none := facts(t, miss, &hits)
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	t.Cleanup(down.Close)

	// A plain EAN skips Google Books, so three confirmed misses are a true miss.
	svc, store := newTestService(t, &config.Config{
		OpenFoodFactsURL: none.URL, OpenBeautyFactsURL: none.URL, OpenProductsFactsURL: none.URL, GoogleBooksURL: none.URL,
	})

	got, err := svc.Lookup(context.Background(), "4006381333931")
	if err != nil || got.Found {
		t.Fatalf("Lookup = %+v, %v; want not found", got, err)
	}

	if store.rows["4006381333931"].Source != sourceNone {
		t.Error("a confirmed miss was not cached")
	}

	svc, store = newTestService(t, &config.Config{
		OpenFoodFactsURL: down.URL, OpenBeautyFactsURL: none.URL, OpenProductsFactsURL: none.URL, GoogleBooksURL: none.URL,
	})

	if got, err := svc.Lookup(context.Background(), "4006381333931"); err != nil || got.Found {
		t.Fatalf("Lookup during an outage = %+v, %v; want not found", got, err)
	}

	if _, cached := store.rows["4006381333931"]; cached {
		t.Error("a catalogue outage was cached as a miss")
	}
}

func TestLookupGivesUpWhenCataloguesAreSlow(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	t.Cleanup(slow.Close)

	svc, _ := newTestService(t, &config.Config{
		OpenFoodFactsURL: slow.URL, OpenBeautyFactsURL: slow.URL, OpenProductsFactsURL: slow.URL, GoogleBooksURL: slow.URL,
	})

	start := time.Now()

	got, err := svc.Lookup(context.Background(), "4006381333931")
	if err != nil || got.Found {
		t.Fatalf("Lookup = %+v, %v; want a quick not found", got, err)
	}

	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Errorf("Lookup took %v; the budget is %v", elapsed, lookupBudget)
	}
}

func TestInvalidCodesAreRejected(t *testing.T) {
	svc, _ := newTestService(t, &config.Config{})

	for _, code := range []string{"", "123", "abc12345678", "123456789012345", "12345678a"} {
		if _, err := svc.Lookup(context.Background(), code); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("Lookup(%q) error = %v, want ErrInvalidCode", code, err)
		}
	}

	if got, ok := normalizeCode("0-306-40615-x"); !ok || got != "030640615X" {
		t.Errorf("normalizeCode ISBN-10 with X = %q, %v", got, ok)
	}
}

package lookup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olehmushka/mykomora/core-api/internal/config"
)

// Product is the normalised answer, whichever catalogue gave it.
type Product struct {
	Code         string `json:"code"`
	Found        bool   `json:"found"`
	Name         string `json:"name,omitempty"`
	Brand        string `json:"brand,omitempty"`
	CategoryHint string `json:"categoryHint,omitempty"`
	ImageURL     string `json:"imageUrl,omitempty"`
	Source       string `json:"source,omitempty"`
}

// Provider asks one catalogue about one code. A code the catalogue does not
// know is (nil, nil), not an error; an error means the catalogue could not
// answer, and only counts against that provider.
type Provider interface {
	Name() string
	Lookup(ctx context.Context, code string) (*Product, error)
}

const userAgent = "mykomora/1.0 (family inventory; +https://github.com/olehmushka/mykomora)"

// Providers builds the catalogues, in no particular order — the service asks
// them all at once.
func Providers(client *http.Client, cfg *config.Config) []Provider {
	return []Provider{
		&openFacts{client: client, name: "openfoodfacts", base: cfg.OpenFoodFactsURL},
		&openFacts{client: client, name: "openbeautyfacts", base: cfg.OpenBeautyFactsURL},
		&openFacts{client: client, name: "openproductsfacts", base: cfg.OpenProductsFactsURL},
		&googleBooks{client: client, base: cfg.GoogleBooksURL, key: cfg.GoogleBooksAPIKey},
	}
}

func getJSON(ctx context.Context, client *http.Client, target string, out any) (found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request: %w", err)
	}

	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close response: %w", cerr)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}

	return true, nil
}

// --- Open Food / Beauty / Products Facts -------------------------------

type openFacts struct {
	client *http.Client
	name   string
	base   string
}

func (o *openFacts) Name() string { return o.name }

func (o *openFacts) Lookup(ctx context.Context, code string) (*Product, error) {
	var body struct {
		Status  int `json:"status"`
		Product struct {
			Name       string `json:"product_name"`
			Brands     string `json:"brands"`
			Categories string `json:"categories"`
			Image      string `json:"image_front_url"`
		} `json:"product"`
	}

	target := fmt.Sprintf("%s/api/v2/product/%s.json?fields=product_name,brands,categories,image_front_url",
		strings.TrimRight(o.base, "/"), url.PathEscape(code))

	found, err := getJSON(ctx, o.client, target, &body)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(body.Product.Name)
	if !found || body.Status != 1 || name == "" {
		return nil, nil
	}

	return &Product{
		Code: code, Found: true, Name: name,
		Brand:        firstCSV(body.Product.Brands),
		CategoryHint: lastCSV(body.Product.Categories),
		ImageURL:     body.Product.Image,
		Source:       o.name,
	}, nil
}

// --- Google Books ------------------------------------------------------

type googleBooks struct {
	client *http.Client
	base   string
	key    string
}

func (g *googleBooks) Name() string { return "googlebooks" }

var errNotISBN = errors.New("not an ISBN")

func (g *googleBooks) Lookup(ctx context.Context, code string) (*Product, error) {
	if !isISBN(code) {
		return nil, errNotISBN
	}

	var body struct {
		Items []struct {
			VolumeInfo struct {
				Title      string   `json:"title"`
				Subtitle   string   `json:"subtitle"`
				Authors    []string `json:"authors"`
				Categories []string `json:"categories"`
				ImageLinks struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}

	q := url.Values{"q": {"isbn:" + code}, "maxResults": {"1"}}
	if g.key != "" {
		q.Set("key", g.key)
	}

	found, err := getJSON(ctx, g.client, strings.TrimRight(g.base, "/")+"/volumes?"+q.Encode(), &body)
	if err != nil {
		return nil, err
	}

	if !found || len(body.Items) == 0 || strings.TrimSpace(body.Items[0].VolumeInfo.Title) == "" {
		return nil, nil
	}

	v := body.Items[0].VolumeInfo
	p := &Product{
		Code: code, Found: true, Name: strings.TrimSpace(v.Title),
		Brand:    strings.Join(v.Authors, ", "),
		ImageURL: strings.Replace(v.ImageLinks.Thumbnail, "http://", "https://", 1),
		Source:   "googlebooks",
	}

	if len(v.Categories) > 0 {
		p.CategoryHint = v.Categories[0]
	}

	return p, nil
}

// --- helpers -----------------------------------------------------------

func firstCSV(s string) string {
	first, _, _ := strings.Cut(s, ",")

	return strings.TrimSpace(first)
}

func lastCSV(s string) string {
	parts := strings.Split(s, ",")

	return strings.TrimSpace(parts[len(parts)-1])
}

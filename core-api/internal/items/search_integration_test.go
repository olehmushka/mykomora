//go:build integration

// M5's own Done-when sentence, against a real Postgres: «де зимові чоботи 32
// розміру» finds the boots and names the box and the room. Also what only a
// database can prove about the rest of the search: nested items are reachable,
// keyset pages neither repeat nor skip, and another family never leaks in.
package items_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func listPage(t *testing.T, f fixture, params api.ListItemsParams) api.ItemPage {
	t.Helper()

	resp, err := f.h.ListItems(f.ctx, api.ListItemsRequestObject{Params: params})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	page, ok := resp.(api.ListItems200JSONResponse)
	if !ok {
		t.Fatalf("ListItems returned %T, want a 200: %+v", resp, resp)
	}

	return api.ItemPage(page)
}

func search(t *testing.T, f fixture, q string) api.ItemPage {
	t.Helper()

	return listPage(t, f, api.ListItemsParams{Q: &q})
}

func TestSearchFindsWinterBootsAndNamesTheBoxAndRoom(t *testing.T) {
	f := newFixture(t)

	box := createItem(t, f, api.CreateItemRequest{
		Name: "Коробка зі взуттям", CategoryId: f.categoryID,
		LocationId: &f.locationID, RoomId: &f.roomAID, IsContainer: true, Quantity: 1,
	})

	winter := api.Winter
	size := "32"

	boots := createItem(t, f, api.CreateItemRequest{
		Name: "Чоботи", CategoryId: f.categoryID, ParentItemId: &box.Id, Quantity: 1,
		Season: &winter, SizeValue: &size,
	})

	// Decoys that must rank below, or not appear at all.
	summer := api.Summer
	createItem(t, f, api.CreateItemRequest{
		Name: "Літні чоботи", CategoryId: f.categoryID, ParentItemId: &box.Id, Quantity: 1, Season: &summer, SizeValue: &size,
	})
	createItem(t, f, api.CreateItemRequest{Name: "Ґумова качка", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})

	page := search(t, f, "де зимові чоботи 32 розміру")
	if len(page.Items) == 0 {
		t.Fatal("no hits for the milestone's own query")
	}

	top := page.Items[0]
	if top.Item.Id != boots.Id {
		t.Fatalf("top hit = %q, want the winter boots %q", top.Item.Name, boots.Name)
	}

	if len(top.Placement.ContainerPath) != 1 || top.Placement.ContainerPath[0].Id != box.Id {
		t.Errorf("container path = %+v, want the box", top.Placement.ContainerPath)
	}

	if top.Placement.Room == nil || top.Placement.Room.Id != f.roomAID || top.Placement.Room.Name != "Bedroom" {
		t.Errorf("room = %+v, want Bedroom", top.Placement.Room)
	}

	for _, hit := range page.Items {
		if hit.Item.Name == "Ґумова качка" {
			t.Error("an unrelated item matched")
		}
	}
}

func TestSearchIsPrefixAndCaseInsensitive(t *testing.T) {
	f := newFixture(t)
	createItem(t, f, api.CreateItemRequest{Name: "Дриль Bosch", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})

	for _, q := range []string{"дри", "ДРИЛЬ", "bosch", "дрил bos"} {
		if got := search(t, f, q); len(got.Items) != 1 {
			t.Errorf("search(%q) = %d hits, want 1", q, len(got.Items))
		}
	}

	if got := search(t, f, "молоток"); len(got.Items) != 0 {
		t.Errorf("search(молоток) = %d hits, want 0", len(got.Items))
	}
}

func TestBrowseListsRootsOnlyButSearchReachesNestedItems(t *testing.T) {
	f := newFixture(t)

	box := createItem(t, f, api.CreateItemRequest{
		Name: "Коробка", CategoryId: f.categoryID, LocationId: &f.locationID, RoomId: &f.roomAID, IsContainer: true, Quantity: 1,
	})
	createItem(t, f, api.CreateItemRequest{Name: "Шарф", CategoryId: f.categoryID, ParentItemId: &box.Id, Quantity: 1})

	browse := listPage(t, f, api.ListItemsParams{RoomId: &f.roomAID})
	if len(browse.Items) != 1 || browse.Items[0].Item.Id != box.Id {
		t.Fatalf("browse by room = %d items, want only the box", len(browse.Items))
	}

	found := listPage(t, f, api.ListItemsParams{RoomId: &f.roomAID, ContainerId: &box.Id})
	if len(found.Items) != 1 || found.Items[0].Item.Name != "Шарф" {
		t.Fatalf("contents = %+v, want the scarf", found.Items)
	}

	// A place filter plus words looks inside containers too.
	q := "шарф"
	if got := listPage(t, f, api.ListItemsParams{Q: &q, RoomId: &f.roomAID}); len(got.Items) != 1 {
		t.Errorf("room + words = %d hits, want 1 (the scarf, via its box's room)", len(got.Items))
	}

	if got := listPage(t, f, api.ListItemsParams{Q: &q, RoomId: &f.roomBID}); len(got.Items) != 0 {
		t.Errorf("wrong room = %d hits, want 0", len(got.Items))
	}
}

func TestKeysetPagesNeitherRepeatNorSkip(t *testing.T) {
	for _, sort := range []api.ListItemsParamsSort{"created", "name"} {
		t.Run(string(sort), func(t *testing.T) {
			f := newFixture(t)

			const total = 7

			for i := range total {
				createItem(t, f, api.CreateItemRequest{
					Name: "Річ " + string(rune('A'+i%3)), CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
				})
			}

			seen := map[uuid.UUID]bool{}
			limit := int32(3)
			params := api.ListItemsParams{Sort: &sort, Limit: &limit}

			for range 5 {
				page := listPage(t, f, params)
				for _, hit := range page.Items {
					if seen[hit.Item.Id] {
						t.Fatalf("%s listed twice across pages", hit.Item.Id)
					}

					seen[hit.Item.Id] = true
				}

				if page.NextCursor == nil {
					break
				}

				params.Cursor = page.NextCursor
			}

			if len(seen) != total {
				t.Errorf("paged through %d items, want %d", len(seen), total)
			}
		})
	}
}

func TestSearchNeverCrossesFamilies(t *testing.T) {
	mine := newFixture(t)
	other := newFixture(t)

	createItem(t, other, api.CreateItemRequest{Name: "Чужий дриль", CategoryId: other.categoryID, LocationId: &other.locationID, Quantity: 1})

	if got := search(t, mine, "дриль"); len(got.Items) != 0 {
		t.Fatalf("saw %d items from another family", len(got.Items))
	}

	if resp, _ := mine.h.FindSimilarItems(mine.ctx, api.FindSimilarItemsRequestObject{Params: api.FindSimilarItemsParams{Name: "Чужий дриль"}}); resp != nil {
		if hints, ok := resp.(api.FindSimilarItems200JSONResponse); ok && len(hints) != 0 {
			t.Fatalf("similar hints leaked %d items from another family", len(hints))
		}
	}
}

func TestSimilarItemsHintsAtDuplicates(t *testing.T) {
	f := newFixture(t)
	createItem(t, f, api.CreateItemRequest{Name: "Дитячий велосипед", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})
	createItem(t, f, api.CreateItemRequest{Name: "Парасолька", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})

	resp, err := f.h.FindSimilarItems(f.ctx, api.FindSimilarItemsRequestObject{Params: api.FindSimilarItemsParams{Name: "дитячий велосипед"}})
	if err != nil {
		t.Fatalf("FindSimilarItems: %v", err)
	}

	hints, ok := resp.(api.FindSimilarItems200JSONResponse)
	if !ok || len(hints) != 1 || hints[0].Name != "Дитячий велосипед" {
		t.Fatalf("hints = %+v, want only the bicycle", resp)
	}
}

func TestSortByRelevanceNeedsAQuery(t *testing.T) {
	f := newFixture(t)
	relevance := api.ListItemsParamsSort("relevance")

	resp, err := f.h.ListItems(f.ctx, api.ListItemsRequestObject{Params: api.ListItemsParams{Sort: &relevance}})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	if _, ok := resp.(api.ListItems400JSONResponse); !ok {
		t.Fatalf("got %T, want a 400", resp)
	}
}

func TestDashboardCountsNestedItemsUnderTheirBoxsLocation(t *testing.T) {
	f := newFixture(t)

	box := createItem(t, f, api.CreateItemRequest{
		Name: "Коробка", CategoryId: f.categoryID, LocationId: &f.locationID, RoomId: &f.roomAID, IsContainer: true, Quantity: 1,
	})
	createItem(t, f, api.CreateItemRequest{Name: "Шарф", CategoryId: f.categoryID, ParentItemId: &box.Id, Quantity: 1})

	resp, err := f.h.GetDashboard(f.ctx, api.GetDashboardRequestObject{})
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}

	d, ok := resp.(api.GetDashboard200JSONResponse)
	if !ok {
		t.Fatalf("got %T, want a 200", resp)
	}

	if d.TotalItems != 2 || len(d.ByLocation) != 1 || d.ByLocation[0].Count != 2 {
		t.Errorf("dashboard = %+v, want 2 items under the one location", d)
	}

	if len(d.RecentEvents) == 0 {
		t.Error("no recent events")
	}
}

// SPEC 8: search p95 under 300 ms. A family tops out around 10k–100k items;
// this seeds 20k and checks both the index use (by timing) and the answer.
func TestSearchStaysFastAtFamilyScale(t *testing.T) {
	f := newFixture(t)

	winter := api.Winter
	size := "32"
	createItem(t, f, api.CreateItemRequest{
		Name: "Зимові чоботи", CategoryId: f.categoryID, LocationId: &f.locationID, RoomId: &f.roomAID,
		Quantity: 1, Season: &winter, SizeValue: &size,
	})

	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO items (family_id, name, category_id, location_id, status, created_by, brand, notes)
		SELECT $1, 'Річ ' || n || ' ' || (ARRAY['куртка','кросівки','шарф','книга','крем','іграшка'])[1 + n % 6],
		       $2, $3, 'stored', (SELECT created_by FROM items WHERE family_id = $1 LIMIT 1),
		       'Бренд' || (n % 50), 'нотатки ' || n
		FROM generate_series(1, 20000) AS n`, f.familyID, f.categoryID, f.locationID)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := f.pool.Exec(f.ctx, "ANALYZE items"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	var worst time.Duration

	for range 10 {
		start := time.Now()
		page := search(t, f, "де зимові чоботи 32 розміру")
		took := time.Since(start)
		worst = max(worst, took)
		t.Logf("search took %s", took)

		if len(page.Items) == 0 || page.Items[0].Item.Name != "Зимові чоботи" {
			t.Fatalf("top hit wrong among 20k items: %+v", page.Items)
		}
	}

	if worst > 300*time.Millisecond {
		t.Errorf("slowest of 10 searches = %s, want under 300ms", worst)
	}

	t.Logf("slowest of 10 searches over 20k items: %s", worst)
}

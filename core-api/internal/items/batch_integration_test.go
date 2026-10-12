//go:build integration

package items_test

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func TestBatchCreatesEveryRowWithItsOwnEvent(t *testing.T) {
	f := newFixture(t)

	rows := []api.CreateItemRequest{
		{Name: "Bodysuit 62", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1},
		{Name: "Bodysuit 68", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1},
	}

	resp, err := f.h.CreateItemsBatch(f.ctx, api.CreateItemsBatchRequestObject{Body: &api.CreateItemsBatchRequest{Items: rows}})
	if err != nil {
		t.Fatalf("CreateItemsBatch: %v", err)
	}

	created, ok := resp.(api.CreateItemsBatch201JSONResponse)
	if !ok || len(created) != 2 {
		t.Fatalf("CreateItemsBatch returned %T (%+v), want a 201 with 2 items", resp, resp)
	}

	for _, item := range created {
		if got := len(listEvents(t, f, item.Id)); got != 1 {
			t.Errorf("item %q has %d events, want 1", item.Name, got)
		}
	}
}

func TestBatchWithOneBadRowCreatesNothing(t *testing.T) {
	f := newFixture(t)

	rows := []api.CreateItemRequest{
		{Name: "Good", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1},
		{Name: "  ", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1},
	}

	resp, err := f.h.CreateItemsBatch(f.ctx, api.CreateItemsBatchRequestObject{Body: &api.CreateItemsBatchRequest{Items: rows}})
	if err != nil {
		t.Fatalf("CreateItemsBatch: %v", err)
	}

	bad, ok := resp.(api.CreateItemsBatch400JSONResponse)
	if !ok {
		t.Fatalf("CreateItemsBatch returned %T, want a 400", resp)
	}

	if !strings.Contains(bad.Message, "row 2") {
		t.Errorf("error %q does not name row 2", bad.Message)
	}

	list, err := f.h.ListItems(f.ctx, api.ListItemsRequestObject{})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	if page, ok := list.(api.ListItems200JSONResponse); !ok || len(page.Items) != 0 {
		t.Errorf("ListItems after a rejected batch = %+v, want empty", list)
	}
}

func TestDuplicateCopiesFieldsAndRecordsItsSource(t *testing.T) {
	f := newFixture(t)

	brand := "Zara"
	src := createItem(t, f, api.CreateItemRequest{
		Name: "Jacket", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1, Brand: &brand,
	})

	count := 2
	resp, err := f.h.DuplicateItem(f.ctx, api.DuplicateItemRequestObject{ItemId: src.Id, Body: &api.DuplicateItemRequest{Count: &count}})
	if err != nil {
		t.Fatalf("DuplicateItem: %v", err)
	}

	copies, ok := resp.(api.DuplicateItem201JSONResponse)
	if !ok || len(copies) != 2 {
		t.Fatalf("DuplicateItem returned %T (%+v), want a 201 with 2 items", resp, resp)
	}

	for _, c := range copies {
		if c.Id == src.Id || c.Name != "Jacket" || c.Brand == nil || *c.Brand != "Zara" {
			t.Errorf("copy %+v does not mirror the source", c)
		}
	}
}

func TestExportWritesHeaderAndEveryLiveItem(t *testing.T) {
	f := newFixture(t)

	createItem(t, f, api.CreateItemRequest{Name: "=Lamp", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 2})
	createItem(t, f, api.CreateItemRequest{Name: "Chair", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})

	resp, err := f.h.ExportItems(f.ctx, api.ExportItemsRequestObject{})
	if err != nil {
		t.Fatalf("ExportItems: %v", err)
	}

	csvResp, ok := resp.(api.ExportItems200TextcsvResponse)
	if !ok {
		t.Fatalf("ExportItems returned %T, want a 200", resp)
	}

	records, err := csv.NewReader(csvResp.Body).ReadAll()
	if err != nil {
		t.Fatalf("read exported csv: %v", err)
	}

	if len(records) != 3 || records[0][0] != "name" {
		t.Fatalf("exported %d records (%v), want a header and 2 items", len(records), records)
	}

	names := map[string]bool{records[1][0]: true, records[2][0]: true}
	if !names["'=Lamp"] || !names["Chair"] {
		t.Errorf("exported names %v, want the formula-looking one quoted", names)
	}
}

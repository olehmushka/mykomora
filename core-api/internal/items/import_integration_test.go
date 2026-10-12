//go:build integration

package items_test

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func (f fixture) importKey() string {
	return fmt.Sprintf("families/%s/imports/%s.csv", f.familyID, uuid.NewString())
}

func (f fixture) upload(t *testing.T, csv string) string {
	t.Helper()

	key := f.importKey()
	f.storage.putData(key, "text/csv", []byte(csv))

	return key
}

func (f fixture) inspect(t *testing.T, key string) api.ImportInspection {
	t.Helper()

	resp, err := f.h.InspectImport(f.ctx, api.InspectImportRequestObject{Body: &api.ImportInspectRequest{StorageKey: key}})
	if err != nil {
		t.Fatalf("InspectImport: %v", err)
	}

	got, ok := resp.(api.InspectImport200JSONResponse)
	if !ok {
		t.Fatalf("InspectImport returned %T: %+v", resp, resp)
	}

	return api.ImportInspection(got)
}

func (f fixture) importRequest(key string, mapping map[string]string) api.ImportRequest {
	return api.ImportRequest{
		StorageKey: key, Mapping: mapping,
		Defaults: &api.ImportDefaults{CategoryId: &f.categoryID, LocationId: &f.locationID},
	}
}

func TestImportDryRunNamesBadRowsAndCommitRefusesThem(t *testing.T) {
	f := newFixture(t)

	// Semicolon-delimited, as a Ukrainian-locale Excel writes it.
	key := f.upload(t, "Назва;Кількість;Сезон\nКуртка;1;winter\nШапка;two;winter\nШарф;1;monsoon\n")

	inspection := f.inspect(t, key)
	if inspection.RowCount != 3 || inspection.SuggestedMapping["name"] != "Назва" || inspection.SuggestedMapping["quantity"] != "Кількість" {
		t.Fatalf("inspection = %+v", inspection)
	}

	req := f.importRequest(key, inspection.SuggestedMapping)

	resp, err := f.h.DryRunImport(f.ctx, api.DryRunImportRequestObject{Body: &req})
	if err != nil {
		t.Fatalf("DryRunImport: %v", err)
	}

	report, ok := resp.(api.DryRunImport200JSONResponse)
	if !ok {
		t.Fatalf("DryRunImport returned %T: %+v", resp, resp)
	}

	if report.TotalRows != 3 || report.ValidRows != 1 || len(report.Errors) != 2 || report.Errors[0].Row != 2 || report.Errors[1].Row != 3 {
		t.Fatalf("report = %+v, want 1 valid row and errors on rows 2 and 3", report)
	}

	commit, err := f.h.CommitImport(f.ctx, api.CommitImportRequestObject{Body: &req})
	if err != nil {
		t.Fatalf("CommitImport: %v", err)
	}

	if _, ok := commit.(api.CommitImport400JSONResponse); !ok {
		t.Fatalf("CommitImport with bad rows returned %T, want a 400", commit)
	}

	if n := countItems(t, f); n != 0 {
		t.Errorf("a refused import left %d items behind", n)
	}
}

func TestImportRejectsAnotherFamilysFile(t *testing.T) {
	f := newFixture(t)

	key := fmt.Sprintf("families/%s/imports/%s.csv", uuid.NewString(), uuid.NewString())
	f.storage.putData(key, "text/csv", []byte("name\nSpy\n"))

	resp, err := f.h.InspectImport(f.ctx, api.InspectImportRequestObject{Body: &api.ImportInspectRequest{StorageKey: key}})
	if err != nil {
		t.Fatalf("InspectImport: %v", err)
	}

	if _, ok := resp.(api.InspectImport400JSONResponse); !ok {
		t.Fatalf("InspectImport of another family's key returned %T, want a 400", resp)
	}
}

func TestExportedFileImportsBack(t *testing.T) {
	f := newFixture(t)

	createItem(t, f, api.CreateItemRequest{Name: "=Lamp", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 2})
	createItem(t, f, api.CreateItemRequest{Name: "Chair", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1})

	exported, err := f.h.ExportItems(f.ctx, api.ExportItemsRequestObject{})
	if err != nil {
		t.Fatalf("ExportItems: %v", err)
	}

	data, err := io.ReadAll(exported.(api.ExportItems200TextcsvResponse).Body)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}

	key := f.upload(t, string(data))
	inspection := f.inspect(t, key)

	req := api.ImportRequest{StorageKey: key, Mapping: inspection.SuggestedMapping}

	resp, err := f.h.CommitImport(f.ctx, api.CommitImportRequestObject{Body: &req})
	if err != nil {
		t.Fatalf("CommitImport: %v", err)
	}

	result, ok := resp.(api.CommitImport201JSONResponse)
	if !ok || result.Created != 2 {
		t.Fatalf("CommitImport returned %T (%+v), want 2 created", resp, resp)
	}

	if n := countItems(t, f); n != 4 {
		t.Errorf("after the round trip there are %d items, want 4", n)
	}

	list, err := f.h.ListItems(f.ctx, api.ListItemsRequestObject{})
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}

	lamps := 0

	for _, hit := range list.(api.ListItems200JSONResponse).Items {
		if hit.Item.Name == "=Lamp" {
			lamps++
		}
	}

	if lamps != 2 {
		t.Errorf("found %d items named =Lamp, want 2 (the formula guard must round-trip)", lamps)
	}

	if _, err := f.storage.GetObject(context.Background(), key, 1<<20); err == nil {
		t.Error("the committed file was not deleted")
	}
}

func countItems(t *testing.T, f fixture) int {
	t.Helper()

	var n int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM items WHERE family_id = $1", f.familyID).Scan(&n); err != nil {
		t.Fatalf("count items: %v", err)
	}

	return n
}

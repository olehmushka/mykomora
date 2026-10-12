package items

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
	"github.com/olehmushka/mykomora/core-api/internal/storage"
)

const (
	maxImportBytes  = 5 << 20
	maxImportRows   = 2000
	maxReportErrors = 100
	previewRowCount = 5
)

var importContentTypes = map[string]struct{}{
	"text/csv": {}, "application/csv": {}, "application/vnd.ms-excel": {}, "text/plain": {},
}

func invalidImport(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_import", Message: message}
}

// --- service -------------------------------------------------------------

// PresignImport mints a presigned PUT for a CSV, under the family's import
// prefix. The file is not tied to any item; the orphaned-object sweeper
// removes it after a day if it is never committed.
func (s *Service) PresignImport(ctx context.Context, principal auth.Principal, body api.ImportPresignRequest) (api.PresignUploadResult, error) {
	if err := validateUploadMeta(body.ContentType, body.ContentLength, importContentTypes, maxImportBytes); err != nil {
		return api.PresignUploadResult{}, err
	}

	key := fmt.Sprintf("families/%s/imports/%s.csv", principal.FamilyID, uuid.NewString())

	url, headers, expiresAt, err := s.storage.PresignPut(ctx, key, body.ContentType, body.ContentLength)
	if err != nil {
		return api.PresignUploadResult{}, fmt.Errorf("presign put: %w", err)
	}

	return api.PresignUploadResult{StorageKey: key, UploadUrl: url, UploadHeaders: headers, ExpiresAt: expiresAt}, nil
}

// InspectImport reads an uploaded CSV's header and suggests a mapping.
func (s *Service) InspectImport(ctx context.Context, principal auth.Principal, key string) (api.ImportInspection, error) {
	file, err := s.loadImport(ctx, principal, key)
	if err != nil {
		return api.ImportInspection{}, err
	}

	rows := 0

	for _, rec := range file.records {
		if !blankRecord(rec) {
			rows++
		}
	}

	return api.ImportInspection{
		Headers: file.headers, RowCount: rows, Fields: csvColumns, SuggestedMapping: suggestMapping(file.headers),
	}, nil
}

// DryRunImport validates every row and reports; it writes nothing.
func (s *Service) DryRunImport(ctx context.Context, principal auth.Principal, body api.ImportRequest) (api.ImportReport, error) {
	plan, err := s.planImport(ctx, principal, body)
	if err != nil {
		return api.ImportReport{}, err
	}

	report := api.ImportReport{
		TotalRows: plan.total, ValidRows: len(plan.items), Errors: plan.errors, Preview: []api.ImportPreviewRow{},
	}

	for i := 0; i < len(plan.items) && i < previewRowCount; i++ {
		report.Preview = append(report.Preview, api.ImportPreviewRow{Row: plan.rowNumbers[i], Name: plan.items[i].params.Name})
	}

	return report, nil
}

// CommitImport creates every row in one transaction, or nothing.
func (s *Service) CommitImport(ctx context.Context, principal auth.Principal, body api.ImportRequest) (api.ImportResult, error) {
	plan, err := s.planImport(ctx, principal, body)
	if err != nil {
		return api.ImportResult{}, err
	}

	if len(plan.errors) > 0 {
		return api.ImportResult{}, &ValidationError{Msg: fmt.Sprintf(
			"%d rows have problems (first: row %d: %s) — nothing was imported; run the dry run to see them all",
			plan.errorCount, plan.errors[0].Row, plan.errors[0].Message)}
	}

	if len(plan.items) == 0 {
		return api.ImportResult{}, &ValidationError{Msg: "the file has no rows to import"}
	}

	created, err := s.insertRows(ctx, principal, plan.items)
	if err != nil {
		return api.ImportResult{}, err
	}

	if err := s.storage.DeleteObject(ctx, body.StorageKey); err != nil {
		s.log.WarnContext(ctx, "delete committed import", "key", body.StorageKey, "error", err)
	}

	return api.ImportResult{Created: len(created)}, nil
}

func (s *Service) loadImport(ctx context.Context, principal auth.Principal, key string) (parsedCSV, error) {
	prefix := fmt.Sprintf("families/%s/imports/", principal.FamilyID)
	if !strings.HasPrefix(key, prefix) || strings.Contains(key[len(prefix):], "/") {
		return parsedCSV{}, &ValidationError{Msg: "that is not one of this family's uploaded files"}
	}

	data, err := s.storage.GetObject(ctx, key, maxImportBytes)

	switch {
	case errors.Is(err, storage.ErrObjectNotFound):
		return parsedCSV{}, &ValidationError{Msg: "no file was found — upload it first"}
	case errors.Is(err, storage.ErrObjectTooLarge):
		return parsedCSV{}, &ValidationError{Msg: "the file is too large"}
	case err != nil:
		return parsedCSV{}, fmt.Errorf("read import: %w", err)
	}

	return parseCSV(data)
}

// importPlan is every row of a file, validated: the ones that will become
// items, and the ones that will not, with why.
type importPlan struct {
	total      int
	items      []newItem
	rowNumbers []int // parallel to items
	errors     []api.ImportRowError
	errorCount int
}

func (p *importPlan) fail(row int, msg string) {
	p.errorCount++

	if len(p.errors) < maxReportErrors {
		p.errors = append(p.errors, api.ImportRowError{Row: row, Message: msg})
	}
}

func (s *Service) planImport(ctx context.Context, principal auth.Principal, body api.ImportRequest) (*importPlan, error) {
	file, err := s.loadImport(ctx, principal, body.StorageKey)
	if err != nil {
		return nil, err
	}

	columns, err := columnIndexes(file.headers, body.Mapping)
	if err != nil {
		return nil, err
	}

	if len(file.records) > maxImportRows {
		return nil, &ValidationError{Msg: fmt.Sprintf("a file holds at most %d rows", maxImportRows)}
	}

	res, err := s.newImportResolver(ctx, pgconv.UUID(principal.FamilyID), body.Defaults)
	if err != nil {
		return nil, err
	}

	plan := &importPlan{}

	for i, rec := range file.records {
		if blankRecord(rec) {
			continue
		}

		row := i + 1
		plan.total++

		req, err := res.request(ctx, func(field string) string {
			idx, ok := columns[field]
			if !ok || idx >= len(rec) {
				return ""
			}

			return csvUnsafe(strings.TrimSpace(rec[idx]))
		})
		if err != nil {
			plan.fail(row, rowMessage(err))

			continue
		}

		params, tagIDs, err := s.prepareCreate(ctx, principal, &req)
		if err != nil {
			plan.fail(row, rowMessage(err))

			continue
		}

		plan.items = append(plan.items, newItem{params: params, tagIDs: tagIDs, extra: map[string]any{"imported": "csv"}})
		plan.rowNumbers = append(plan.rowNumbers, row)
	}

	return plan, nil
}

// rowMessage turns a row failure into text for the report. Validation and
// not-found failures are the user's to fix; anything else is reported
// generically rather than leaking internals into a report.
func rowMessage(err error) string {
	if msg := asValidation(err); msg != "" {
		return msg
	}

	if what := asNotFound(err); what != "" {
		return what + " not found"
	}

	var rowErr *importRowError
	if errors.As(err, &rowErr) {
		return rowErr.msg
	}

	return "could not be checked"
}

// importRowError is a problem found while reading a row's cells, before any
// database validation.
type importRowError struct{ msg string }

func (e *importRowError) Error() string { return e.msg }

func cellErr(format string, args ...any) error {
	return &importRowError{msg: fmt.Sprintf(format, args...)}
}

// --- resolving names to ids ----------------------------------------------

type importResolver struct {
	s        *Service
	familyID pgtype.UUID
	defaults api.ImportDefaults

	categories map[string]uuid.UUID
	locations  map[string]uuid.UUID
	rooms      map[uuid.UUID]map[string]uuid.UUID
	people     map[string]uuid.UUID
	tags       map[string]uuid.UUID
}

func (s *Service) newImportResolver(ctx context.Context, familyID pgtype.UUID, defaults *api.ImportDefaults) (*importResolver, error) {
	r := &importResolver{
		s: s, familyID: familyID,
		categories: map[string]uuid.UUID{}, locations: map[string]uuid.UUID{},
		rooms: map[uuid.UUID]map[string]uuid.UUID{}, people: map[string]uuid.UUID{}, tags: map[string]uuid.UUID{},
	}
	if defaults != nil {
		r.defaults = *defaults
	}

	categories, err := s.store.ListCategories(ctx, db.ListCategoriesParams{FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	// Built-ins come first in the result; a family's own category with the
	// same slug is listed later and wins.
	for i := range categories {
		r.categories[strings.ToLower(categories[i].Slug)] = pgconv.ToUUID(categories[i].ID)
	}

	locations, err := s.store.ListLocations(ctx, db.ListLocationsParams{FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list locations: %w", err)
	}

	for i := range locations {
		r.locations[strings.ToLower(locations[i].Name)] = pgconv.ToUUID(locations[i].ID)
	}

	people, err := s.store.ListPeople(ctx, db.ListPeopleParams{FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}

	for i := range people {
		r.people[strings.ToLower(people[i].Name)] = pgconv.ToUUID(people[i].ID)
	}

	tags, err := s.store.ListTags(ctx, familyID)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	for i := range tags {
		r.tags[strings.ToLower(tags[i].Name)] = pgconv.ToUUID(tags[i].ID)
	}

	return r, nil
}

func (r *importResolver) roomsOf(ctx context.Context, locationID uuid.UUID) (map[string]uuid.UUID, error) {
	if m, ok := r.rooms[locationID]; ok {
		return m, nil
	}

	rooms, err := r.s.store.ListRoomsByLocation(ctx, db.ListRoomsByLocationParams{
		LocationID: pgconv.UUID(locationID), FamilyID: r.familyID,
	})
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}

	m := make(map[string]uuid.UUID, len(rooms))
	for i := range rooms {
		m[strings.ToLower(rooms[i].Name)] = pgconv.ToUUID(rooms[i].ID)
	}

	r.rooms[locationID] = m

	return m, nil
}

// request turns one row's cells into a create request. Names are resolved
// to ids here; everything else (traits, containment, tags belonging to the
// family) is validated afterwards by prepareCreate, exactly as for a single
// create.
func (r *importResolver) request(ctx context.Context, cell func(field string) string) (api.CreateItemRequest, error) {
	req := api.CreateItemRequest{Name: cell("name"), Quantity: 1}

	if req.Name == "" {
		return req, cellErr("the name is empty")
	}

	if err := r.resolveCategory(&req, cell("category")); err != nil {
		return req, err
	}

	if err := r.resolvePlace(ctx, &req, cell("location"), cell("room"), cell("container")); err != nil {
		return req, err
	}

	if err := r.resolveRefs(&req, cell("for_person"), cell("tags")); err != nil {
		return req, err
	}

	if err := fillScalars(&req, cell); err != nil {
		return req, err
	}

	return req, nil
}

func (r *importResolver) resolveCategory(req *api.CreateItemRequest, name string) error {
	switch {
	case name != "":
		id, ok := r.categories[strings.ToLower(name)]
		if !ok {
			return cellErr("unknown category %q — use a category slug such as the ones in an export", name)
		}

		req.CategoryId = id
	case r.defaults.CategoryId != nil:
		req.CategoryId = *r.defaults.CategoryId
	default:
		return cellErr("no category — map a column or choose a default")
	}

	return nil
}

func (r *importResolver) resolvePlace(ctx context.Context, req *api.CreateItemRequest, location, room, container string) error {
	if container != "" {
		return r.resolveContainer(ctx, req, container)
	}

	var locationID uuid.UUID

	switch {
	case location != "":
		id, ok := r.locations[strings.ToLower(location)]
		if !ok {
			return cellErr("unknown location %q", location)
		}

		locationID = id
	case r.defaults.LocationId != nil:
		locationID = *r.defaults.LocationId
	default:
		return cellErr("no location or container — map a column or choose a default")
	}

	req.LocationId = &locationID

	switch {
	case room != "":
		rooms, err := r.roomsOf(ctx, locationID)
		if err != nil {
			return err
		}

		id, ok := rooms[strings.ToLower(room)]
		if !ok {
			return cellErr("unknown room %q in that location", room)
		}

		req.RoomId = &id
	case location == "" && r.defaults.RoomId != nil:
		req.RoomId = r.defaults.RoomId
	}

	return nil
}

func (r *importResolver) resolveContainer(ctx context.Context, req *api.CreateItemRequest, container string) error {
	boxes, err := r.s.store.ListContainersByName(ctx, db.ListContainersByNameParams{FamilyID: r.familyID, Name: container})
	if err != nil {
		return fmt.Errorf("find container: %w", err)
	}

	switch len(boxes) {
	case 0:
		return cellErr("no container called %q (containers made by the same import are not visible yet)", container)
	case 1:
		id := pgconv.ToUUID(boxes[0].ID)
		req.ParentItemId = &id

		return nil
	default:
		return cellErr("more than one container is called %q", container)
	}
}

func (r *importResolver) resolveRefs(req *api.CreateItemRequest, person, tags string) error {
	if person != "" {
		id, ok := r.people[strings.ToLower(person)]
		if !ok {
			return cellErr("unknown person %q", person)
		}

		req.ForPersonId = &id
	}

	if tags != "" {
		ids := []uuid.UUID{}

		for _, name := range strings.Split(tags, ";") {
			if name = strings.TrimSpace(name); name == "" {
				continue
			}

			id, ok := r.tags[strings.ToLower(name)]
			if !ok {
				return cellErr("unknown tag %q — create it first", name)
			}

			ids = append(ids, id)
		}

		req.TagIds = &ids
	}

	return nil
}

// fillScalars sets the plain-valued fields from their cells.
func fillScalars(req *api.CreateItemRequest, cell func(string) string) error {
	var err error

	if v := cell("quantity"); v != "" {
		q, perr := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 32)
		if perr != nil || q < 0 {
			return cellErr("quantity %q is not a number", v)
		}

		req.Quantity = float32(q)
	}

	req.Spot = optional(cell("spot"))
	req.Unit = optional(cell("unit"))
	req.Brand = optional(cell("brand"))
	req.SizeValue = optional(cell("size"))
	req.SerialNumber = optional(cell("serial_number"))
	req.Notes = optional(cell("notes"))
	req.PurchaseCurrency = optional(strings.ToUpper(cell("purchase_currency")))

	if v := cell("is_container"); v != "" {
		req.IsContainer = truthy(v)
	}

	if req.MinQuantity, err = floatCell("min_quantity", cell("min_quantity")); err != nil {
		return err
	}

	if req.PurchasePrice, err = floatCell("purchase_price", cell("purchase_price")); err != nil {
		return err
	}

	if req.PaoMonths, err = intCell("pao_months", cell("pao_months")); err != nil {
		return err
	}

	for field, dst := range map[string]**openapi_types.Date{
		"expires_at": &req.ExpiresAt, "opened_at": &req.OpenedAt,
		"warranty_until": &req.WarrantyUntil, "purchase_date": &req.PurchaseDate,
	} {
		if *dst, err = dateCell(field, cell(field)); err != nil {
			return err
		}
	}

	return fillEnums(req, cell)
}

func fillEnums(req *api.CreateItemRequest, cell func(string) string) error {
	if v := strings.ToLower(cell("status")); v != "" {
		st := api.ItemStatus(v)
		if !st.Valid() {
			return cellErr("unknown status %q", v)
		}

		req.Status = &st
	}

	if v := strings.ToLower(cell("condition")); v != "" {
		c := api.ItemCondition(v)
		if !c.Valid() {
			return cellErr("unknown condition %q", v)
		}

		req.Condition = &c
	}

	if v := strings.ToLower(cell("season")); v != "" {
		se := api.ItemSeason(v)
		if !se.Valid() {
			return cellErr("unknown season %q", v)
		}

		req.Season = &se
	}

	if v := strings.ToLower(cell("size_system")); v != "" {
		sy := api.SizeSystem(v)
		if !sy.Valid() {
			return cellErr("unknown size system %q", v)
		}

		req.SizeSystem = &sy
	}

	if v := cell("traits"); v != "" {
		var traits map[string]any
		if err := json.Unmarshal([]byte(v), &traits); err != nil {
			return cellErr("traits must be a JSON object")
		}

		req.Traits = &traits
	}

	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func truthy(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "y", "так", "т":
		return true
	default:
		return false
	}
}

func floatCell(field, v string) (*float32, error) {
	if v == "" {
		return nil, nil
	}

	f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 32)
	if err != nil {
		return nil, cellErr("%s %q is not a number", field, v)
	}

	out := float32(f)

	return &out, nil
}

func intCell(field, v string) (*int32, error) {
	if v == "" {
		return nil, nil
	}

	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return nil, cellErr("%s %q is not a whole number", field, v)
	}

	out := int32(n)

	return &out, nil
}

func dateCell(field, v string) (*openapi_types.Date, error) {
	if v == "" {
		return nil, nil
	}

	for _, layout := range []string{"2006-01-02", "02.01.2006", "02/01/2006"} {
		if t, err := time.Parse(layout, v); err == nil {
			return &openapi_types.Date{Time: t}, nil
		}
	}

	return nil, cellErr("%s %q is not a date — use YYYY-MM-DD", field, v)
}

// --- handlers ------------------------------------------------------------

func importBadRequest(err error) (api.BadRequestJSONResponse, bool) {
	if msg := asValidation(err); msg != "" {
		return invalidImport(msg), true
	}

	return api.BadRequestJSONResponse{}, false
}

// PresignImport mints the upload URL for a CSV.
//
//nolint:dupl // only the generated request/response types differ between the import handlers
func (h *Handler) PresignImport(ctx context.Context, request api.PresignImportRequestObject) (api.PresignImportResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.PresignImport401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.PresignImport400JSONResponse{BadRequestJSONResponse: invalidImport("the request body is empty")}, nil
	}

	result, err := h.service.PresignImport(ctx, principal, *request.Body)
	if bad, isBad := importBadRequest(err); isBad {
		return api.PresignImport400JSONResponse{BadRequestJSONResponse: bad}, nil
	} else if err != nil {
		return nil, fmt.Errorf("presign import: %w", err)
	}

	return api.PresignImport200JSONResponse(result), nil
}

// InspectImport reads an uploaded file's header.
//
//nolint:dupl // only the generated request/response types differ between the import handlers
func (h *Handler) InspectImport(ctx context.Context, request api.InspectImportRequestObject) (api.InspectImportResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.InspectImport401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.InspectImport400JSONResponse{BadRequestJSONResponse: invalidImport("the request body is empty")}, nil
	}

	result, err := h.service.InspectImport(ctx, principal, request.Body.StorageKey)
	if bad, isBad := importBadRequest(err); isBad {
		return api.InspectImport400JSONResponse{BadRequestJSONResponse: bad}, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect import: %w", err)
	}

	return api.InspectImport200JSONResponse(result), nil
}

// DryRunImport validates an import without writing.
//
//nolint:dupl // only the generated request/response types differ between the import handlers
func (h *Handler) DryRunImport(ctx context.Context, request api.DryRunImportRequestObject) (api.DryRunImportResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.DryRunImport401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.DryRunImport400JSONResponse{BadRequestJSONResponse: invalidImport("the request body is empty")}, nil
	}

	result, err := h.service.DryRunImport(ctx, principal, *request.Body)
	if bad, isBad := importBadRequest(err); isBad {
		return api.DryRunImport400JSONResponse{BadRequestJSONResponse: bad}, nil
	} else if err != nil {
		return nil, fmt.Errorf("dry run import: %w", err)
	}

	return api.DryRunImport200JSONResponse(result), nil
}

// CommitImport creates the items from an uploaded file.
//
//nolint:dupl // only the generated request/response types differ between the import handlers
func (h *Handler) CommitImport(ctx context.Context, request api.CommitImportRequestObject) (api.CommitImportResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CommitImport401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CommitImport400JSONResponse{BadRequestJSONResponse: invalidImport("the request body is empty")}, nil
	}

	result, err := h.service.CommitImport(ctx, principal, *request.Body)
	if bad, isBad := importBadRequest(err); isBad {
		return api.CommitImport400JSONResponse{BadRequestJSONResponse: bad}, nil
	} else if err != nil {
		return nil, fmt.Errorf("commit import: %w", err)
	}

	return api.CommitImport201JSONResponse(result), nil
}

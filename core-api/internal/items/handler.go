package items

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
)

// Handler dispatches the generated strict interface's items and tags
// operations to Service, translating NotFoundError/ValidationError into
// the right response type for each operation. The business logic itself —
// validation, containment, transactions — lives entirely in Service; this
// file only ever does request/response shaping.
type Handler struct {
	service *Service
	log     *slog.Logger
}

// NewHandler builds the items handler.
func NewHandler(service *Service, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log.With(slog.String("component", "items"))}
}

// CreateItem adds an item.
//
//nolint:dupl // only the generated request/response types differ from PresignUpload/ConfirmItemPhoto/ConfirmItemAttachment
func (h *Handler) CreateItem(ctx context.Context, request api.CreateItemRequestObject) (api.CreateItemResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateItem401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateItem400JSONResponse{BadRequestJSONResponse: invalidItem("an item needs at least a name and a category")}, nil
	}

	item, err := h.service.CreateItem(ctx, principal, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.CreateItem404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.CreateItem400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("create item: %w", err)
	}

	return api.CreateItem201JSONResponse(item), nil
}

// CreateItemsBatch adds several items in one transaction.
func (h *Handler) CreateItemsBatch(ctx context.Context, request api.CreateItemsBatchRequestObject) (api.CreateItemsBatchResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateItemsBatch401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateItemsBatch400JSONResponse{BadRequestJSONResponse: invalidItem("a batch needs at least one item")}, nil
	}

	items, err := h.service.CreateItems(ctx, principal, request.Body.Items)

	switch {
	case asNotFound(err) != "":
		return api.CreateItemsBatch404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.CreateItemsBatch400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("create items batch: %w", err)
	}

	return api.CreateItemsBatch201JSONResponse(items), nil
}

// DuplicateItem copies an item into the same place.
func (h *Handler) DuplicateItem(ctx context.Context, request api.DuplicateItemRequestObject) (api.DuplicateItemResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.DuplicateItem401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	count := 1
	if request.Body != nil && request.Body.Count != nil {
		count = *request.Body.Count
	}

	items, err := h.service.DuplicateItem(ctx, principal, request.ItemId, count)

	switch {
	case asNotFound(err) != "":
		return api.DuplicateItem404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.DuplicateItem400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("duplicate item: %w", err)
	}

	return api.DuplicateItem201JSONResponse(items), nil
}

// ExportItems streams the inventory as CSV.
func (h *Handler) ExportItems(ctx context.Context, _ api.ExportItemsRequestObject) (api.ExportItemsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ExportItems401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	disposition := `attachment; filename="mykomora-items.csv"`

	return api.ExportItems200TextcsvResponse{
		Body:    h.service.ExportCSV(ctx, principal),
		Headers: api.ExportItems200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}

// GetItem returns one item.
func (h *Handler) GetItem(ctx context.Context, request api.GetItemRequestObject) (api.GetItemResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetItem401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	item, err := h.service.GetItem(ctx, principal, request.ItemId)

	switch {
	case asNotFound(err) != "":
		return api.GetItem404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("get item: %w", err)
	}

	return api.GetItem200JSONResponse(item), nil
}

// UpdateItem changes an item. Place and status are handled by their own
// endpoints.
//
//nolint:dupl // only the generated request/response types differ from MoveItem/SetItemStatus/UpdateTag
func (h *Handler) UpdateItem(ctx context.Context, request api.UpdateItemRequestObject) (api.UpdateItemResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateItem401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.UpdateItem400JSONResponse{BadRequestJSONResponse: invalidItem("the request body is empty")}, nil
	}

	item, err := h.service.UpdateItem(ctx, principal, request.ItemId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.UpdateItem404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.UpdateItem400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("update item: %w", err)
	}

	return api.UpdateItem200JSONResponse(item), nil
}

// ArchiveItem retires an item outright.
func (h *Handler) ArchiveItem(ctx context.Context, request api.ArchiveItemRequestObject) (api.ArchiveItemResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ArchiveItem401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	err := h.service.Archive(ctx, principal, request.ItemId)

	switch {
	case asNotFound(err) != "":
		return api.ArchiveItem404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("archive item: %w", err)
	}

	return api.ArchiveItem204Response{}, nil
}

// MoveItem moves an item to a place or into a container.
//
//nolint:dupl // only the generated request/response types differ from UpdateItem/SetItemStatus/UpdateTag
func (h *Handler) MoveItem(ctx context.Context, request api.MoveItemRequestObject) (api.MoveItemResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.MoveItem401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.MoveItem400JSONResponse{BadRequestJSONResponse: invalidItem("the request body is empty")}, nil
	}

	item, err := h.service.MoveItem(ctx, principal, request.ItemId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.MoveItem404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.MoveItem400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("move item: %w", err)
	}

	return api.MoveItem200JSONResponse(item), nil
}

// SetItemStatus changes an item's lifecycle status.
//
//nolint:dupl // only the generated request/response types differ from UpdateItem/MoveItem/UpdateTag
func (h *Handler) SetItemStatus(ctx context.Context, request api.SetItemStatusRequestObject) (api.SetItemStatusResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.SetItemStatus401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.SetItemStatus400JSONResponse{BadRequestJSONResponse: invalidItem("the request body is empty")}, nil
	}

	item, err := h.service.SetStatus(ctx, principal, request.ItemId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.SetItemStatus404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.SetItemStatus400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("set item status: %w", err)
	}

	return api.SetItemStatus200JSONResponse(item), nil
}

// ListItemContents returns an item's direct children.
func (h *Handler) ListItemContents(ctx context.Context, request api.ListItemContentsRequestObject) (api.ListItemContentsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListItemContents401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.ListContents(ctx, principal, request.ItemId)

	switch {
	case asNotFound(err) != "":
		return api.ListItemContents404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("list item contents: %w", err)
	}

	return api.ListItemContents200JSONResponse(out), nil
}

// ListItemEvents returns an item's history.
func (h *Handler) ListItemEvents(ctx context.Context, request api.ListItemEventsRequestObject) (api.ListItemEventsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListItemEvents401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.ListEvents(ctx, principal, request.ItemId, request.Params)

	switch {
	case asNotFound(err) != "":
		return api.ListItemEvents404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("list item events: %w", err)
	}

	return api.ListItemEvents200JSONResponse(out), nil
}

// ListItemTags returns an item's attached tags.
func (h *Handler) ListItemTags(ctx context.Context, request api.ListItemTagsRequestObject) (api.ListItemTagsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListItemTags401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.ListItemTags(ctx, principal, request.ItemId)

	switch {
	case asNotFound(err) != "":
		return api.ListItemTags404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("list item tags: %w", err)
	}

	return api.ListItemTags200JSONResponse(out), nil
}

// AttachItemTag attaches a tag to an item.
func (h *Handler) AttachItemTag(ctx context.Context, request api.AttachItemTagRequestObject) (api.AttachItemTagResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.AttachItemTag401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.AttachItemTag400JSONResponse{BadRequestJSONResponse: invalidItem("a tagId is required")}, nil
	}

	err := h.service.AttachTag(ctx, principal, request.ItemId, request.Body.TagId)

	switch {
	case asNotFound(err) != "":
		return api.AttachItemTag404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("attach item tag: %w", err)
	}

	return api.AttachItemTag204Response{}, nil
}

// DetachItemTag detaches a tag from an item.
func (h *Handler) DetachItemTag(ctx context.Context, request api.DetachItemTagRequestObject) (api.DetachItemTagResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.DetachItemTag401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	err := h.service.DetachTag(ctx, principal, request.ItemId, request.TagId)

	switch {
	case asNotFound(err) != "":
		return api.DetachItemTag404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("detach item tag: %w", err)
	}

	return api.DetachItemTag204Response{}, nil
}

// ListTags returns the family's tags.
func (h *Handler) ListTags(ctx context.Context, _ api.ListTagsRequestObject) (api.ListTagsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListTags401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.ListTags(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	return api.ListTags200JSONResponse(out), nil
}

// CreateTag adds a tag.
func (h *Handler) CreateTag(ctx context.Context, request api.CreateTagRequestObject) (api.CreateTagResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.CreateTag401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.CreateTag400JSONResponse{BadRequestJSONResponse: invalidTag("a tag needs a name")}, nil
	}

	tag, err := h.service.CreateTag(ctx, principal, *request.Body)

	switch {
	case asValidation(err) != "":
		return api.CreateTag400JSONResponse{BadRequestJSONResponse: invalidTag(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("create tag: %w", err)
	}

	return api.CreateTag201JSONResponse(tag), nil
}

// UpdateTag renames or recolors a tag.
//
//nolint:dupl // only the generated request/response types differ from UpdateItem/MoveItem/SetItemStatus
func (h *Handler) UpdateTag(ctx context.Context, request api.UpdateTagRequestObject) (api.UpdateTagResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateTag401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.UpdateTag400JSONResponse{BadRequestJSONResponse: invalidTag("the request body is empty")}, nil
	}

	tag, err := h.service.UpdateTag(ctx, principal, request.TagId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.UpdateTag404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.UpdateTag400JSONResponse{BadRequestJSONResponse: invalidTag(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("update tag: %w", err)
	}

	return api.UpdateTag200JSONResponse(tag), nil
}

// DeleteTag removes a tag.
func (h *Handler) DeleteTag(ctx context.Context, request api.DeleteTagRequestObject) (api.DeleteTagResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.DeleteTag401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	err := h.service.DeleteTag(ctx, principal, request.TagId)

	switch {
	case asNotFound(err) != "":
		return api.DeleteTag404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("delete tag: %w", err)
	}

	return api.DeleteTag204Response{}, nil
}

// asNotFound returns the "what" of a NotFoundError, or "" if err is not one.
func asNotFound(err error) string {
	var nf *NotFoundError
	if errors.As(err, &nf) {
		return nf.What
	}

	return ""
}

// asValidation returns the message of a ValidationError, or "" if err is
// not one.
func asValidation(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Msg
	}

	return ""
}

func unauthorized() api.UnauthorizedJSONResponse {
	return api.UnauthorizedJSONResponse{Code: "no_session", Message: "sign in to continue"}
}

func notFound(what string) api.NotFoundJSONResponse {
	return api.NotFoundJSONResponse{Code: "not_found", Message: "no such " + what + " in this family"}
}

func invalidItem(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_item", Message: message}
}

func invalidTag(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_tag", Message: message}
}

// ListItems browses and searches the family's items.
func (h *Handler) ListItems(ctx context.Context, request api.ListItemsRequestObject) (api.ListItemsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListItems401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	page, err := h.service.ListItems(ctx, principal, request.Params)

	switch {
	case asValidation(err) != "":
		return api.ListItems400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("list items: %w", err)
	}

	return api.ListItems200JSONResponse(page), nil
}

// FindSimilarItems returns duplicate hints for a prospective item name.
func (h *Handler) FindSimilarItems(ctx context.Context, request api.FindSimilarItemsRequestObject) (api.FindSimilarItemsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.FindSimilarItems401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.FindSimilar(ctx, principal, request.Params.Name)

	switch {
	case asValidation(err) != "":
		return api.FindSimilarItems400JSONResponse{BadRequestJSONResponse: invalidItem(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("find similar items: %w", err)
	}

	return api.FindSimilarItems200JSONResponse(out), nil
}

// GetDashboard returns the family's counts, value and recent activity.
func (h *Handler) GetDashboard(ctx context.Context, _ api.GetDashboardRequestObject) (api.GetDashboardResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.GetDashboard401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.Dashboard(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("get dashboard: %w", err)
	}

	return api.GetDashboard200JSONResponse(out), nil
}

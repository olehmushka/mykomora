package items

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

const maxFilenameLen = 255

// --- handler -----------------------------------------------------------

// ListItemAttachments returns an item's receipts, manuals and other
// documents.
func (h *Handler) ListItemAttachments(ctx context.Context, request api.ListItemAttachmentsRequestObject) (api.ListItemAttachmentsResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListItemAttachments401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.ListAttachments(ctx, principal, request.ItemId)

	switch {
	case asNotFound(err) != "":
		return api.ListItemAttachments404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("list item attachments: %w", err)
	}

	return api.ListItemAttachments200JSONResponse(out), nil
}

// ConfirmItemAttachment confirms an attachment after it has been uploaded
// to object storage.
//
//nolint:dupl // only the generated request/response types differ from CreateItem/PresignUpload/ConfirmItemPhoto
func (h *Handler) ConfirmItemAttachment(ctx context.Context, request api.ConfirmItemAttachmentRequestObject) (api.ConfirmItemAttachmentResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ConfirmItemAttachment401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.ConfirmItemAttachment400JSONResponse{BadRequestJSONResponse: invalidUpload("the request body is empty")}, nil
	}

	attachment, err := h.service.ConfirmAttachment(ctx, principal, request.ItemId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.ConfirmItemAttachment404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.ConfirmItemAttachment400JSONResponse{BadRequestJSONResponse: invalidUpload(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("confirm item attachment: %w", err)
	}

	return api.ConfirmItemAttachment201JSONResponse(attachment), nil
}

// DeleteItemAttachment deletes an attachment.
func (h *Handler) DeleteItemAttachment(ctx context.Context, request api.DeleteItemAttachmentRequestObject) (api.DeleteItemAttachmentResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.DeleteItemAttachment401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	err := h.service.DeleteAttachment(ctx, principal, request.ItemId, request.AttachmentId)

	switch {
	case asNotFound(err) != "":
		return api.DeleteItemAttachment404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("delete item attachment: %w", err)
	}

	return api.DeleteItemAttachment204Response{}, nil
}

// --- service -------------------------------------------------------------

// ListAttachments returns an item's attachments, oldest first.
//
//nolint:dupl // only the stored/returned type differs from ListPhotos
func (s *Service) ListAttachments(ctx context.Context, principal auth.Principal, itemID uuid.UUID) ([]api.ItemAttachment, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	rows, err := s.store.ListItemAttachments(ctx, db.ListItemAttachmentsParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list item attachments: %w", err)
	}

	out := make([]api.ItemAttachment, 0, len(rows))

	for i := range rows {
		attachment, err := s.toAPIAttachment(ctx, rows[i])
		if err != nil {
			return nil, err
		}

		out = append(out, attachment)
	}

	return out, nil
}

// ConfirmAttachment confirms an attachment once it has been PUT to object
// storage.
func (s *Service) ConfirmAttachment(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.ConfirmItemAttachmentRequest,
) (api.ItemAttachment, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.ItemAttachment{}, wrapped
	}

	filename, problem := validAttachmentFilename(body.Filename)
	if problem != "" {
		return api.ItemAttachment{}, &ValidationError{Msg: problem}
	}

	if body.Bytes <= 0 {
		return api.ItemAttachment{}, &ValidationError{Msg: "bytes must be positive"}
	}

	if err := s.verifyUpload(ctx, body.StorageKey, body.ContentType, body.Bytes, attachmentContentTypes, maxAttachmentBytes); err != nil {
		return api.ItemAttachment{}, err
	}

	row, err := s.confirmAttachmentTx(ctx, principal, familyID, itemID, filename, body)
	if err != nil {
		return api.ItemAttachment{}, err
	}

	return s.toAPIAttachment(ctx, row)
}

func (s *Service) confirmAttachmentTx(
	ctx context.Context, principal auth.Principal, familyID pgtype.UUID, itemID uuid.UUID, filename string, body api.ConfirmItemAttachmentRequest,
) (db.ItemAttachment, error) {
	var created db.ItemAttachment

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.CreateItemAttachment(ctx, db.CreateItemAttachmentParams{
			ItemID: pgconv.UUID(itemID), Kind: db.AttachmentKind(body.Kind),
			StorageKey: body.StorageKey, Filename: filename, ContentType: body.ContentType, Bytes: body.Bytes,
			UploadedBy: pgconv.UUID(principal.UserID), FamilyID: familyID,
		})
		if err != nil {
			return fmt.Errorf("create item attachment: %w", err)
		}

		created = row

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: pgconv.UUID(itemID), ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventAttachmentAdded, Payload: pgconv.JSONB(attachmentAddedPayload(&row)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})

	return created, err
}

// DeleteAttachment deletes an attachment.
func (s *Service) DeleteAttachment(ctx context.Context, principal auth.Principal, itemID, attachmentID uuid.UUID) error {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return wrapped
	}

	var deleted db.ItemAttachment

	err = s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.DeleteItemAttachment(ctx, db.DeleteItemAttachmentParams{
			ID: pgconv.UUID(attachmentID), ItemID: pgconv.UUID(itemID), FamilyID: familyID,
		})

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return &NotFoundError{What: "attachment"}
		case err != nil:
			return fmt.Errorf("delete item attachment: %w", err)
		}

		deleted = row

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: pgconv.UUID(itemID), ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventAttachmentRemoved, Payload: pgconv.JSONB(attachmentRemovedPayload(&row)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Best-effort, outside the transaction — see photos.go's
	// deletePhotoObjects for why.
	if err := s.storage.DeleteObject(ctx, deleted.StorageKey); err != nil {
		s.log.Warn("delete attachment object", slog.String("key", deleted.StorageKey), slog.Any("error", err))
	}

	return nil
}

func (s *Service) toAPIAttachment(ctx context.Context, row db.ItemAttachment) (api.ItemAttachment, error) {
	url, err := s.storage.PresignGet(ctx, row.StorageKey)
	if err != nil {
		return api.ItemAttachment{}, fmt.Errorf("presign get: %w", err)
	}

	return apimap.ItemAttachment(&row, url), nil
}

func validAttachmentFilename(raw string) (name, problem string) {
	name = strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxFilenameLen {
		return "", "a filename must be between 1 and 255 characters"
	}

	return name, ""
}

package items

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// --- handler -----------------------------------------------------------

// ListItemPhotos returns an item's photo gallery.
func (h *Handler) ListItemPhotos(ctx context.Context, request api.ListItemPhotosRequestObject) (api.ListItemPhotosResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ListItemPhotos401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	out, err := h.service.ListPhotos(ctx, principal, request.ItemId)

	switch {
	case asNotFound(err) != "":
		return api.ListItemPhotos404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("list item photos: %w", err)
	}

	return api.ListItemPhotos200JSONResponse(out), nil
}

// ConfirmItemPhoto confirms a photo after it has been uploaded to object
// storage.
//
//nolint:dupl // only the generated request/response types differ from CreateItem/PresignUpload/ConfirmItemAttachment
func (h *Handler) ConfirmItemPhoto(ctx context.Context, request api.ConfirmItemPhotoRequestObject) (api.ConfirmItemPhotoResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.ConfirmItemPhoto401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.ConfirmItemPhoto400JSONResponse{BadRequestJSONResponse: invalidUpload("the request body is empty")}, nil
	}

	photo, err := h.service.ConfirmPhoto(ctx, principal, request.ItemId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.ConfirmItemPhoto404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.ConfirmItemPhoto400JSONResponse{BadRequestJSONResponse: invalidUpload(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("confirm item photo: %w", err)
	}

	return api.ConfirmItemPhoto201JSONResponse(photo), nil
}

// UpdateItemPhoto reorders a photo or makes it the primary one.
//
//nolint:dupl // only the generated request/response types differ from DeleteItemAttachment's sibling shapes
func (h *Handler) UpdateItemPhoto(ctx context.Context, request api.UpdateItemPhotoRequestObject) (api.UpdateItemPhotoResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.UpdateItemPhoto401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.UpdateItemPhoto400JSONResponse{BadRequestJSONResponse: invalidUpload("the request body is empty")}, nil
	}

	photo, err := h.service.UpdatePhoto(ctx, principal, request.ItemId, request.PhotoId, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.UpdateItemPhoto404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.UpdateItemPhoto400JSONResponse{BadRequestJSONResponse: invalidUpload(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("update item photo: %w", err)
	}

	return api.UpdateItemPhoto200JSONResponse(photo), nil
}

// DeleteItemPhoto deletes a photo, promoting the next one to primary if the
// deleted photo was primary and others remain.
func (h *Handler) DeleteItemPhoto(ctx context.Context, request api.DeleteItemPhotoRequestObject) (api.DeleteItemPhotoResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.DeleteItemPhoto401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	err := h.service.DeletePhoto(ctx, principal, request.ItemId, request.PhotoId)

	switch {
	case asNotFound(err) != "":
		return api.DeleteItemPhoto404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("delete item photo: %w", err)
	}

	return api.DeleteItemPhoto204Response{}, nil
}

// --- service -------------------------------------------------------------

// ListPhotos returns an item's photo gallery, in display order.
//
//nolint:dupl // only the stored/returned type differs from ListAttachments
func (s *Service) ListPhotos(ctx context.Context, principal auth.Principal, itemID uuid.UUID) ([]api.ItemPhoto, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return nil, wrapped
	}

	rows, err := s.store.ListItemPhotos(ctx, db.ListItemPhotosParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list item photos: %w", err)
	}

	out := make([]api.ItemPhoto, 0, len(rows))

	for i := range rows {
		photo, err := s.toAPIPhoto(ctx, rows[i])
		if err != nil {
			return nil, err
		}

		out = append(out, photo)
	}

	return out, nil
}

// ConfirmPhoto confirms a photo once its full image and thumbnail have
// both been PUT to object storage. The first photo on an item is always
// primary, regardless of the request.
func (s *Service) ConfirmPhoto(
	ctx context.Context, principal auth.Principal, itemID uuid.UUID, body api.ConfirmItemPhotoRequest,
) (api.ItemPhoto, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.ItemPhoto{}, wrapped
	}

	if err := validateConfirmPhoto(body); err != nil {
		return api.ItemPhoto{}, err
	}

	if err := s.verifyUpload(ctx, body.StorageKey, body.ContentType, body.Bytes, photoContentTypes, maxPhotoBytes); err != nil {
		return api.ItemPhoto{}, err
	}

	if err := s.verifyUpload(ctx, body.ThumbStorageKey, "", 0, photoContentTypes, maxPhotoThumbBytes); err != nil {
		return api.ItemPhoto{}, err
	}

	row, err := s.confirmPhotoTx(ctx, principal, familyID, itemID, body)
	if err != nil {
		return api.ItemPhoto{}, err
	}

	return s.toAPIPhoto(ctx, row)
}

func validateConfirmPhoto(body api.ConfirmItemPhotoRequest) error {
	switch {
	case body.StorageKey == "" || body.ThumbStorageKey == "":
		return &ValidationError{Msg: "storageKey and thumbStorageKey are required"}
	case body.Width <= 0 || body.Height <= 0:
		return &ValidationError{Msg: "width and height must be positive"}
	case body.Bytes <= 0:
		return &ValidationError{Msg: "bytes must be positive"}
	}

	return nil
}

func (s *Service) confirmPhotoTx(
	ctx context.Context, principal auth.Principal, familyID pgtype.UUID, itemID uuid.UUID, body api.ConfirmItemPhotoRequest,
) (db.ItemPhoto, error) {
	var created db.ItemPhoto

	err := s.tx.InTx(ctx, func(q *db.Queries) error {
		count, err := q.CountItemPhotos(ctx, db.CountItemPhotosParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})
		if err != nil {
			return fmt.Errorf("count item photos: %w", err)
		}

		row, err := q.CreateItemPhoto(ctx, db.CreateItemPhotoParams{
			ItemID: pgconv.UUID(itemID), FamilyID: familyID,
			StorageKey: body.StorageKey, ThumbKey: body.ThumbStorageKey,
			Width: body.Width, Height: body.Height, Bytes: body.Bytes, ContentType: body.ContentType,
			IsPrimary:  count == 0,   // only the first photo may claim primary at insert time
			SortOrder:  int32(count), //nolint:gosec // photo counts never approach int32 range
			UploadedBy: pgconv.UUID(principal.UserID),
		})
		if err != nil {
			return fmt.Errorf("create item photo: %w", err)
		}

		wantPrimary := count > 0 && body.IsPrimary != nil && *body.IsPrimary
		if wantPrimary {
			if err := q.ClearItemPhotoPrimary(ctx, db.ClearItemPhotoPrimaryParams{ItemID: pgconv.UUID(itemID), ID: row.ID, FamilyID: familyID}); err != nil {
				return fmt.Errorf("clear item photo primary: %w", err)
			}

			row, err = q.UpdateItemPhoto(ctx, db.UpdateItemPhotoParams{
				IsPrimary: pgtype.Bool{Bool: true, Valid: true},
				ID:        row.ID, ItemID: pgconv.UUID(itemID), FamilyID: familyID,
			})
			if err != nil {
				return fmt.Errorf("promote confirmed item photo: %w", err)
			}
		}

		created = row

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: pgconv.UUID(itemID), ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventPhotoAdded, Payload: pgconv.JSONB(photoAddedPayload(&row)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})

	return created, err
}

// UpdatePhoto reorders a photo or makes it the primary one, unsetting
// whichever photo was primary before.
func (s *Service) UpdatePhoto(
	ctx context.Context, principal auth.Principal, itemID, photoID uuid.UUID, body api.UpdateItemPhotoRequest,
) (api.ItemPhoto, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.ItemPhoto{}, wrapped
	}

	var updated db.ItemPhoto

	err = s.tx.InTx(ctx, func(q *db.Queries) error {
		if body.IsPrimary != nil && *body.IsPrimary {
			if err := q.ClearItemPhotoPrimary(ctx, db.ClearItemPhotoPrimaryParams{
				ItemID: pgconv.UUID(itemID), ID: pgconv.UUID(photoID), FamilyID: familyID,
			}); err != nil {
				return fmt.Errorf("clear item photo primary: %w", err)
			}
		}

		row, err := q.UpdateItemPhoto(ctx, db.UpdateItemPhotoParams{
			IsPrimary: pgconv.Bool(body.IsPrimary),
			SortOrder: pgconv.Int4Ptr(body.SortOrder),
			ID:        pgconv.UUID(photoID), ItemID: pgconv.UUID(itemID), FamilyID: familyID,
		})

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return &NotFoundError{What: "photo"}
		case err != nil:
			return fmt.Errorf("update item photo: %w", err)
		}

		updated = row

		return nil
	})
	if err != nil {
		return api.ItemPhoto{}, err
	}

	return s.toAPIPhoto(ctx, updated)
}

// DeletePhoto deletes a photo. If it was primary and others remain, the
// lowest-sortOrder survivor becomes the new primary.
func (s *Service) DeletePhoto(ctx context.Context, principal auth.Principal, itemID, photoID uuid.UUID) error {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(itemID), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return wrapped
	}

	var deleted db.ItemPhoto

	err = s.tx.InTx(ctx, func(q *db.Queries) error {
		row, err := q.DeleteItemPhoto(ctx, db.DeleteItemPhotoParams{ID: pgconv.UUID(photoID), ItemID: pgconv.UUID(itemID), FamilyID: familyID})

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return &NotFoundError{What: "photo"}
		case err != nil:
			return fmt.Errorf("delete item photo: %w", err)
		}

		deleted = row

		var promoted *db.ItemPhoto

		if row.IsPrimary {
			p, err := q.PromoteNextItemPhoto(ctx, db.PromoteNextItemPhotoParams{ItemID: pgconv.UUID(itemID), FamilyID: familyID})

			switch {
			case errors.Is(err, pgx.ErrNoRows):
				// No photos left to promote — the gallery is now empty.
			case err != nil:
				return fmt.Errorf("promote next item photo: %w", err)
			default:
				promoted = &p
			}
		}

		if _, err := q.CreateItemEvent(ctx, db.CreateItemEventParams{
			FamilyID: familyID, ItemID: pgconv.UUID(itemID), ActorUserID: pgconv.UUID(principal.UserID),
			Type: eventPhotoRemoved, Payload: pgconv.JSONB(photoRemovedPayload(&row, promoted)),
		}); err != nil {
			return fmt.Errorf("create item event: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Best-effort, outside the transaction: the database row is the thing
	// that must be gone; a failed object delete leaves orphaned bytes the
	// cleanup sweeper (internal/storage/cleanup.go) reclaims later.
	s.deletePhotoObjects(ctx, &deleted)

	return nil
}

func (s *Service) deletePhotoObjects(ctx context.Context, photo *db.ItemPhoto) {
	if err := s.storage.DeleteObject(ctx, photo.StorageKey); err != nil {
		s.log.Warn("delete photo object", slog.String("key", photo.StorageKey), slog.Any("error", err))
	}

	if err := s.storage.DeleteObject(ctx, photo.ThumbKey); err != nil {
		s.log.Warn("delete photo thumb object", slog.String("key", photo.ThumbKey), slog.Any("error", err))
	}
}

func (s *Service) toAPIPhoto(ctx context.Context, row db.ItemPhoto) (api.ItemPhoto, error) {
	url, err := s.storage.PresignGet(ctx, row.StorageKey)
	if err != nil {
		return api.ItemPhoto{}, fmt.Errorf("presign get: %w", err)
	}

	thumbURL, err := s.storage.PresignGet(ctx, row.ThumbKey)
	if err != nil {
		return api.ItemPhoto{}, fmt.Errorf("presign get: %w", err)
	}

	return apimap.ItemPhoto(&row, url, thumbURL), nil
}

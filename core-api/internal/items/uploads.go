package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
	"github.com/olehmushka/mykomora/core-api/internal/storage"
)

// Per-purpose content-type and size allowlists a presign request is
// checked against before a URL is ever minted (SPEC 6, "Photos and
// attachments"). The confirm step (photos.go/attachments.go) re-checks the
// real object once uploaded, so a client gains nothing by lying here.
const (
	maxPhotoBytes      = 8 << 20  // 8 MiB
	maxPhotoThumbBytes = 1 << 20  // 1 MiB
	maxAttachmentBytes = 15 << 20 // 15 MiB
)

var (
	photoContentTypes      = map[string]struct{}{"image/jpeg": {}, "image/png": {}, "image/webp": {}}
	attachmentContentTypes = map[string]struct{}{"image/jpeg": {}, "image/png": {}, "application/pdf": {}}
)

// PresignUpload mints a presigned PUT for a photo, its thumbnail, or an
// attachment, scoped under an item the caller's family actually owns.
//
//nolint:dupl // only the generated request/response types differ from CreateItem/ConfirmItemPhoto/ConfirmItemAttachment
func (h *Handler) PresignUpload(ctx context.Context, request api.PresignUploadRequestObject) (api.PresignUploadResponseObject, error) {
	principal, ok := auth.PrincipalFrom(ctx)
	if !ok {
		return api.PresignUpload401JSONResponse{UnauthorizedJSONResponse: unauthorized()}, nil
	}

	if request.Body == nil {
		return api.PresignUpload400JSONResponse{BadRequestJSONResponse: invalidUpload("the request body is empty")}, nil
	}

	result, err := h.service.PresignUpload(ctx, principal, *request.Body)

	switch {
	case asNotFound(err) != "":
		return api.PresignUpload404JSONResponse{NotFoundJSONResponse: notFound(asNotFound(err))}, nil
	case asValidation(err) != "":
		return api.PresignUpload400JSONResponse{BadRequestJSONResponse: invalidUpload(asValidation(err))}, nil
	case err != nil:
		return nil, fmt.Errorf("presign upload: %w", err)
	}

	return api.PresignUpload200JSONResponse(result), nil
}

// PresignUpload validates the request against its purpose's allowlist and
// mints a presigned PUT under a key scoped to the family and item.
func (s *Service) PresignUpload(ctx context.Context, principal auth.Principal, body api.PresignUploadRequest) (api.PresignUploadResult, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	_, err := s.store.GetItem(ctx, db.GetItemParams{ID: pgconv.UUID(body.ItemId), FamilyID: familyID})
	if wrapped := wrapNotFound(err, "item"); wrapped != nil {
		return api.PresignUploadResult{}, wrapped
	}

	if err := validateUploadRequest(body); err != nil {
		return api.PresignUploadResult{}, err
	}

	key := uploadKey(principal.FamilyID, body.ItemId, body.Purpose, body.ContentType)

	url, headers, expiresAt, err := s.storage.PresignPut(ctx, key, body.ContentType, body.ContentLength)
	if err != nil {
		return api.PresignUploadResult{}, fmt.Errorf("presign put: %w", err)
	}

	return api.PresignUploadResult{StorageKey: key, UploadUrl: url, UploadHeaders: headers, ExpiresAt: expiresAt}, nil
}

func validateUploadRequest(body api.PresignUploadRequest) error {
	switch body.Purpose {
	case api.Photo:
		return validateUploadMeta(body.ContentType, body.ContentLength, photoContentTypes, maxPhotoBytes)
	case api.PhotoThumb:
		return validateUploadMeta(body.ContentType, body.ContentLength, photoContentTypes, maxPhotoThumbBytes)
	case api.Attachment:
		return validateUploadMeta(body.ContentType, body.ContentLength, attachmentContentTypes, maxAttachmentBytes)
	default:
		return &ValidationError{Msg: "unknown upload purpose"}
	}
}

func validateUploadMeta(contentType string, contentLength int64, allowed map[string]struct{}, maxBytes int64) error {
	if _, ok := allowed[contentType]; !ok {
		return &ValidationError{Msg: "unsupported content type: " + contentType}
	}

	if contentLength <= 0 || contentLength > maxBytes {
		return &ValidationError{Msg: "content length out of bounds"}
	}

	return nil
}

// uploadKey scopes every object under its family and item, so a cross-
// family read is impossible even if a caller somehow had another family's
// key to try, and so the orphaned-object cleanup sweeper's bucket listing
// is self-describing without consulting the database for anything but
// which keys still exist.
func uploadKey(familyID, itemID uuid.UUID, purpose api.UploadPurpose, contentType string) string {
	return fmt.Sprintf("families/%s/items/%s/%s/%s%s", familyID, itemID, purposeDir(purpose), uuid.NewString(), extensionFor(contentType))
}

func purposeDir(purpose api.UploadPurpose) string {
	if purpose == api.Attachment {
		return "attachments"
	}

	return "photos" // a thumbnail lives alongside its full image
}

func extensionFor(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func invalidUpload(message string) api.BadRequestJSONResponse {
	return api.BadRequestJSONResponse{Code: "invalid_upload", Message: message}
}

// verifyUpload HEADs the real object and checks it against both the
// allowlist a presign request would have been checked against and, when a
// confirm request declared non-zero metadata of its own, that too. This is
// the gap that opens once large files stop passing through Go — core-api
// never sees the bytes, so it must ask the object store what it actually
// got rather than trust the confirm request.
func (s *Service) verifyUpload(
	ctx context.Context, key, declaredContentType string, declaredBytes int64, allowed map[string]struct{}, maxBytes int64,
) error {
	actualContentType, actualBytes, err := s.storage.HeadObject(ctx, key)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return &ValidationError{Msg: "no object was found at that storage key — upload it before confirming"}
		}

		return fmt.Errorf("head object: %w", err)
	}

	if _, ok := allowed[actualContentType]; !ok {
		return &ValidationError{Msg: "uploaded object has an unsupported content type"}
	}

	if actualBytes <= 0 || actualBytes > maxBytes {
		return &ValidationError{Msg: "uploaded object is too large"}
	}

	if declaredContentType != "" && actualContentType != declaredContentType {
		return &ValidationError{Msg: "uploaded object's content type does not match what was declared"}
	}

	if declaredBytes > 0 && actualBytes != declaredBytes {
		return &ValidationError{Msg: "uploaded object's size does not match what was declared"}
	}

	return nil
}

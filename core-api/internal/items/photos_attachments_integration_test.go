//go:build integration

// Integration tests for M4's photo/attachment flow. Unit tests cover the
// pure validation helpers (uploads_test.go); what only a real database can
// prove is the transactional part — primary promotion on delete, and the
// confirm step's real HeadObject-based rejection of a mismatched upload —
// the same split handler_integration_test.go's own header comment
// describes for M3.
package items_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func presign(t *testing.T, f fixture, itemID uuid.UUID, purpose api.UploadPurpose, contentType string, size int64) api.PresignUploadResult {
	t.Helper()

	resp, err := f.h.PresignUpload(f.ctx, api.PresignUploadRequestObject{
		Body: &api.PresignUploadRequest{
			ItemId: itemID, Purpose: purpose, ContentType: contentType, ContentLength: size,
		},
	})
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}

	result, ok := resp.(api.PresignUpload200JSONResponse)
	if !ok {
		t.Fatalf("PresignUpload returned %T, want a 200: %+v", resp, resp)
	}

	return api.PresignUploadResult(result)
}

// TestPhotoLifecycle walks confirm -> list -> make-primary -> delete ->
// promote, plus the one thing only a real HeadObject call can prove: a
// confirm whose declared size disagrees with the object actually sitting
// at its storage key is rejected.
func TestPhotoLifecycle(t *testing.T) {
	f := newFixture(t)

	item := createItem(t, f, api.CreateItemRequest{
		Name: "Framed photo", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
	})

	photo1 := confirmPhotoFixture(t, f, item.Id, false)
	if !photo1.IsPrimary {
		t.Error("the first photo on an item must be primary regardless of the request")
	}

	photo2 := confirmPhotoFixture(t, f, item.Id, true)
	if !photo2.IsPrimary {
		t.Error("confirming a second photo with isPrimary: true must make it primary")
	}

	list := listPhotos(t, f, item.Id)
	if len(list) != 2 {
		t.Fatalf("ListItemPhotos = %d photos, want 2", len(list))
	}

	primaryCount := 0

	for _, p := range list {
		if p.IsPrimary {
			primaryCount++
		}
	}

	if primaryCount != 1 {
		t.Fatalf("exactly one photo must be primary, found %d", primaryCount)
	}

	// Deleting the primary promotes the survivor.
	deleteResp, err := f.h.DeleteItemPhoto(f.ctx, api.DeleteItemPhotoRequestObject{ItemId: item.Id, PhotoId: photo2.Id})
	if err != nil {
		t.Fatalf("DeleteItemPhoto: %v", err)
	}

	if _, ok := deleteResp.(api.DeleteItemPhoto204Response); !ok {
		t.Fatalf("DeleteItemPhoto returned %T, want a 204", deleteResp)
	}

	survivors := listPhotos(t, f, item.Id)
	if len(survivors) != 1 || !survivors[0].IsPrimary || survivors[0].Id != photo1.Id {
		t.Fatalf("after deleting the primary photo, want exactly [photo1, primary]; got %+v", survivors)
	}

	// A confirm whose declared size disagrees with what was actually
	// uploaded must be rejected — large files never pass through Go, so
	// this HeadObject check is the only thing standing between a lying
	// client and a wrong row.
	mismatchKey := presign(t, f, item.Id, api.Photo, "image/jpeg", 10)
	f.storage.put(mismatchKey.StorageKey, "image/jpeg", 999) // actually-uploaded size disagrees
	thumbKey := presign(t, f, item.Id, api.PhotoThumb, "image/jpeg", 10)
	f.storage.put(thumbKey.StorageKey, "image/jpeg", 10)

	confirmResp, err := f.h.ConfirmItemPhoto(f.ctx, api.ConfirmItemPhotoRequestObject{
		ItemId: item.Id,
		Body: &api.ConfirmItemPhotoRequest{
			StorageKey: mismatchKey.StorageKey, ThumbStorageKey: thumbKey.StorageKey,
			Width: 100, Height: 100, Bytes: 10, ContentType: "image/jpeg",
		},
	})
	if err != nil {
		t.Fatalf("ConfirmItemPhoto: %v", err)
	}

	if _, ok := confirmResp.(api.ConfirmItemPhoto400JSONResponse); !ok {
		t.Fatalf("ConfirmItemPhoto with a mismatched size returned %T, want a 400", confirmResp)
	}
}

// confirmPhotoFixture presigns, "uploads" and confirms one photo, returning it.
func confirmPhotoFixture(t *testing.T, f fixture, itemID uuid.UUID, isPrimary bool) api.ItemPhoto {
	t.Helper()

	full := presign(t, f, itemID, api.Photo, "image/jpeg", 100)
	f.storage.put(full.StorageKey, "image/jpeg", 100)

	thumb := presign(t, f, itemID, api.PhotoThumb, "image/jpeg", 20)
	f.storage.put(thumb.StorageKey, "image/jpeg", 20)

	resp, err := f.h.ConfirmItemPhoto(f.ctx, api.ConfirmItemPhotoRequestObject{
		ItemId: itemID,
		Body: &api.ConfirmItemPhotoRequest{
			StorageKey: full.StorageKey, ThumbStorageKey: thumb.StorageKey,
			Width: 800, Height: 600, Bytes: 100, ContentType: "image/jpeg", IsPrimary: &isPrimary,
		},
	})
	if err != nil {
		t.Fatalf("ConfirmItemPhoto: %v", err)
	}

	photo, ok := resp.(api.ConfirmItemPhoto201JSONResponse)
	if !ok {
		t.Fatalf("ConfirmItemPhoto returned %T, want a 201: %+v", resp, resp)
	}

	return api.ItemPhoto(photo)
}

func listPhotos(t *testing.T, f fixture, itemID uuid.UUID) []api.ItemPhoto {
	t.Helper()

	resp, err := f.h.ListItemPhotos(f.ctx, api.ListItemPhotosRequestObject{ItemId: itemID})
	if err != nil {
		t.Fatalf("ListItemPhotos: %v", err)
	}

	list, ok := resp.(api.ListItemPhotos200JSONResponse)
	if !ok {
		t.Fatalf("ListItemPhotos returned %T, want a 200", resp)
	}

	return list
}

// TestAttachmentLifecycle walks confirm -> list -> delete, plus the
// rejection of a confirm whose storage key was never actually uploaded to.
func TestAttachmentLifecycle(t *testing.T) {
	f := newFixture(t)

	item := createItem(t, f, api.CreateItemRequest{
		Name: "Washing machine", CategoryId: f.categoryID, LocationId: &f.locationID, Quantity: 1,
	})

	presigned := presign(t, f, item.Id, api.Attachment, "application/pdf", 500)
	f.storage.put(presigned.StorageKey, "application/pdf", 500)

	confirmResp, err := f.h.ConfirmItemAttachment(f.ctx, api.ConfirmItemAttachmentRequestObject{
		ItemId: item.Id,
		Body: &api.ConfirmItemAttachmentRequest{
			StorageKey: presigned.StorageKey, Filename: "receipt.pdf",
			ContentType: "application/pdf", Bytes: 500, Kind: api.AttachmentKindReceipt,
		},
	})
	if err != nil {
		t.Fatalf("ConfirmItemAttachment: %v", err)
	}

	attachment, ok := confirmResp.(api.ConfirmItemAttachment201JSONResponse)
	if !ok {
		t.Fatalf("ConfirmItemAttachment returned %T, want a 201: %+v", confirmResp, confirmResp)
	}

	listResp, err := f.h.ListItemAttachments(f.ctx, api.ListItemAttachmentsRequestObject{ItemId: item.Id})
	if err != nil {
		t.Fatalf("ListItemAttachments: %v", err)
	}

	list, ok := listResp.(api.ListItemAttachments200JSONResponse)
	if !ok || len(list) != 1 {
		t.Fatalf("ListItemAttachments = %T %+v, want a 200 with exactly one attachment", listResp, listResp)
	}

	deleteResp, err := f.h.DeleteItemAttachment(f.ctx, api.DeleteItemAttachmentRequestObject{
		ItemId: item.Id, AttachmentId: attachment.Id,
	})
	if err != nil {
		t.Fatalf("DeleteItemAttachment: %v", err)
	}

	if _, ok := deleteResp.(api.DeleteItemAttachment204Response); !ok {
		t.Fatalf("DeleteItemAttachment returned %T, want a 204", deleteResp)
	}

	// Confirming a key nothing was ever PUT to must be rejected — this is
	// the gap that opens once large files stop passing through Go.
	neverUploaded := presign(t, f, item.Id, api.Attachment, "application/pdf", 500)

	rejectResp, err := f.h.ConfirmItemAttachment(f.ctx, api.ConfirmItemAttachmentRequestObject{
		ItemId: item.Id,
		Body: &api.ConfirmItemAttachmentRequest{
			StorageKey: neverUploaded.StorageKey, Filename: "receipt.pdf",
			ContentType: "application/pdf", Bytes: 500, Kind: api.AttachmentKindReceipt,
		},
	})
	if err != nil {
		t.Fatalf("ConfirmItemAttachment: %v", err)
	}

	if _, ok := rejectResp.(api.ConfirmItemAttachment400JSONResponse); !ok {
		t.Fatalf("ConfirmItemAttachment for a never-uploaded key returned %T, want a 400", rejectResp)
	}
}

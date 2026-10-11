package items

import (
	"testing"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func TestValidateUploadRequest(t *testing.T) {
	tests := []struct {
		name    string
		body    api.PresignUploadRequest
		wantErr bool
	}{
		{"photo within allowlist", api.PresignUploadRequest{Purpose: api.Photo, ContentType: "image/jpeg", ContentLength: 1024}, false},
		{"photo thumb within its smaller cap", api.PresignUploadRequest{Purpose: api.PhotoThumb, ContentType: "image/png", ContentLength: maxPhotoThumbBytes}, false},
		{"attachment pdf", api.PresignUploadRequest{Purpose: api.Attachment, ContentType: "application/pdf", ContentLength: 1024}, false},
		{"photo exceeding its cap", api.PresignUploadRequest{Purpose: api.Photo, ContentType: "image/jpeg", ContentLength: maxPhotoBytes + 1}, true},
		{"thumb exceeding its smaller cap even though it would fit a full photo", api.PresignUploadRequest{Purpose: api.PhotoThumb, ContentType: "image/jpeg", ContentLength: maxPhotoThumbBytes + 1}, true},
		{"attachment content type not allowed for photos", api.PresignUploadRequest{Purpose: api.Photo, ContentType: "application/pdf", ContentLength: 1024}, true},
		{"zero length", api.PresignUploadRequest{Purpose: api.Photo, ContentType: "image/jpeg", ContentLength: 0}, true},
		{"negative length", api.PresignUploadRequest{Purpose: api.Photo, ContentType: "image/jpeg", ContentLength: -1}, true},
		{"unknown purpose", api.PresignUploadRequest{Purpose: "video", ContentType: "image/jpeg", ContentLength: 1024}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUploadRequest(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateUploadRequest(%+v) error = %v, wantErr %v", tt.body, err, tt.wantErr)
			}
		})
	}
}

func TestUploadKeyScopesUnderFamilyAndItem(t *testing.T) {
	familyID := uuid.New()
	itemID := uuid.New()

	photoKey := uploadKey(familyID, itemID, api.Photo, "image/jpeg")
	thumbKey := uploadKey(familyID, itemID, api.PhotoThumb, "image/jpeg")
	attachmentKey := uploadKey(familyID, itemID, api.Attachment, "application/pdf")

	wantPrefix := "families/" + familyID.String() + "/items/" + itemID.String() + "/"

	for _, key := range []string{photoKey, thumbKey, attachmentKey} {
		if len(key) <= len(wantPrefix) || key[:len(wantPrefix)] != wantPrefix {
			t.Errorf("uploadKey = %q, want it to start with %q", key, wantPrefix)
		}
	}

	if photoKey[len(wantPrefix):len(wantPrefix)+6] != "photos" {
		t.Errorf("photo key = %q, want a photos/ directory", photoKey)
	}

	if thumbKey[len(wantPrefix):len(wantPrefix)+6] != "photos" {
		t.Errorf("thumb key = %q, want to live alongside its full image under photos/", thumbKey)
	}

	if attachmentKey[len(wantPrefix):len(wantPrefix)+11] != "attachments" {
		t.Errorf("attachment key = %q, want an attachments/ directory", attachmentKey)
	}

	if photoKey == thumbKey {
		t.Error("the full image and its thumbnail must get distinct keys")
	}
}

func TestExtensionFor(t *testing.T) {
	tests := map[string]string{
		"image/jpeg":      ".jpg",
		"image/png":       ".png",
		"image/webp":      ".webp",
		"application/pdf": ".pdf",
		"text/plain":      "",
	}

	for contentType, want := range tests {
		if got := extensionFor(contentType); got != want {
			t.Errorf("extensionFor(%q) = %q, want %q", contentType, got, want)
		}
	}
}

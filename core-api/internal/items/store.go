package items

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// Store is the slice of the generated queries this package needs.
//
// It reaches into three other domains' tables (categories/trait_defs,
// locations/rooms, people) purely to validate that an item's references
// belong to this family before writing them — the same generated Querier
// methods those packages' own Store interfaces list, declared again here
// narrowly rather than taken as db.Querier.
type Store interface {
	CreateItem(ctx context.Context, arg db.CreateItemParams) (db.Item, error)
	GetItem(ctx context.Context, arg db.GetItemParams) (db.Item, error)
	GetItemAncestorChain(ctx context.Context, arg db.GetItemAncestorChainParams) ([]db.GetItemAncestorChainRow, error)
	GetItemDescendantDepths(ctx context.Context, arg db.GetItemDescendantDepthsParams) ([]db.GetItemDescendantDepthsRow, error)
	SearchItems(ctx context.Context, arg db.SearchItemsParams) ([]db.SearchItemsRow, error)
	ListItemPlacements(ctx context.Context, arg db.ListItemPlacementsParams) ([]db.ListItemPlacementsRow, error)
	FindSimilarItems(ctx context.Context, arg db.FindSimilarItemsParams) ([]db.FindSimilarItemsRow, error)
	ExportItems(ctx context.Context, arg db.ExportItemsParams) ([]db.ExportItemsRow, error)
	DashboardCountsByLocation(ctx context.Context, familyID pgtype.UUID) ([]db.DashboardCountsByLocationRow, error)
	DashboardCountsByCategory(ctx context.Context, familyID pgtype.UUID) ([]db.DashboardCountsByCategoryRow, error)
	DashboardValueByCurrency(ctx context.Context, familyID pgtype.UUID) ([]db.DashboardValueByCurrencyRow, error)
	ListRecentItemEvents(ctx context.Context, arg db.ListRecentItemEventsParams) ([]db.ListRecentItemEventsRow, error)
	ListItemChildren(ctx context.Context, arg db.ListItemChildrenParams) ([]db.Item, error)
	UpdateItem(ctx context.Context, arg db.UpdateItemParams) (db.Item, error)
	MoveItem(ctx context.Context, arg db.MoveItemParams) (db.Item, error)
	SetItemStatus(ctx context.Context, arg db.SetItemStatusParams) (db.Item, error)
	ArchiveItem(ctx context.Context, arg db.ArchiveItemParams) (int64, error)

	CreateItemEvent(ctx context.Context, arg db.CreateItemEventParams) (db.ItemEvent, error)
	ListItemEvents(ctx context.Context, arg db.ListItemEventsParams) ([]db.ItemEvent, error)

	CreateTag(ctx context.Context, arg db.CreateTagParams) (db.Tag, error)
	GetTag(ctx context.Context, arg db.GetTagParams) (db.Tag, error)
	ListTags(ctx context.Context, familyID pgtype.UUID) ([]db.Tag, error)
	UpdateTag(ctx context.Context, arg db.UpdateTagParams) (db.Tag, error)
	DeleteTag(ctx context.Context, arg db.DeleteTagParams) (int64, error)
	AttachItemTag(ctx context.Context, arg db.AttachItemTagParams) error
	DetachItemTag(ctx context.Context, arg db.DetachItemTagParams) (int64, error)
	ListTagsForItem(ctx context.Context, arg db.ListTagsForItemParams) ([]db.Tag, error)

	GetCategory(ctx context.Context, arg db.GetCategoryParams) (db.Category, error)
	ListTraitDefsForCategories(ctx context.Context, arg db.ListTraitDefsForCategoriesParams) ([]db.TraitDef, error)
	GetLocation(ctx context.Context, arg db.GetLocationParams) (db.Location, error)
	GetRoom(ctx context.Context, arg db.GetRoomParams) (db.Room, error)
	GetPerson(ctx context.Context, arg db.GetPersonParams) (db.Person, error)

	CreateItemPhoto(ctx context.Context, arg db.CreateItemPhotoParams) (db.ItemPhoto, error)
	ListItemPhotos(ctx context.Context, arg db.ListItemPhotosParams) ([]db.ItemPhoto, error)
	CountItemPhotos(ctx context.Context, arg db.CountItemPhotosParams) (int64, error)
	ClearItemPhotoPrimary(ctx context.Context, arg db.ClearItemPhotoPrimaryParams) error
	UpdateItemPhoto(ctx context.Context, arg db.UpdateItemPhotoParams) (db.ItemPhoto, error)
	DeleteItemPhoto(ctx context.Context, arg db.DeleteItemPhotoParams) (db.ItemPhoto, error)
	PromoteNextItemPhoto(ctx context.Context, arg db.PromoteNextItemPhotoParams) (db.ItemPhoto, error)

	CreateItemAttachment(ctx context.Context, arg db.CreateItemAttachmentParams) (db.ItemAttachment, error)
	ListItemAttachments(ctx context.Context, arg db.ListItemAttachmentsParams) ([]db.ItemAttachment, error)
	DeleteItemAttachment(ctx context.Context, arg db.DeleteItemAttachmentParams) (db.ItemAttachment, error)
}

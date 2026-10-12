package items

import (
	"context"
	"fmt"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// Dashboard summarises the family's inventory: what is where, what it cost,
// and what changed lately.
func (s *Service) Dashboard(ctx context.Context, principal auth.Principal) (api.Dashboard, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	locations, err := s.store.DashboardCountsByLocation(ctx, familyID)
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("dashboard counts by location: %w", err)
	}

	categories, err := s.store.DashboardCountsByCategory(ctx, familyID)
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("dashboard counts by category: %w", err)
	}

	values, err := s.store.DashboardValueByCurrency(ctx, familyID)
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("dashboard value: %w", err)
	}

	events, err := s.store.ListRecentItemEvents(ctx, db.ListRecentItemEventsParams{FamilyID: familyID, ResultLimit: recentEventCount})
	if err != nil {
		return api.Dashboard{}, fmt.Errorf("dashboard recent events: %w", err)
	}

	out := api.Dashboard{
		ByLocation:   make([]api.DashboardLocationCount, 0, len(locations)),
		ByCategory:   make([]api.DashboardCategoryCount, 0, len(categories)),
		Value:        make([]api.DashboardValue, 0, len(values)),
		RecentEvents: make([]api.DashboardEvent, 0, len(events)),
	}

	for _, row := range locations {
		out.ByLocation = append(out.ByLocation, api.DashboardLocationCount{
			LocationId: pgconv.ToUUID(row.LocationID), Name: row.LocationName, Count: row.ItemCount,
		})
	}

	for _, row := range categories {
		out.TotalItems += row.ItemCount
		out.ByCategory = append(out.ByCategory, api.DashboardCategoryCount{
			CategoryId: pgconv.ToUUID(row.CategoryID), Count: row.ItemCount,
		})
	}

	for _, row := range values {
		out.Value = append(out.Value, api.DashboardValue{
			Currency: row.Currency, Total: pgconv.ToNumeric(row.Total), ItemCount: row.ItemCount,
		})
	}

	for _, row := range events {
		out.RecentEvents = append(out.RecentEvents, api.DashboardEvent{
			Id:          pgconv.ToUUID(row.ID),
			ItemId:      pgconv.ToUUID(row.ItemID),
			ItemName:    row.ItemName,
			Type:        row.Type,
			ActorUserId: pgconv.ToUUID(row.ActorUserID),
			CreatedAt:   pgconv.ToTime(row.CreatedAt),
		})
	}

	return out, nil
}

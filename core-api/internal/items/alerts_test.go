package items

import (
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/olehmushka/mykomora/core-api/internal/api"
)

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)

	return &t
}

func kindsOf(alerts []api.Alert) []api.AlertKind {
	out := make([]api.AlertKind, 0, len(alerts))
	for i := range alerts {
		out = append(out, alerts[i].Kind)
	}

	return out
}

func TestClassifyExpiryBoundaries(t *testing.T) {
	today := *day(2026, time.October, 12)

	cases := []struct {
		name   string
		expiry *time.Time
		want   []api.AlertKind
	}{
		{"yesterday is expired", day(2026, time.October, 11), []api.AlertKind{api.AlertKindExpired}},
		{"today is expiring, not expired", day(2026, time.October, 12), []api.AlertKind{api.AlertKindExpiring}},
		{"day 30 is expiring", day(2026, time.November, 11), []api.AlertKind{api.AlertKindExpiring}},
		{"day 31 is quiet", day(2026, time.November, 12), nil},
		{"no expiry is quiet", nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kindsOf(classify(&alertCandidate{itemID: uuid.New(), effectiveExpiry: tc.expiry}, today))
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClassifyBelowMinimum(t *testing.T) {
	today := *day(2026, time.October, 12)
	minQty := 2.0

	if got := classify(&alertCandidate{quantity: 2, minQuantity: &minQty}, today); len(got) != 0 {
		t.Fatalf("quantity == minimum must not alert, got %v", kindsOf(got))
	}

	got := classify(&alertCandidate{quantity: 1, minQuantity: &minQty}, today)
	if len(got) != 1 || got[0].Kind != api.AlertKindBelowMinimum || got[0].DueDate != nil {
		t.Fatalf("got %+v", got)
	}

	if got := classify(&alertCandidate{quantity: 0}, today); len(got) != 0 {
		t.Fatalf("no minimum set must not alert, got %v", kindsOf(got))
	}
}

func TestClassifyWarrantyWindow(t *testing.T) {
	today := *day(2026, time.October, 12)

	cases := []struct {
		name  string
		until *time.Time
		alert bool
	}{
		{"already ended", day(2026, time.October, 11), false},
		{"ends today", day(2026, time.October, 12), true},
		{"day 60", day(2026, time.December, 11), true},
		{"day 61", day(2026, time.December, 12), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classify(&alertCandidate{warrantyUntil: tc.until}, today)
			if (len(got) == 1) != tc.alert {
				t.Fatalf("got %v, want alert=%v", kindsOf(got), tc.alert)
			}
		})
	}
}

func TestClassifyRaisesSeveralKinds(t *testing.T) {
	today := *day(2026, time.October, 12)
	minQty := 3.0

	got := classify(&alertCandidate{
		quantity: 1, minQuantity: &minQty, effectiveExpiry: day(2026, time.October, 1),
	}, today)

	if len(got) != 2 || got[0].Kind != api.AlertKindExpired || got[1].Kind != api.AlertKindBelowMinimum {
		t.Fatalf("got %v", kindsOf(got))
	}
}

func TestSortAlertsByUrgencyThenDate(t *testing.T) {
	alerts := []api.Alert{
		{Kind: api.AlertKindWarrantyEnding, ItemName: "w"},
		{Kind: api.AlertKindBelowMinimum, ItemName: "b"},
		{Kind: api.AlertKindExpiring, ItemName: "late", DueDate: dateOf(day(2026, time.October, 20))},
		{Kind: api.AlertKindExpiring, ItemName: "soon", DueDate: dateOf(day(2026, time.October, 13))},
		{Kind: api.AlertKindExpired, ItemName: "x", DueDate: dateOf(day(2026, time.October, 1))},
	}

	sortAlerts(alerts)

	want := []string{"x", "soon", "late", "b", "w"}
	for i, a := range alerts {
		if a.ItemName != want[i] {
			t.Fatalf("position %d: got %s, want %s", i, a.ItemName, want[i])
		}
	}
}

func dateOf(t *time.Time) *openapi_types.Date {
	return &openapi_types.Date{Time: *t}
}

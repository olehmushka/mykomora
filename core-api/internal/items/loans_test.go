package items

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

func TestCheckLoanOwnedStatus(t *testing.T) {
	for _, tc := range []struct {
		from, to api.ItemStatus
		wantErr  bool
	}{
		{api.InUse, api.Lent, true},
		{api.Lent, api.InUse, true},
		{api.Lent, api.Stored, true},
		{api.Lent, api.Sold, false}, // leaving for a terminal status closes the loan
		{api.InUse, api.Stored, false},
		{api.Stored, api.Discarded, false},
	} {
		if err := checkLoanOwnedStatus(tc.from, tc.to); (err != nil) != tc.wantErr {
			t.Errorf("checkLoanOwnedStatus(%s, %s) = %v, wantErr %v", tc.from, tc.to, err, tc.wantErr)
		}
	}
}

func TestValidateLendNeedsExactlyOneBorrower(t *testing.T) {
	person, name, blank := uuid.New(), "Сусід", "  "

	for _, tc := range []struct {
		name    string
		body    api.LendItemRequest
		wantErr bool
	}{
		{"person", api.LendItemRequest{BorrowerPersonId: &person}, false},
		{"name", api.LendItemRequest{BorrowerName: &name}, false},
		{"neither", api.LendItemRequest{}, true},
		{"blank name", api.LendItemRequest{BorrowerName: &blank}, true},
		{"both", api.LendItemRequest{BorrowerPersonId: &person, BorrowerName: &name}, true},
	} {
		if _, err := validateLend(tc.body); (err != nil) != tc.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", tc.name, err, tc.wantErr)
		}
	}
}

func TestLoanAlertsOnlyForPastDueDates(t *testing.T) {
	today := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	day := func(offset int) pgtype.Date {
		d := today.AddDate(0, 0, offset)

		return pgconv.Date(&d)
	}

	loans := []db.ListOpenLoansRow{
		{ItemName: "Дриль", DueAt: day(-1), BorrowerName: pgconv.Text("Сусід")},
		{ItemName: "Драбина", DueAt: day(0)},
		{ItemName: "Пилосос", DueAt: day(5)},
		{ItemName: "Намет"},
	}

	got := loanAlerts(loans, today)
	if len(got) != 1 || got[0].ItemName != "Дриль" || got[0].Kind != api.AlertKindLoanOverdue {
		t.Fatalf("loanAlerts = %+v, want only the drill", got)
	}

	if got[0].Borrower == nil || *got[0].Borrower != "Сусід" {
		t.Errorf("borrower = %v, want Сусід", got[0].Borrower)
	}
}

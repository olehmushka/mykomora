package items

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

const exportPageSize = 500

// ExportCSV streams the family's live items as CSV. The returned reader is
// fed by a goroutine that pages through the database, so memory stays flat
// however large the inventory; a failure part-way closes the stream with
// that error, which the HTTP layer surfaces as a broken download rather than
// a silently short file.
func (s *Service) ExportCSV(ctx context.Context, principal auth.Principal) io.Reader {
	pr, pw := io.Pipe()

	go func() {
		pw.CloseWithError(s.writeExport(ctx, principal, pw))
	}()

	return pr
}

func (s *Service) writeExport(ctx context.Context, principal auth.Principal, w io.Writer) error {
	familyID := pgconv.UUID(principal.FamilyID)
	out := csv.NewWriter(w)

	if err := out.Write(csvColumns); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	after := pgconv.UUID(uuid.Nil)

	for {
		rows, err := s.store.ExportItems(ctx, db.ExportItemsParams{FamilyID: familyID, AfterID: after, PageSize: exportPageSize})
		if err != nil {
			return fmt.Errorf("export items: %w", err)
		}

		for i := range rows {
			if err := out.Write(exportRecord(&rows[i])); err != nil {
				return fmt.Errorf("write csv row: %w", err)
			}
		}

		out.Flush()

		if err := out.Error(); err != nil {
			return fmt.Errorf("flush csv: %w", err)
		}

		if len(rows) < exportPageSize {
			return nil
		}

		after = rows[len(rows)-1].ID
	}
}

func exportRecord(r *db.ExportItemsRow) []string {
	return []string{
		csvSafe(r.Name), r.CategorySlug, csvSafe(text(r.LocationName)), csvSafe(text(r.RoomName)),
		csvSafe(text(r.ContainerName)), csvSafe(text(r.Spot)), strconv.FormatBool(r.IsContainer), string(r.Status),
		number(r.Quantity), csvSafe(text(r.Unit)), number(r.MinQuantity),
		date(r.ExpiresAt), date(r.OpenedAt), intText(r.PaoMonths),
		csvSafe(text(r.Brand)), enumText(r.Condition.ItemCondition, r.Condition.Valid),
		enumText(r.Season.ItemSeason, r.Season.Valid), enumText(r.SizeSystem.SizeSystem, r.SizeSystem.Valid),
		csvSafe(text(r.SizeValue)), csvSafe(text(r.PersonName)), csvSafe(text(r.SerialNumber)),
		date(r.WarrantyUntil), date(r.PurchaseDate), number(r.PurchasePrice), text(r.PurchaseCurrency),
		csvSafe(text(r.Notes)), csvSafe(r.TagNames), traitsText(r.Traits),
	}
}

func text(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}

	return v.String
}

func number(v pgtype.Numeric) string {
	if !v.Valid {
		return ""
	}

	return strconv.FormatFloat(pgconv.ToNumeric(v), 'f', -1, 64)
}

func intText(v pgtype.Int4) string {
	if !v.Valid {
		return ""
	}

	return strconv.Itoa(int(v.Int32))
}

func date(v pgtype.Date) string {
	if t := pgconv.ToDatePtr(v); t != nil {
		return t.Format("2006-01-02")
	}

	return ""
}

func enumText[T ~string](v T, valid bool) string {
	if !valid {
		return ""
	}

	return string(v)
}

func traitsText(raw []byte) string {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return ""
	}

	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}

	return string(b)
}

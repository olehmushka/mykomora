package items

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/apimap"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
	"github.com/olehmushka/mykomora/core-api/internal/db"
	"github.com/olehmushka/mykomora/core-api/internal/pgconv"
)

// maxFitItems bounds one fits set; a family's wardrobe for one child is far
// below it, and the cap keeps the placement lookup small.
const maxFitItems = 500

// maxSizeValueLen mirrors the contract's limit on a size label.
const maxSizeValueLen = 32

// ListPersonSizes returns a person's size history, newest first.
func (s *Service) ListPersonSizes(ctx context.Context, principal auth.Principal, personID uuid.UUID) ([]api.PersonSize, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	if _, err := s.store.GetPerson(ctx, db.GetPersonParams{ID: pgconv.UUID(personID), FamilyID: familyID}); err != nil {
		return nil, wrapNotFound(err, "person")
	}

	rows, err := s.store.ListPersonSizes(ctx, db.ListPersonSizesParams{PersonID: pgconv.UUID(personID), FamilyID: familyID})
	if err != nil {
		return nil, fmt.Errorf("list person sizes: %w", err)
	}

	out := make([]api.PersonSize, 0, len(rows))
	for i := range rows {
		out = append(out, apimap.PersonSize(&rows[i]))
	}

	return out, nil
}

// RecordPersonSize appends a size to a person's history. The database fills
// in the normalised range; a label it does not know simply has none.
func (s *Service) RecordPersonSize(
	ctx context.Context, principal auth.Principal, personID uuid.UUID, body api.RecordPersonSizeRequest,
) (api.PersonSize, error) {
	value := strings.TrimSpace(body.SizeValue)
	if value == "" || len([]rune(value)) > maxSizeValueLen {
		return api.PersonSize{}, &ValidationError{Msg: "a size must be between 1 and 32 characters"}
	}

	row, err := s.store.CreatePersonSize(ctx, db.CreatePersonSizeParams{
		PersonID: pgconv.UUID(personID), FamilyID: pgconv.UUID(principal.FamilyID),
		SizeType: db.SizeType(body.SizeType), SizeSystem: db.SizeSystem(body.SizeSystem), SizeValue: value,
	})
	if wrapped := wrapNotFound(err, "person"); wrapped != nil {
		return api.PersonSize{}, wrapped
	}

	return apimap.PersonSize(&row), nil
}

// PersonFits answers "what do we already own that fits", at the person's
// current size (offset 0) or the next size up (offset 1).
func (s *Service) PersonFits(ctx context.Context, principal auth.Principal, personID uuid.UUID, offset int) (api.PersonFits, error) {
	if offset != 0 && offset != 1 {
		return api.PersonFits{}, &ValidationError{Msg: "offset must be 0 or 1"}
	}

	familyID := pgconv.UUID(principal.FamilyID)

	if _, err := s.store.GetPerson(ctx, db.GetPersonParams{ID: pgconv.UUID(personID), FamilyID: familyID}); err != nil {
		return api.PersonFits{}, wrapNotFound(err, "person")
	}

	current, err := s.store.ListCurrentPersonSizes(ctx, db.ListCurrentPersonSizesParams{PersonID: pgconv.UUID(personID), FamilyID: familyID})
	if err != nil {
		return api.PersonFits{}, fmt.Errorf("list current sizes: %w", err)
	}

	sets := make([]api.FitSet, 0, len(current))

	for i := range current {
		size := &current[i]

		lo, hi, ok := fitRange(pgconv.ToNumericPtr(size.SizeNormMin), pgconv.ToNumericPtr(size.SizeNormMax), offset)
		// An unmapped label, or a head size (items carry no head size), has
		// nothing to match against.
		if !ok || size.SizeType == db.SizeTypeHead {
			continue
		}

		set, err := s.fitSet(ctx, principal, personID, size, lo, hi)
		if err != nil {
			return api.PersonFits{}, err
		}

		sets = append(sets, set)
	}

	return api.PersonFits{Offset: offset, Sets: sets}, nil
}

func (s *Service) fitSet(
	ctx context.Context, principal auth.Principal, personID uuid.UUID, size *db.PersonSize, lo, hi float64,
) (api.FitSet, error) {
	familyID := pgconv.UUID(principal.FamilyID)

	rows, err := s.store.ListFitItems(ctx, db.ListFitItemsParams{
		FamilyID: familyID, PersonID: pgconv.UUID(personID), SizeType: string(size.SizeType),
		RangeMin: pgconv.Numeric(lo), RangeMax: pgconv.Numeric(hi), ResultLimit: maxFitItems,
	})
	if err != nil {
		return api.FitSet{}, fmt.Errorf("list fit items: %w", err)
	}

	hitRows := make([]db.SearchItemsRow, len(rows))
	for i := range rows {
		hitRows[i] = db.SearchItemsRow{Item: rows[i].Item, PrimaryThumbKey: rows[i].PrimaryThumbKey}
	}

	placements, err := s.placements(ctx, familyID, hitRows)
	if err != nil {
		return api.FitSet{}, err
	}

	hits := make([]api.ItemHit, 0, len(hitRows))

	for i := range hitRows {
		hit, err := s.toHit(ctx, &hitRows[i], placements)
		if err != nil {
			return api.FitSet{}, err
		}

		hits = append(hits, hit)
	}

	return api.FitSet{
		SizeType: api.SizeType(size.SizeType), SizeSystem: api.SizeSystem(size.SizeSystem), SizeValue: size.SizeValue,
		RangeMin: float32(lo), RangeMax: float32(hi), Items: hits,
	}, nil
}

// fitRange is the normalised range to match for an offset: the person's own
// range, or, for the next size up, the range of the same width directly above
// it. ok is false when the person's size has no range.
func fitRange(minNorm, maxNorm *float64, offset int) (lo, hi float64, ok bool) {
	if minNorm == nil || maxNorm == nil {
		return 0, 0, false
	}

	lo, hi = *minNorm, *maxNorm
	if offset == 1 {
		lo, hi = hi, hi+(hi-lo)
	}

	return lo, hi, true
}

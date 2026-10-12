// Package lookup serves barcode and ISBN lookups against public product
// catalogues, behind a shared cache.
package lookup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/olehmushka/mykomora/core-api/internal/api"
	"github.com/olehmushka/mykomora/core-api/internal/auth"
)

// Handler dispatches the lookup operation to Service.
type Handler struct {
	service *Service
	log     *slog.Logger
}

// NewHandler builds the lookup handler.
func NewHandler(service *Service, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log.With(slog.String("component", "lookup"))}
}

// LookupBarcode resolves a barcode or ISBN.
func (h *Handler) LookupBarcode(ctx context.Context, request api.LookupBarcodeRequestObject) (api.LookupBarcodeResponseObject, error) {
	if _, ok := auth.PrincipalFrom(ctx); !ok {
		return api.LookupBarcode401JSONResponse{UnauthorizedJSONResponse: api.UnauthorizedJSONResponse{
			Code: "unauthorized", Message: "sign in to continue",
		}}, nil
	}

	product, err := h.service.Lookup(ctx, request.Code)

	switch {
	case errors.Is(err, ErrInvalidCode):
		return api.LookupBarcode400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{
			Code: "invalid_barcode", Message: "that is not a valid barcode or ISBN",
		}}, nil
	case err != nil:
		return nil, fmt.Errorf("lookup barcode: %w", err)
	}

	return api.LookupBarcode200JSONResponse{
		Code: product.Code, Found: product.Found,
		Name: strPtr(product.Name), Brand: strPtr(product.Brand), CategoryHint: strPtr(product.CategoryHint),
		ImageUrl: strPtr(product.ImageURL), Source: strPtr(product.Source),
	}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

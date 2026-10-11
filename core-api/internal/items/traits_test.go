package items

import (
	"testing"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

func traitDef(key string, dataType db.TraitDataType, required bool, enumOptions ...string) db.TraitDef {
	var raw []byte
	if len(enumOptions) > 0 {
		raw = []byte(`["` + join(enumOptions, `","`) + `"]`)
	}

	return db.TraitDef{Key: key, DataType: dataType, Required: required, EnumOptions: raw}
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}

		out += p
	}

	return out
}

func TestValidateLongTailTraitsRejectsHotColumnKeys(t *testing.T) {
	for key := range hotColumnKeys {
		_, problem := validateLongTailTraits(nil, map[string]any{key: "x"}, map[string]bool{})
		if problem == "" {
			t.Errorf("key %q: want rejection, got none", key)
		}
	}
}

func TestValidateLongTailTraitsRejectsUnknownKey(t *testing.T) {
	_, problem := validateLongTailTraits(nil, map[string]any{"isbn": "123"}, map[string]bool{})
	if problem == "" {
		t.Error("want rejection for a key no trait_def declares")
	}
}

func TestValidateLongTailTraitsRequiredField(t *testing.T) {
	defs := []db.TraitDef{traitDef("author", db.TraitDataTypeText, true)}

	if _, problem := validateLongTailTraits(defs, map[string]any{}, map[string]bool{}); problem == "" {
		t.Error("want rejection when a required long-tail field is missing")
	}

	if _, problem := validateLongTailTraits(defs, map[string]any{"author": "Jane"}, map[string]bool{}); problem != "" {
		t.Errorf("unexpected rejection: %s", problem)
	}
}

func TestValidateLongTailTraitsRequiredHotColumnSatisfiedByDedicatedField(t *testing.T) {
	defs := []db.TraitDef{traitDef("season", db.TraitDataTypeEnum, true, "summer", "winter")}

	if _, problem := validateLongTailTraits(defs, map[string]any{}, map[string]bool{"season": false}); problem == "" {
		t.Error("want rejection when a required hot-column field is absent from both the dedicated field and traits")
	}

	if _, problem := validateLongTailTraits(defs, map[string]any{}, map[string]bool{"season": true}); problem != "" {
		t.Errorf("unexpected rejection when the dedicated field satisfies requiredness: %s", problem)
	}
}

func TestCoerceTraitValue(t *testing.T) {
	cases := []struct {
		name    string
		def     db.TraitDef
		value   any
		wantErr bool
	}{
		{"text ok", traitDef("material", db.TraitDataTypeText, false), "cotton", false},
		{"text wrong type", traitDef("material", db.TraitDataTypeText, false), 5.0, true},
		{"number ok", traitDef("volume", db.TraitDataTypeNumber, false), 12.5, false},
		{"number wrong type", traitDef("volume", db.TraitDataTypeNumber, false), "12.5", true},
		{"integer ok", traitDef("pieces_count", db.TraitDataTypeInteger, false), 12.0, false},
		{"integer rejects fraction", traitDef("pieces_count", db.TraitDataTypeInteger, false), 12.5, true},
		{"bool ok", traitDef("dishwasher_safe", db.TraitDataTypeBool, false), true, false},
		{"bool wrong type", traitDef("dishwasher_safe", db.TraitDataTypeBool, false), "true", true},
		{"date ok", traitDef("issued_at", db.TraitDataTypeDate, false), "2024-01-15", false},
		{"date malformed", traitDef("issued_at", db.TraitDataTypeDate, false), "15/01/2024", true},
		{"enum ok", traitDef("read_status", db.TraitDataTypeEnum, false, "unread", "read"), "read", false},
		{"enum not a member", traitDef("read_status", db.TraitDataTypeEnum, false, "unread", "read"), "lost", true},
		{"size accepted like text", traitDef("size", db.TraitDataTypeSize, false), "110", false},
		{"size wrong type", traitDef("size", db.TraitDataTypeSize, false), 110.0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problem := coerceTraitValue(tc.def, tc.value)
			if (problem != "") != tc.wantErr {
				t.Errorf("coerceTraitValue(%v) problem = %q, wantErr = %v", tc.value, problem, tc.wantErr)
			}
		})
	}
}

func TestValidateLongTailTraitsNullValueIsSkipped(t *testing.T) {
	defs := []db.TraitDef{traitDef("author", db.TraitDataTypeText, false)}

	longTail, problem := validateLongTailTraits(defs, map[string]any{"author": nil}, map[string]bool{})
	if problem != "" {
		t.Fatalf("unexpected rejection: %s", problem)
	}

	if _, present := longTail["author"]; present {
		t.Error("a null value should not be stored")
	}
}

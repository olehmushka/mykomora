package items

import (
	"encoding/json"
	"time"

	"github.com/olehmushka/mykomora/core-api/internal/db"
)

// hotColumnKeys is the trait key -> item column mapping M2's taxonomy seed
// anticipated (db/migrations/taxonomy_seed_data.go's text() helper doc
// comment) and 00008_items.sql's header spells out in full. A request's
// `traits` object may never contain one of these keys: the value belongs in
// its own dedicated request field instead, and this is the one place that
// rule is enforced.
var hotColumnKeys = map[string]struct{}{
	"size":           {},
	"size_system":    {},
	"season":         {},
	"condition":      {},
	"for_person":     {},
	"expires_at":     {},
	"opened_at":      {},
	"pao_months":     {},
	"quantity":       {},
	"unit":           {},
	"min_quantity":   {},
	"serial_number":  {},
	"warranty_until": {},
}

// isHotColumnKey reports whether key names one of items' own hot columns
// rather than a long-tail trait.
func isHotColumnKey(key string) bool {
	_, ok := hotColumnKeys[key]

	return ok
}

const dateLayout = "2006-01-02"

// validateLongTailTraits checks a request's traits map — long-tail keys
// only — against a category's resolved trait_defs (built-in ∪ family,
// non-archived): type coercion per data_type, enum membership, and
// requiredness. hotFieldsPresent reports, for each hot-column trait key,
// whether the request supplied it through its own dedicated field, so a
// required hot-column trait_def is satisfied correctly rather than by
// `traits`. Unknown keys and hot-column keys found inside traits are both
// rejected. The returned map is ready to marshal into the traits column.
func validateLongTailTraits(
	defs []db.TraitDef,
	input map[string]any,
	hotFieldsPresent map[string]bool,
) (longTail map[string]any, problem string) {
	defsByKey := make(map[string]db.TraitDef, len(defs))
	for _, d := range defs {
		defsByKey[d.Key] = d
	}

	longTail = make(map[string]any, len(input))

	for key, value := range input {
		if isHotColumnKey(key) {
			return nil, "\"" + key + "\" is a built-in field — set it directly, not inside traits"
		}

		def, known := defsByKey[key]
		if !known {
			return nil, "\"" + key + "\" is not a field of this category"
		}

		if value == nil {
			continue
		}

		if problem := coerceTraitValue(def, value); problem != "" {
			return nil, "\"" + key + "\": " + problem
		}

		longTail[key] = value
	}

	for _, def := range defs {
		if !def.Required {
			continue
		}

		if isHotColumnKey(def.Key) {
			if !hotFieldsPresent[def.Key] {
				return nil, "\"" + def.Key + "\" is required"
			}

			continue
		}

		if _, present := longTail[def.Key]; !present {
			return nil, "\"" + def.Key + "\" is required"
		}
	}

	return longTail, ""
}

// coerceTraitValue checks one decoded JSON value against its field's
// declared type. "size" is accepted like "text": full normalisation against
// size_lookup is M8's job (00008_items.sql's header), so a custom field of
// this type is validated only as a non-empty string here.
func coerceTraitValue(def db.TraitDef, value any) string {
	switch def.DataType {
	case db.TraitDataTypeText, db.TraitDataTypeSize:
		return coerceTextValue(value)
	case db.TraitDataTypeNumber:
		return coerceNumberValue(value)
	case db.TraitDataTypeInteger:
		return coerceIntegerValue(value)
	case db.TraitDataTypeBool:
		return coerceBoolValue(value)
	case db.TraitDataTypeDate:
		return coerceDateValue(value)
	case db.TraitDataTypeEnum:
		return coerceEnumValue(def.EnumOptions, value)
	}

	return ""
}

func coerceTextValue(value any) string {
	if _, ok := value.(string); !ok {
		return "must be a string"
	}

	return ""
}

func coerceNumberValue(value any) string {
	if _, ok := value.(float64); !ok {
		return "must be a number"
	}

	return ""
}

func coerceIntegerValue(value any) string {
	f, ok := value.(float64)
	if !ok || f != float64(int64(f)) {
		return "must be a whole number"
	}

	return ""
}

func coerceBoolValue(value any) string {
	if _, ok := value.(bool); !ok {
		return "must be a boolean"
	}

	return ""
}

func coerceDateValue(value any) string {
	s, ok := value.(string)
	if !ok {
		return "must be a date string (YYYY-MM-DD)"
	}

	if _, err := time.Parse(dateLayout, s); err != nil {
		return "must be a date string (YYYY-MM-DD)"
	}

	return ""
}

func coerceEnumValue(enumOptions []byte, value any) string {
	s, ok := value.(string)
	if !ok || !enumContains(enumOptions, s) {
		return "must be one of this field's options"
	}

	return ""
}

// enumContains reports whether s is one of the field's declared options. A
// malformed column (which nothing in this package writes) is treated as "no
// options", the same trade taxonomy's own decodeEnumOptions makes.
func enumContains(raw []byte, s string) bool {
	if len(raw) == 0 {
		return false
	}

	var options []string
	if err := json.Unmarshal(raw, &options); err != nil {
		return false
	}

	for _, o := range options {
		if o == s {
			return true
		}
	}

	return false
}

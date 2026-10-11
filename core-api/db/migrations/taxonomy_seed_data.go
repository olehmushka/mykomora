package migrations

// BuiltinTrait is one row the seed migration inserts into trait_defs for a
// built-in category.
//
// LabelKey is a catalogue key only — "trait.<category_slug>.<key>" — never
// literal text. The uk/en strings it resolves to live in
// internal/catalogue, matched by this exact key; coverage_test.go in that
// package asserts the two never drift apart.
type BuiltinTrait struct {
	Key         string
	DataType    string // one of the trait_data_type enum values
	Unit        *string
	EnumOptions []string // non-nil only when DataType == "enum"
	LabelKey    string
	Required    bool
	Filterable  bool
	SortOrder   int
}

// BuiltinCategory is one top-level category the seed migration inserts.
//
// NameKey is a catalogue key only, same reasoning as BuiltinTrait.LabelKey.
type BuiltinCategory struct {
	Slug      string
	Icon      string
	NameKey   string
	SortOrder int
	Traits    []BuiltinTrait
}

// text is also used for the trait keys SPEC.md's `items` table lists as real
// hot columns (size, season, for_person, expires_at, opened_at, pao_months,
// quantity, unit, min_quantity, warranty_until, condition): DataType "text"
// is a placeholder for those, because M3's item-write path is expected to
// special-case them by key name and route the value to the real column
// instead of the JSONB traits bag, rather than M2 inventing a new declared
// type for them. Call sites below note this inline with "// reserved".
func text(key, labelKey string, sortOrder int) BuiltinTrait {
	return BuiltinTrait{Key: key, DataType: "text", LabelKey: labelKey, SortOrder: sortOrder}
}

func integer(key, labelKey string, sortOrder int) BuiltinTrait {
	return BuiltinTrait{Key: key, DataType: "integer", LabelKey: labelKey, SortOrder: sortOrder}
}

func date(key, labelKey string, sortOrder int) BuiltinTrait {
	return BuiltinTrait{Key: key, DataType: "date", LabelKey: labelKey, SortOrder: sortOrder}
}

// boolean traits are filterable: a yes/no field is a cheap, meaningful filter
// (SPEC 5, filter panel driven by trait_defs.filterable).
func boolean(key, labelKey string, sortOrder int) BuiltinTrait {
	return BuiltinTrait{Key: key, DataType: "bool", LabelKey: labelKey, Filterable: true, SortOrder: sortOrder}
}

// enumTrait traits are filterable for the same reason booleans are: a closed
// set of options is exactly what the M5 filter panel wants.
func enumTrait(key, labelKey string, options []string, sortOrder int) BuiltinTrait {
	return BuiltinTrait{
		Key: key, DataType: "enum", EnumOptions: options, LabelKey: labelKey,
		Filterable: true, SortOrder: sortOrder,
	}
}

// BuiltinCategories is the single source of truth for the seed migration.
// Per-trait data_type/required/filterable are schema decisions SPEC.md does
// not make (it only names trait keys) — authored here alongside the
// catalogue labels, flagged for review in the PR that introduces this file.
var BuiltinCategories = []BuiltinCategory{
	{
		Slug: "clothing_footwear", Icon: "shirt", NameKey: "category.clothing_footwear", SortOrder: 10,
		Traits: []BuiltinTrait{
			text("size", "trait.clothing_footwear.size", 10),
			text("size_system", "trait.clothing_footwear.size_system", 20),
			text("season", "trait.clothing_footwear.season", 30),
			text("for_person", "trait.clothing_footwear.for_person", 40),
			text("color", "trait.clothing_footwear.color", 50),
			text("material", "trait.clothing_footwear.material", 60),
			text("condition", "trait.clothing_footwear.condition", 70),
			enumTrait("gender_age", "trait.clothing_footwear.gender_age",
				[]string{"unisex", "boys", "girls", "men", "women"}, 80),
		},
	},
	{
		Slug: "kids_toys", Icon: "blocks", NameKey: "category.kids_toys", SortOrder: 20,
		Traits: []BuiltinTrait{
			integer("age_min", "trait.kids_toys.age_min", 10),
			integer("age_max", "trait.kids_toys.age_max", 20),
			text("for_person", "trait.kids_toys.for_person", 30),
			integer("pieces_count", "trait.kids_toys.pieces_count", 40),
			enumTrait("battery_type", "trait.kids_toys.battery_type",
				[]string{"AA", "AAA", "CR2032", "rechargeable", "none"}, 50),
			text("condition", "trait.kids_toys.condition", 60),
		},
	},
	{
		Slug: "baby_nursery_gear", Icon: "baby", NameKey: "category.baby_nursery_gear", SortOrder: 30,
		Traits: []BuiltinTrait{
			text("for_person", "trait.baby_nursery_gear.for_person", 10),
			text("age_range", "trait.baby_nursery_gear.age_range", 20),
			date("safety_expiry", "trait.baby_nursery_gear.safety_expiry", 30),
			text("condition", "trait.baby_nursery_gear.condition", 40),
		},
	},
	{
		Slug: "electronics", Icon: "cpu", NameKey: "category.electronics", SortOrder: 40,
		Traits: []BuiltinTrait{
			text("model", "trait.electronics.model", 10),
			text("serial_number", "trait.electronics.serial_number", 20),
			text("warranty_until", "trait.electronics.warranty_until", 30),
			text("power", "trait.electronics.power", 40),
			text("connectors", "trait.electronics.connectors", 50),
			text("imei", "trait.electronics.imei", 60),
			text("condition", "trait.electronics.condition", 70),
		},
	},
	{
		Slug: "books_media", Icon: "book", NameKey: "category.books_media", SortOrder: 50,
		Traits: []BuiltinTrait{
			text("author", "trait.books_media.author", 10),
			text("isbn", "trait.books_media.isbn", 20),
			enumTrait("language", "trait.books_media.language", []string{"en", "uk", "other"}, 30),
			text("publisher", "trait.books_media.publisher", 40),
			integer("year", "trait.books_media.year", 50),
			enumTrait("format", "trait.books_media.format",
				[]string{"paperback", "hardcover", "ebook", "audiobook", "other"}, 60),
			enumTrait("read_status", "trait.books_media.read_status",
				[]string{"unread", "reading", "finished"}, 70),
		},
	},
	{
		Slug: "kitchen_tools_tableware", Icon: "utensils", NameKey: "category.kitchen_tools_tableware", SortOrder: 60,
		Traits: []BuiltinTrait{
			text("material", "trait.kitchen_tools_tableware.material", 10),
			text("capacity", "trait.kitchen_tools_tableware.capacity", 20),
			integer("set_size", "trait.kitchen_tools_tableware.set_size", 30),
			boolean("dishwasher_safe", "trait.kitchen_tools_tableware.dishwasher_safe", 40),
		},
	},
	{
		Slug: "tools_hardware", Icon: "wrench", NameKey: "category.tools_hardware", SortOrder: 70,
		Traits: []BuiltinTrait{
			enumTrait("tool_type", "trait.tools_hardware.tool_type",
				[]string{"hand", "power", "measuring", "other"}, 10),
			enumTrait("power_source", "trait.tools_hardware.power_source",
				[]string{"manual", "corded", "battery", "other"}, 20),
			text("battery_platform", "trait.tools_hardware.battery_platform", 30),
			text("spec_size", "trait.tools_hardware.spec_size", 40),
		},
	},
	{
		Slug: "cosmetics", Icon: "sparkles", NameKey: "category.cosmetics", SortOrder: 80,
		Traits: []BuiltinTrait{
			text("expires_at", "trait.cosmetics.expires_at", 10),
			text("opened_at", "trait.cosmetics.opened_at", 20),
			text("pao_months", "trait.cosmetics.pao_months", 30),
			text("volume", "trait.cosmetics.volume", 40),
			text("shade", "trait.cosmetics.shade", 50),
			enumTrait("skin_type", "trait.cosmetics.skin_type",
				[]string{"normal", "dry", "oily", "combination", "sensitive"}, 60),
			text("for_person", "trait.cosmetics.for_person", 70),
		},
	},
	{
		Slug: "hygiene_household_chemicals", Icon: "spray-can", NameKey: "category.hygiene_household_chemicals", SortOrder: 90,
		Traits: []BuiltinTrait{
			text("quantity", "trait.hygiene_household_chemicals.quantity", 10),
			text("unit", "trait.hygiene_household_chemicals.unit", 20),
			text("min_quantity", "trait.hygiene_household_chemicals.min_quantity", 30),
			text("expires_at", "trait.hygiene_household_chemicals.expires_at", 40),
			text("for_person", "trait.hygiene_household_chemicals.for_person", 50),
		},
	},
	{
		Slug: "medicines_first_aid", Icon: "pill", NameKey: "category.medicines_first_aid", SortOrder: 100,
		Traits: []BuiltinTrait{
			text("expires_at", "trait.medicines_first_aid.expires_at", 10),
			enumTrait("form", "trait.medicines_first_aid.form",
				[]string{"tablet", "capsule", "syrup", "ointment", "drops", "injection", "other"}, 20),
			text("dosage", "trait.medicines_first_aid.dosage", 30),
			text("active_substance", "trait.medicines_first_aid.active_substance", 40),
			boolean("prescription", "trait.medicines_first_aid.prescription", 50),
		},
	},
	{
		Slug: "food_pantry", Icon: "apple", NameKey: "category.food_pantry", SortOrder: 110,
		Traits: []BuiltinTrait{
			text("expires_at", "trait.food_pantry.expires_at", 10),
			text("quantity", "trait.food_pantry.quantity", 20),
			text("unit", "trait.food_pantry.unit", 30),
			enumTrait("storage_kind", "trait.food_pantry.storage_kind",
				[]string{"pantry", "fridge", "freezer"}, 40),
			text("opened_at", "trait.food_pantry.opened_at", 50),
		},
	},
	{
		Slug: "textiles_bedding", Icon: "bed", NameKey: "category.textiles_bedding", SortOrder: 120,
		Traits: []BuiltinTrait{
			enumTrait("bedding_size", "trait.textiles_bedding.bedding_size",
				[]string{"single", "double", "queen", "king", "other"}, 10),
			text("material", "trait.textiles_bedding.material", 20),
			integer("set_size", "trait.textiles_bedding.set_size", 30),
			text("season", "trait.textiles_bedding.season", 40),
		},
	},
	{
		Slug: "furniture_interior", Icon: "sofa", NameKey: "category.furniture_interior", SortOrder: 130,
		Traits: []BuiltinTrait{
			text("dimensions", "trait.furniture_interior.dimensions", 10),
			text("material", "trait.furniture_interior.material", 20),
			text("color", "trait.furniture_interior.color", 30),
		},
	},
	{
		Slug: "sports_outdoor", Icon: "dumbbell", NameKey: "category.sports_outdoor", SortOrder: 140,
		Traits: []BuiltinTrait{
			text("activity", "trait.sports_outdoor.activity", 10),
			text("size", "trait.sports_outdoor.size", 20),
			text("for_person", "trait.sports_outdoor.for_person", 30),
		},
	},
	{
		Slug: "hobby_crafts", Icon: "scissors", NameKey: "category.hobby_crafts", SortOrder: 150,
		Traits: []BuiltinTrait{
			text("craft_type", "trait.hobby_crafts.craft_type", 10),
			text("material", "trait.hobby_crafts.material", 20),
			text("quantity", "trait.hobby_crafts.quantity", 30),
			text("unit", "trait.hobby_crafts.unit", 40),
		},
	},
	{
		Slug: "office_stationery", Icon: "pencil", NameKey: "category.office_stationery", SortOrder: 160,
		Traits: []BuiltinTrait{
			text("kind", "trait.office_stationery.kind", 10),
			text("quantity", "trait.office_stationery.quantity", 20),
			text("unit", "trait.office_stationery.unit", 30),
		},
	},
	{
		Slug: "bags_accessories", Icon: "briefcase", NameKey: "category.bags_accessories", SortOrder: 170,
		Traits: []BuiltinTrait{
			text("kind", "trait.bags_accessories.kind", 10),
			text("material", "trait.bags_accessories.material", 20),
			text("color", "trait.bags_accessories.color", 30),
			text("for_person", "trait.bags_accessories.for_person", 40),
		},
	},
	{
		Slug: "auto_bike", Icon: "car", NameKey: "category.auto_bike", SortOrder: 180,
		Traits: []BuiltinTrait{
			text("vehicle", "trait.auto_bike.vehicle", 10),
			text("part_number", "trait.auto_bike.part_number", 20),
			text("size", "trait.auto_bike.size", 30),
		},
	},
	{
		Slug: "garden_balcony", Icon: "flower", NameKey: "category.garden_balcony", SortOrder: 190,
		Traits: []BuiltinTrait{
			text("kind", "trait.garden_balcony.kind", 10),
			text("season", "trait.garden_balcony.season", 20),
		},
	},
	{
		Slug: "pet_supplies", Icon: "paw-print", NameKey: "category.pet_supplies", SortOrder: 200,
		Traits: []BuiltinTrait{
			enumTrait("pet", "trait.pet_supplies.pet", []string{"dog", "cat", "bird", "fish", "other"}, 10),
			text("kind", "trait.pet_supplies.kind", 20),
			text("expires_at", "trait.pet_supplies.expires_at", 30),
		},
	},
	{
		Slug: "seasonal_decor", Icon: "party-popper", NameKey: "category.seasonal_decor", SortOrder: 210,
		Traits: []BuiltinTrait{
			text("occasion", "trait.seasonal_decor.occasion", 10),
			text("season", "trait.seasonal_decor.season", 20),
		},
	},
	{
		Slug: "documents_valuables", Icon: "file-text", NameKey: "category.documents_valuables", SortOrder: 220,
		Traits: []BuiltinTrait{
			text("doc_type", "trait.documents_valuables.doc_type", 10),
			text("expires_at", "trait.documents_valuables.expires_at", 20),
			text("issued_by", "trait.documents_valuables.issued_by", 30),
			date("issued_at", "trait.documents_valuables.issued_at", 40),
		},
	},
	{
		Slug: "other", Icon: "box", NameKey: "category.other", SortOrder: 230,
		Traits: nil,
	},
}

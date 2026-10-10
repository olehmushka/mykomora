package catalogue

// BuiltinLabels is the single source of truth for every built-in category
// and trait label. Keys must exactly match the NameKey/LabelKey values in
// db/migrations' seed data (taxonomy_seed_data.go) — coverage_test.go
// asserts the two never drift apart.
//
// Drafted by the author of M2 for the family's review before merge; these are
// not final translations, especially the Ukrainian column.
var BuiltinLabels = []Entry{
	// Categories.
	{Key: "category.clothing_footwear", En: "Clothing & footwear", Uk: "Одяг та взуття"},
	{Key: "category.kids_toys", En: "Kids' toys", Uk: "Дитячі іграшки"},
	{Key: "category.baby_nursery_gear", En: "Baby & nursery gear", Uk: "Товари для малюків"},
	{Key: "category.electronics", En: "Electronics", Uk: "Електроніка"},
	{Key: "category.books_media", En: "Books & media", Uk: "Книги та медіа"},
	{Key: "category.kitchen_tools_tableware", En: "Kitchen tools & tableware", Uk: "Кухонне приладдя та посуд"},
	{Key: "category.tools_hardware", En: "Tools & hardware", Uk: "Інструменти та фурнітура"},
	{Key: "category.cosmetics", En: "Cosmetics", Uk: "Косметика"},
	{
		Key: "category.hygiene_household_chemicals", En: "Hygiene & household chemicals",
		Uk: "Засоби гігієни та побутова хімія",
	},
	{Key: "category.medicines_first_aid", En: "Medicines & first aid", Uk: "Ліки та аптечка"},
	{Key: "category.food_pantry", En: "Food & pantry", Uk: "Їжа та запаси"},
	{Key: "category.textiles_bedding", En: "Textiles & bedding", Uk: "Текстиль та постільна білизна"},
	{Key: "category.furniture_interior", En: "Furniture & interior", Uk: "Меблі та інтер'єр"},
	{Key: "category.sports_outdoor", En: "Sports & outdoor", Uk: "Спорт та активний відпочинок"},
	{Key: "category.hobby_crafts", En: "Hobby & crafts", Uk: "Хобі та рукоділля"},
	{Key: "category.office_stationery", En: "Office & stationery", Uk: "Офіс та канцелярія"},
	{Key: "category.bags_accessories", En: "Bags & accessories", Uk: "Сумки та аксесуари"},
	{Key: "category.auto_bike", En: "Auto & bike", Uk: "Авто та велосипеди"},
	{Key: "category.garden_balcony", En: "Garden & balcony", Uk: "Сад та балкон"},
	{Key: "category.pet_supplies", En: "Pet supplies", Uk: "Товари для тварин"},
	{Key: "category.seasonal_decor", En: "Seasonal & decor", Uk: "Сезонний декор"},
	{Key: "category.documents_valuables", En: "Documents & valuables", Uk: "Документи та цінності"},
	{Key: "category.other", En: "Other", Uk: "Інше"},

	// clothing_footwear
	{Key: "trait.clothing_footwear.size", En: "Size", Uk: "Розмір"},
	{Key: "trait.clothing_footwear.size_system", En: "Size system", Uk: "Система розмірів"},
	{Key: "trait.clothing_footwear.season", En: "Season", Uk: "Сезон"},
	{Key: "trait.clothing_footwear.for_person", En: "For", Uk: "Для кого"},
	{Key: "trait.clothing_footwear.color", En: "Color", Uk: "Колір"},
	{Key: "trait.clothing_footwear.material", En: "Material", Uk: "Матеріал"},
	{Key: "trait.clothing_footwear.condition", En: "Condition", Uk: "Стан"},
	{Key: "trait.clothing_footwear.gender_age", En: "Gender / age group", Uk: "Стать / вікова група"},

	// kids_toys
	{Key: "trait.kids_toys.age_min", En: "Minimum age", Uk: "Мінімальний вік"},
	{Key: "trait.kids_toys.age_max", En: "Maximum age", Uk: "Максимальний вік"},
	{Key: "trait.kids_toys.for_person", En: "For", Uk: "Для кого"},
	{Key: "trait.kids_toys.pieces_count", En: "Number of pieces", Uk: "Кількість деталей"},
	{Key: "trait.kids_toys.battery_type", En: "Battery type", Uk: "Тип батарейки"},
	{Key: "trait.kids_toys.condition", En: "Condition", Uk: "Стан"},

	// baby_nursery_gear
	{Key: "trait.baby_nursery_gear.for_person", En: "For", Uk: "Для кого"},
	{Key: "trait.baby_nursery_gear.age_range", En: "Age range", Uk: "Віковий діапазон"},
	{Key: "trait.baby_nursery_gear.safety_expiry", En: "Safety expiry", Uk: "Термін безпечного використання"},
	{Key: "trait.baby_nursery_gear.condition", En: "Condition", Uk: "Стан"},

	// electronics
	{Key: "trait.electronics.model", En: "Model", Uk: "Модель"},
	{Key: "trait.electronics.serial_number", En: "Serial number", Uk: "Серійний номер"},
	{Key: "trait.electronics.warranty_until", En: "Warranty until", Uk: "Гарантія до"},
	{Key: "trait.electronics.power", En: "Power", Uk: "Потужність"},
	{Key: "trait.electronics.connectors", En: "Connectors", Uk: "Роз'єми"},
	{Key: "trait.electronics.imei", En: "IMEI", Uk: "IMEI"},
	{Key: "trait.electronics.condition", En: "Condition", Uk: "Стан"},

	// books_media
	{Key: "trait.books_media.author", En: "Author", Uk: "Автор"},
	{Key: "trait.books_media.isbn", En: "ISBN", Uk: "ISBN"},
	{Key: "trait.books_media.language", En: "Language", Uk: "Мова"},
	{Key: "trait.books_media.publisher", En: "Publisher", Uk: "Видавництво"},
	{Key: "trait.books_media.year", En: "Year", Uk: "Рік видання"},
	{Key: "trait.books_media.format", En: "Format", Uk: "Формат"},
	{Key: "trait.books_media.read_status", En: "Read status", Uk: "Статус читання"},

	// kitchen_tools_tableware
	{Key: "trait.kitchen_tools_tableware.material", En: "Material", Uk: "Матеріал"},
	{Key: "trait.kitchen_tools_tableware.capacity", En: "Capacity", Uk: "Об'єм"},
	{Key: "trait.kitchen_tools_tableware.set_size", En: "Set size", Uk: "Кількість у наборі"},
	{Key: "trait.kitchen_tools_tableware.dishwasher_safe", En: "Dishwasher safe", Uk: "Можна мити в посудомийці"},

	// tools_hardware
	{Key: "trait.tools_hardware.tool_type", En: "Tool type", Uk: "Тип інструмента"},
	{Key: "trait.tools_hardware.power_source", En: "Power source", Uk: "Джерело живлення"},
	{Key: "trait.tools_hardware.battery_platform", En: "Battery platform", Uk: "Акумуляторна платформа"},
	{Key: "trait.tools_hardware.spec_size", En: "Size / spec", Uk: "Розмір / типорозмір"},

	// cosmetics
	{Key: "trait.cosmetics.expires_at", En: "Expires", Uk: "Термін придатності"},
	{Key: "trait.cosmetics.opened_at", En: "Opened on", Uk: "Дата відкриття"},
	{Key: "trait.cosmetics.pao_months", En: "Period after opening (months)", Uk: "Термін після відкриття (місяців)"},
	{Key: "trait.cosmetics.volume", En: "Volume", Uk: "Об'єм"},
	{Key: "trait.cosmetics.shade", En: "Shade", Uk: "Відтінок"},
	{Key: "trait.cosmetics.skin_type", En: "Skin type", Uk: "Тип шкіри"},
	{Key: "trait.cosmetics.for_person", En: "For", Uk: "Для кого"},

	// hygiene_household_chemicals
	{Key: "trait.hygiene_household_chemicals.quantity", En: "Quantity", Uk: "Кількість"},
	{Key: "trait.hygiene_household_chemicals.unit", En: "Unit", Uk: "Одиниця виміру"},
	{Key: "trait.hygiene_household_chemicals.min_quantity", En: "Minimum quantity", Uk: "Мінімальна кількість"},
	{Key: "trait.hygiene_household_chemicals.expires_at", En: "Expires", Uk: "Термін придатності"},
	{Key: "trait.hygiene_household_chemicals.for_person", En: "For", Uk: "Для кого"},

	// medicines_first_aid
	{Key: "trait.medicines_first_aid.expires_at", En: "Expires", Uk: "Термін придатності"},
	{Key: "trait.medicines_first_aid.form", En: "Form", Uk: "Форма випуску"},
	{Key: "trait.medicines_first_aid.dosage", En: "Dosage", Uk: "Дозування"},
	{Key: "trait.medicines_first_aid.active_substance", En: "Active substance", Uk: "Діюча речовина"},
	{Key: "trait.medicines_first_aid.prescription", En: "Prescription only", Uk: "За рецептом"},

	// food_pantry
	{Key: "trait.food_pantry.expires_at", En: "Expires", Uk: "Термін придатності"},
	{Key: "trait.food_pantry.quantity", En: "Quantity", Uk: "Кількість"},
	{Key: "trait.food_pantry.unit", En: "Unit", Uk: "Одиниця виміру"},
	{Key: "trait.food_pantry.storage_kind", En: "Storage", Uk: "Умови зберігання"},
	{Key: "trait.food_pantry.opened_at", En: "Opened on", Uk: "Дата відкриття"},

	// textiles_bedding
	{Key: "trait.textiles_bedding.bedding_size", En: "Bedding size", Uk: "Розмір білизни"},
	{Key: "trait.textiles_bedding.material", En: "Material", Uk: "Матеріал"},
	{Key: "trait.textiles_bedding.set_size", En: "Set size", Uk: "Кількість у наборі"},
	{Key: "trait.textiles_bedding.season", En: "Season", Uk: "Сезон"},

	// furniture_interior
	{Key: "trait.furniture_interior.dimensions", En: "Dimensions", Uk: "Розміри"},
	{Key: "trait.furniture_interior.material", En: "Material", Uk: "Матеріал"},
	{Key: "trait.furniture_interior.color", En: "Color", Uk: "Колір"},

	// sports_outdoor
	{Key: "trait.sports_outdoor.activity", En: "Activity", Uk: "Вид активності"},
	{Key: "trait.sports_outdoor.size", En: "Size", Uk: "Розмір"},
	{Key: "trait.sports_outdoor.for_person", En: "For", Uk: "Для кого"},

	// hobby_crafts
	{Key: "trait.hobby_crafts.craft_type", En: "Craft type", Uk: "Вид рукоділля"},
	{Key: "trait.hobby_crafts.material", En: "Material", Uk: "Матеріал"},
	{Key: "trait.hobby_crafts.quantity", En: "Quantity", Uk: "Кількість"},
	{Key: "trait.hobby_crafts.unit", En: "Unit", Uk: "Одиниця виміру"},

	// office_stationery
	{Key: "trait.office_stationery.kind", En: "Kind", Uk: "Вид"},
	{Key: "trait.office_stationery.quantity", En: "Quantity", Uk: "Кількість"},
	{Key: "trait.office_stationery.unit", En: "Unit", Uk: "Одиниця виміру"},

	// bags_accessories
	{Key: "trait.bags_accessories.kind", En: "Kind", Uk: "Вид"},
	{Key: "trait.bags_accessories.material", En: "Material", Uk: "Матеріал"},
	{Key: "trait.bags_accessories.color", En: "Color", Uk: "Колір"},
	{Key: "trait.bags_accessories.for_person", En: "For", Uk: "Для кого"},

	// auto_bike
	{Key: "trait.auto_bike.vehicle", En: "Vehicle", Uk: "Транспортний засіб"},
	{Key: "trait.auto_bike.part_number", En: "Part number", Uk: "Номер деталі"},
	{Key: "trait.auto_bike.size", En: "Size", Uk: "Розмір"},

	// garden_balcony
	{Key: "trait.garden_balcony.kind", En: "Kind", Uk: "Вид"},
	{Key: "trait.garden_balcony.season", En: "Season", Uk: "Сезон"},

	// pet_supplies
	{Key: "trait.pet_supplies.pet", En: "Pet", Uk: "Тварина"},
	{Key: "trait.pet_supplies.kind", En: "Kind", Uk: "Вид"},
	{Key: "trait.pet_supplies.expires_at", En: "Expires", Uk: "Термін придатності"},

	// seasonal_decor
	{Key: "trait.seasonal_decor.occasion", En: "Occasion", Uk: "Привід"},
	{Key: "trait.seasonal_decor.season", En: "Season", Uk: "Сезон"},

	// documents_valuables
	{Key: "trait.documents_valuables.doc_type", En: "Document type", Uk: "Тип документа"},
	{Key: "trait.documents_valuables.expires_at", En: "Expires", Uk: "Термін придатності"},
	{Key: "trait.documents_valuables.issued_by", En: "Issued by", Uk: "Ким видано"},
	{Key: "trait.documents_valuables.issued_at", En: "Issued on", Uk: "Дата видачі"},
}

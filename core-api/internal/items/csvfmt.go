package items

import "strings"

// csvColumns is the CSV layout shared by export and import, so a file
// exported from mykomora can be imported back. Import also accepts any
// header it can be mapped from; these are the names it suggests.
var csvColumns = []string{
	"name", "category", "location", "room", "container", "spot", "is_container", "status",
	"quantity", "unit", "min_quantity", "expires_at", "opened_at", "pao_months",
	"brand", "condition", "season", "size_system", "size", "for_person", "serial_number",
	"warranty_until", "purchase_date", "purchase_price", "purchase_currency",
	"notes", "tags", "traits",
}

// csvSafe defuses spreadsheet formula injection: a cell that starts with
// = + - or @ would be evaluated by Excel or Sheets, so it gets a leading
// apostrophe, which csvUnsafe removes again on import. Text that already
// begins with apostrophes before such a character is quoted too, so the pair
// round-trips exactly.
func csvSafe(s string) string {
	if startsRisky(strings.TrimLeft(s, "'")) {
		return "'" + s
	}

	return s
}

// csvUnsafe undoes csvSafe, only where csvSafe could have added the quote.
func csvUnsafe(s string) string {
	if strings.HasPrefix(s, "'") && startsRisky(strings.TrimLeft(s, "'")) {
		return s[1:]
	}

	return s
}

func startsRisky(s string) bool {
	return s != "" && strings.ContainsRune("=+-@", rune(s[0]))
}

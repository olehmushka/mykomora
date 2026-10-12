package items

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// parsedCSV is an uploaded file split into its header and data records.
type parsedCSV struct {
	headers []string
	records [][]string
}

// parseCSV reads a CSV the way spreadsheets write it: optional BOM, and a
// comma, semicolon or tab delimiter (Excel in a Ukrainian locale writes
// semicolons), detected from the header line.
func parseCSV(data []byte) (parsedCSV, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	if !utf8.Valid(data) {
		return parsedCSV{}, &ValidationError{Msg: "the file is not UTF-8 text — save it as CSV (UTF-8)"}
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = detectDelimiter(data)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	all, err := reader.ReadAll()
	if err != nil {
		return parsedCSV{}, &ValidationError{Msg: "the file could not be read as CSV: " + err.Error()}
	}

	if len(all) == 0 {
		return parsedCSV{}, &ValidationError{Msg: "the file is empty"}
	}

	headers := make([]string, len(all[0]))
	for i, h := range all[0] {
		headers[i] = strings.TrimSpace(h)
	}

	return parsedCSV{headers: headers, records: all[1:]}, nil
}

func detectDelimiter(data []byte) rune {
	line, _, _ := bytes.Cut(data, []byte("\n"))

	best, bestCount := ',', bytes.Count(line, []byte(","))

	for _, d := range []rune{';', '\t'} {
		if n := bytes.Count(line, []byte(string(d))); n > bestCount {
			best, bestCount = d, n
		}
	}

	return best
}

func blankRecord(rec []string) bool {
	return !slices.ContainsFunc(rec, func(c string) bool { return strings.TrimSpace(c) != "" })
}

// headerAliases maps a normalised header to the item field it most likely
// holds. The exported column names map to themselves; the rest are common
// spreadsheet headings in English and Ukrainian.
var headerAliases = map[string]string{
	"title": "name", "item": "name", "назва": "name", "річ": "name", "назва_речі": "name",
	"категорія": "category", "location": "location", "локація": "location", "місце": "location",
	"кімната": "room", "контейнер": "container", "коробка": "container",
	"qty": "quantity", "count": "quantity", "кількість": "quantity", "к-сть": "quantity",
	"бренд": "brand", "виробник": "brand", "розмір": "size", "size_value": "size",
	"сезон": "season", "стан": "condition", "notes": "notes", "note": "notes", "comment": "notes",
	"примітки": "notes", "нотатки": "notes", "коментар": "notes",
	"tag": "tags", "теги": "tags", "person": "for_person", "для_кого": "for_person", "людина": "for_person",
	"price": "purchase_price", "ціна": "purchase_price", "currency": "purchase_currency", "валюта": "purchase_currency",
	"expires": "expires_at", "expiry": "expires_at", "термін_придатності": "expires_at",
	"warranty": "warranty_until", "гарантія": "warranty_until", "serial": "serial_number",
	"серійний_номер": "serial_number", "unit": "unit", "одиниця": "unit",
}

func normalizeHeader(h string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(h)), " ", "_"), "-", "_")
}

// suggestMapping guesses which header holds each item field. Each header is
// used for at most one field.
func suggestMapping(headers []string) map[string]string {
	out := map[string]string{}
	taken := map[string]bool{}

	for _, h := range headers {
		field := normalizeHeader(h)
		if alias, ok := headerAliases[field]; ok {
			field = alias
		}

		if !slices.Contains(csvColumns, field) || taken[h] {
			continue
		}

		if _, dup := out[field]; dup {
			continue
		}

		out[field] = h
		taken[h] = true
	}

	return out
}

var errNoNameColumn = errors.New("map a column to the item name")

// columnIndexes resolves a field→header mapping to field→column index,
// rejecting unknown fields and headers the file does not have.
func columnIndexes(headers []string, mapping map[string]string) (map[string]int, error) {
	idx := make(map[string]int, len(mapping))

	for field, header := range mapping {
		if header == "" {
			continue
		}

		if !slices.Contains(csvColumns, field) {
			return nil, &ValidationError{Msg: "unknown field: " + field}
		}

		i := slices.Index(headers, header)
		if i < 0 {
			return nil, &ValidationError{Msg: fmt.Sprintf("the file has no column %q", header)}
		}

		idx[field] = i
	}

	if _, ok := idx["name"]; !ok {
		return nil, &ValidationError{Msg: errNoNameColumn.Error()}
	}

	return idx, nil
}

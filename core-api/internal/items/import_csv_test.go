package items

import "testing"

func TestParseCSVDetectsDelimiterAndBOM(t *testing.T) {
	for name, in := range map[string]string{
		"comma":     "name,qty\nLamp,2\n",
		"semicolon": "\xef\xbb\xbfname;qty\nLamp;2\n",
		"tab":       "name\tqty\nLamp\t2\n",
	} {
		file, err := parseCSV([]byte(in))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if len(file.headers) != 2 || file.headers[0] != "name" || len(file.records) != 1 || file.records[0][1] != "2" {
			t.Errorf("%s: parsed %+v", name, file)
		}
	}
}

func TestParseCSVRejectsNonUTF8(t *testing.T) {
	if _, err := parseCSV([]byte("name\n\xff\xfe\n")); err == nil {
		t.Error("expected an error for invalid UTF-8")
	}
}

func TestSuggestMappingUsesAliasesAndEachHeaderOnce(t *testing.T) {
	got := suggestMapping([]string{"Title", "Qty", "Розмір", "Unknown", "name"})

	if got["name"] != "Title" || got["quantity"] != "Qty" || got["size"] != "Розмір" {
		t.Errorf("suggestMapping = %v", got)
	}

	if _, err := columnIndexes([]string{"a"}, map[string]string{"quantity": "a"}); err == nil {
		t.Error("a mapping without a name column should be rejected")
	}
}

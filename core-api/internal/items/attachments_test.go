package items

import "testing"

func TestValidAttachmentFilename(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantName string
		wantErr  bool
	}{
		{"trims whitespace", "  receipt.pdf  ", "receipt.pdf", false},
		{"empty after trim", "   ", "", true},
		{"too long", stringOfLen(maxFilenameLen + 1), "", true},
		{"exactly the limit", stringOfLen(maxFilenameLen), stringOfLen(maxFilenameLen), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, problem := validAttachmentFilename(tt.raw)
			if (problem != "") != tt.wantErr {
				t.Fatalf("validAttachmentFilename(%q) problem = %q, wantErr %v", tt.raw, problem, tt.wantErr)
			}

			if !tt.wantErr && name != tt.wantName {
				t.Errorf("validAttachmentFilename(%q) = %q, want %q", tt.raw, name, tt.wantName)
			}
		})
	}
}

func stringOfLen(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}

	return string(b)
}

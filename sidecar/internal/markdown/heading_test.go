package markdown

import "testing"

func TestParseHeadingLine(t *testing.T) {
	tests := []struct {
		line      string
		wantText  string
		wantLevel int
		wantOK    bool
	}{
		{"# Title", "Title", 1, true},
		{"## Section", "Section", 2, true},
		{"### Sub Section", "Sub Section", 3, true},
		{"#### Deep", "Deep", 4, true},
		{"##### Deeper", "Deeper", 5, true},
		{"###### Deepest", "Deepest", 6, true},
		{"####### Too deep", "", 0, false},
		{"#NoSpace", "", 0, false},
		{"# ", "", 0, false},
		{"Not a heading", "", 0, false},
		{"  ## Indented", "Indented", 2, true},
		{"", "", 0, false},
	}

	for _, tt := range tests {
		text, level, ok := ParseHeadingLine(tt.line)
		if ok != tt.wantOK || text != tt.wantText || level != tt.wantLevel {
			t.Errorf("ParseHeadingLine(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tt.line, text, level, ok, tt.wantText, tt.wantLevel, tt.wantOK)
		}
	}
}

func TestExtractHeadings(t *testing.T) {
	text := `# First
Some text.

## Second

More text.

### Third`

	headings := ExtractHeadings(text)
	if len(headings) != 3 {
		t.Fatalf("got %d headings, want 3", len(headings))
	}

	want := []struct {
		text  string
		level int
		line  int
	}{
		{"First", 1, 0},
		{"Second", 2, 3},
		{"Third", 3, 7},
	}

	for i, w := range want {
		h := headings[i]
		if h.Text != w.text || h.Level != w.level || h.Line != w.line {
			t.Errorf("heading[%d] = {%q, %d, line %d}, want {%q, %d, line %d}",
				i, h.Text, h.Level, h.Line, w.text, w.level, w.line)
		}
	}
}

func TestExtractHeadingsSkipsCodeFences(t *testing.T) {
	text := "# Real Heading\n```\n# Not A Heading\n```\n## Another Real"

	headings := ExtractHeadings(text)
	if len(headings) != 2 {
		t.Fatalf("got %d headings, want 2", len(headings))
	}

	if headings[0].Text != "Real Heading" {
		t.Errorf("heading[0] = %q, want %q", headings[0].Text, "Real Heading")
	}
	if headings[1].Text != "Another Real" {
		t.Errorf("heading[1] = %q, want %q", headings[1].Text, "Another Real")
	}
}

func TestExtractHeadingsWithFrontmatter(t *testing.T) {
	text := "---\ntitle: Test\n---\n\n# Main Title\n\n## Section"

	headings := ExtractHeadings(text)
	if len(headings) != 2 {
		t.Fatalf("got %d headings, want 2", len(headings))
	}

	if headings[0].Line != 4 {
		t.Errorf("heading[0].Line = %d, want 4", headings[0].Line)
	}
	if headings[1].Line != 6 {
		t.Errorf("heading[1].Line = %d, want 6", headings[1].Line)
	}
}

func TestExtractHeadingsEmpty(t *testing.T) {
	headings := ExtractHeadings("")
	if len(headings) != 0 {
		t.Errorf("got %d headings for empty text, want 0", len(headings))
	}
}

func TestExtractHeadingsByteOffsets(t *testing.T) {
	text := "# A\ntext\n## B"

	headings := ExtractHeadings(text)
	if len(headings) != 2 {
		t.Fatalf("got %d headings, want 2", len(headings))
	}

	if headings[0].ByteOffset != 0 {
		t.Errorf("heading[0].ByteOffset = %d, want 0", headings[0].ByteOffset)
	}
	// "# A\ntext\n" = 4 + 5 = 9
	if headings[1].ByteOffset != 9 {
		t.Errorf("heading[1].ByteOffset = %d, want 9", headings[1].ByteOffset)
	}
}

func TestExtractHeadingsTildeFence(t *testing.T) {
	text := "# Real\n~~~\n# Fenced\n~~~\n## Also Real"

	headings := ExtractHeadings(text)
	if len(headings) != 2 {
		t.Fatalf("got %d headings, want 2", len(headings))
	}
	if headings[0].Text != "Real" || headings[1].Text != "Also Real" {
		t.Errorf("got %q and %q", headings[0].Text, headings[1].Text)
	}
}

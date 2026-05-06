package lsp

import (
	"reflect"
	"testing"
)

func TestEncodeTokensSingleToken(t *testing.T) {
	tokens := []rawToken{
		{line: 0, startChar: 5, length: 10, tokenType: tokenTypeWikilink},
	}

	data := encodeTokens(tokens)
	want := []uint32{0, 5, 10, tokenTypeWikilink, 0}

	if !reflect.DeepEqual(data, want) {
		t.Errorf("encodeTokens = %v, want %v", data, want)
	}
}

func TestEncodeTokensSameLine(t *testing.T) {
	tokens := []rawToken{
		{line: 0, startChar: 2, length: 4, tokenType: tokenTypeWikiBracket},
		{line: 0, startChar: 10, length: 4, tokenType: tokenTypeWikiBracket},
	}

	data := encodeTokens(tokens)
	// Token 1: deltaLine=0, deltaChar=2, len=4
	// Token 2: deltaLine=0, deltaChar=8 (10-2), len=4
	want := []uint32{
		0, 2, 4, tokenTypeWikiBracket, 0,
		0, 8, 4, tokenTypeWikiBracket, 0,
	}

	if !reflect.DeepEqual(data, want) {
		t.Errorf("encodeTokens = %v, want %v", data, want)
	}
}

func TestEncodeTokensDifferentLines(t *testing.T) {
	tokens := []rawToken{
		{line: 1, startChar: 3, length: 5, tokenType: tokenTypeWikilink},
		{line: 4, startChar: 0, length: 8, tokenType: tokenTypeTag},
	}

	data := encodeTokens(tokens)
	// Token 1: deltaLine=1, deltaChar=3 (new line, absolute)
	// Token 2: deltaLine=3 (4-1), deltaChar=0 (new line, absolute)
	want := []uint32{
		1, 3, 5, tokenTypeWikilink, 0,
		3, 0, 8, tokenTypeTag, 0,
	}

	if !reflect.DeepEqual(data, want) {
		t.Errorf("encodeTokens = %v, want %v", data, want)
	}
}

func TestEncodeTokensEmpty(t *testing.T) {
	data := encodeTokens(nil)
	if len(data) != 0 {
		t.Errorf("encodeTokens(nil) = %v, want empty", data)
	}
}

func TestExtractTagTokens(t *testing.T) {
	text := "Some text #myTag and #another-tag here.\nMore #stuff"

	tokens := extractTagTokens(text)
	if len(tokens) != 3 {
		t.Fatalf("got %d tag tokens, want 3", len(tokens))
	}

	if tokens[0].line != 0 || tokens[0].tokenType != tokenTypeTag {
		t.Errorf("token[0] = {line: %d, type: %d}, want {line: 0, type: %d}",
			tokens[0].line, tokens[0].tokenType, tokenTypeTag)
	}
}

func TestExtractTagTokensSkipsCodeFence(t *testing.T) {
	text := "#outside\n```\n#inside-fence\n```\n#after"

	tokens := extractTagTokens(text)
	// #outside is on a heading-like line (starts with # then space? no, "#outside" has no space)
	// Actually "#outside" starts with # but next char is 'o', not ' ', so it's NOT a heading.
	// But our heading skip check is: starts with # and trimmed[1] == ' '.
	// "#outside" -> trimmed[1] == 'o' != ' ', so it won't be skipped as a heading.
	// However tagRe requires a space or start-of-line before #.
	// "#outside" is at start of line, so the regex `(?:^|[ \t])#...` matches with ^.
	// #inside-fence is in a code fence, skipped.
	// #after is at start of line, matches.
	if len(tokens) != 2 {
		t.Fatalf("got %d tag tokens, want 2", len(tokens))
	}
}

func TestExtractTagTokensSkipsHeadings(t *testing.T) {
	text := "# Heading\nSome text #tag"

	tokens := extractTagTokens(text)
	if len(tokens) != 1 {
		t.Fatalf("got %d tag tokens, want 1", len(tokens))
	}
	if tokens[0].line != 1 {
		t.Errorf("token line = %d, want 1", tokens[0].line)
	}
}

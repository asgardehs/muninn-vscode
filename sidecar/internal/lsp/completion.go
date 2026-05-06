package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

func (s *Server) handleCompletion(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CompletionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	text := s.getDoc(params.TextDocument.URI)
	if text == "" {
		return reply(ctx, nil, nil)
	}

	// Check if completing a heading fragment inside [[target#...
	if target, partial, ok := isHeadingFragmentContext(text, params.Position); ok {
		return s.completeHeadingFragment(ctx, reply, target, partial)
	}

	// Check if completing a callout type after > [!
	if partial, ok := isCalloutContext(text, params.Position); ok {
		return s.completeCallout(ctx, reply, partial)
	}

	// Check if completing a #tag outside wikilinks.
	if partial, ok := isTagContext(text, params.Position); ok {
		return s.completeTag(ctx, reply, partial)
	}

	// Phase D will reintroduce schema-driven frontmatter value completion here,
	// reading enum vocabularies from the schema engine instead of mdbase types.

	// Only complete inside [[ context.
	if !isWikilinkContext(text, params.Position) {
		return reply(ctx, nil, nil)
	}

	// Get the partial target text being typed.
	partial := wikilinkPartial(text, params.Position)

	// Build completion items from vault note filenames.
	files := s.noteFilenames()
	var items []protocol.CompletionItem

	for _, relPath := range files {
		name := strings.TrimSuffix(relPath, ".md")
		if partial != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(partial)) {
			continue
		}

		items = append(items, protocol.CompletionItem{
			Label:      name,
			Kind:       protocol.CompletionItemKindFile,
			Detail:     relPath,
			InsertText: name + "]]",
		})
	}

	return reply(ctx, &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
}

// completeHeadingFragment provides heading completions for [[target#partial...
func (s *Server) completeHeadingFragment(ctx context.Context, reply jsonrpc2.Replier, target, partial string) error {
	relPath := s.resolveTarget(target)
	if relPath == "" {
		return reply(ctx, &protocol.CompletionList{}, nil)
	}

	headings := s.noteHeadings(relPath)
	lowerPartial := strings.ToLower(partial)
	var items []protocol.CompletionItem

	for _, h := range headings {
		if lowerPartial != "" && !strings.Contains(strings.ToLower(h.Text), lowerPartial) {
			continue
		}

		items = append(items, protocol.CompletionItem{
			Label:      h.Text,
			Kind:       protocol.CompletionItemKindReference,
			Detail:     fmt.Sprintf("H%d", h.Level),
			InsertText: h.Text + "]]",
		})
	}

	return reply(ctx, &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
}

// isWikilinkContext returns true if the cursor is after [[ on the current line.
func isWikilinkContext(text string, pos protocol.Position) bool {
	lines := strings.Split(text, "\n")
	if int(pos.Line) >= len(lines) {
		return false
	}

	line := lines[pos.Line]
	if int(pos.Character) > len(line) {
		return false
	}

	before := line[:pos.Character]

	// Find the last [[ before the cursor that isn't closed.
	lastOpen := strings.LastIndex(before, "[[")
	if lastOpen == -1 {
		return false
	}

	// Check there's no ]] between [[ and cursor.
	between := before[lastOpen:]
	return !strings.Contains(between, "]]")
}

// isHeadingFragmentContext returns true if the cursor is inside [[target#...
// It returns the target note name and the partial heading text typed so far.
func isHeadingFragmentContext(text string, pos protocol.Position) (target, partial string, ok bool) {
	lines := strings.Split(text, "\n")
	if int(pos.Line) >= len(lines) {
		return "", "", false
	}

	line := lines[pos.Line]
	if int(pos.Character) > len(line) {
		return "", "", false
	}

	before := line[:pos.Character]

	// Find the last unclosed [[.
	lastOpen := strings.LastIndex(before, "[[")
	if lastOpen == -1 {
		return "", "", false
	}

	between := before[lastOpen:]
	if strings.Contains(between, "]]") {
		return "", "", false
	}

	// Check for # after [[.
	inner := before[lastOpen+2:]
	hashIdx := strings.Index(inner, "#")
	if hashIdx == -1 {
		return "", "", false
	}

	target = strings.TrimSpace(inner[:hashIdx])
	partial = inner[hashIdx+1:]

	// Strip alias part if | is present.
	if pipeIdx := strings.Index(partial, "|"); pipeIdx != -1 {
		return "", "", false
	}

	return target, partial, target != ""
}

// isTagContext returns true if the cursor follows a # that is preceded by
// whitespace or line-start, and is NOT inside a wikilink.
func isTagContext(text string, pos protocol.Position) (partial string, ok bool) {
	lines := strings.Split(text, "\n")
	if int(pos.Line) >= len(lines) {
		return "", false
	}

	line := lines[pos.Line]
	if int(pos.Character) > len(line) {
		return "", false
	}

	before := line[:pos.Character]

	// Must not be inside a wikilink.
	if isWikilinkContext(text, pos) {
		return "", false
	}

	// Find the last # before cursor.
	hashIdx := strings.LastIndex(before, "#")
	if hashIdx == -1 {
		return "", false
	}

	// The # must be preceded by whitespace or be at line start.
	if hashIdx > 0 && before[hashIdx-1] != ' ' && before[hashIdx-1] != '\t' {
		return "", false
	}

	// The partial is everything after # up to cursor.
	partial = before[hashIdx+1:]

	// Must not contain spaces (tags are single words/hyphenated).
	if strings.ContainsAny(partial, " \t") {
		return "", false
	}

	return partial, true
}

// completeTag provides tag completions for #partial...
func (s *Server) completeTag(ctx context.Context, reply jsonrpc2.Replier, partial string) error {
	tags, err := s.vault.CollectTags()
	if err != nil {
		tags = nil
	}

	lowerPartial := strings.ToLower(partial)
	var items []protocol.CompletionItem

	for _, tag := range tags {
		if lowerPartial != "" && !strings.Contains(strings.ToLower(tag), lowerPartial) {
			continue
		}

		items = append(items, protocol.CompletionItem{
			Label:  tag,
			Kind:   protocol.CompletionItemKindValue,
			Detail: "tag",
		})
	}

	return reply(ctx, &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
}

// calloutTypes is the vocabulary of supported callout/admonition types.
var calloutTypes = []string{
	"note", "warning", "tip", "info", "danger",
	"success", "example", "quote", "bug", "abstract",
}

// isCalloutContext returns true if the cursor is after "> [!" on the current line.
func isCalloutContext(text string, pos protocol.Position) (partial string, ok bool) {
	lines := strings.Split(text, "\n")
	if int(pos.Line) >= len(lines) {
		return "", false
	}

	line := lines[pos.Line]
	if int(pos.Character) > len(line) {
		return "", false
	}

	before := line[:pos.Character]
	trimmed := strings.TrimSpace(before)

	// Must start with > [! (blockquote callout syntax).
	if !strings.HasPrefix(trimmed, "> [!") {
		return "", false
	}

	// Extract partial after > [!
	idx := strings.Index(before, "> [!")
	if idx == -1 {
		return "", false
	}

	partial = before[idx+4:]

	// If ] already present, not in callout context anymore.
	if strings.Contains(partial, "]") {
		return "", false
	}

	return partial, true
}

// completeCallout provides callout type completions for > [!partial...
func (s *Server) completeCallout(ctx context.Context, reply jsonrpc2.Replier, partial string) error {
	lowerPartial := strings.ToLower(partial)
	var items []protocol.CompletionItem

	for _, ct := range calloutTypes {
		if lowerPartial != "" && !strings.Contains(ct, lowerPartial) {
			continue
		}

		items = append(items, protocol.CompletionItem{
			Label:      ct,
			Kind:       protocol.CompletionItemKindEnum,
			Detail:     "callout",
			InsertText: ct + "] ",
		})
	}

	return reply(ctx, &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}, nil)
}

// wikilinkPartial returns the text typed so far inside [[ ... cursor.
func wikilinkPartial(text string, pos protocol.Position) string {
	lines := strings.Split(text, "\n")
	if int(pos.Line) >= len(lines) {
		return ""
	}

	line := lines[pos.Line]
	if int(pos.Character) > len(line) {
		return ""
	}

	before := line[:pos.Character]
	lastOpen := strings.LastIndex(before, "[[")
	if lastOpen == -1 {
		return ""
	}

	return before[lastOpen+2:]
}


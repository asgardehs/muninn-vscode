package lsp

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/asgardehs/muninn-sidecar/internal/wikilink"
)

// semanticTokensProviderOptions is a local definition because
// go.lsp.dev/protocol@v0.12.0 has an incomplete SemanticTokensOptions
// (missing Legend, Full, Range fields). SemanticTokensProvider is interface{},
// so we provide our own struct.
type semanticTokensProviderOptions struct {
	Legend protocol.SemanticTokensLegend `json:"legend"`
	Full   bool                          `json:"full"`
}

// Token type indices into the legend.
const (
	tokenTypeWikilink       = 0
	tokenTypeUnresolvedLink = 1
	tokenTypeWikiBracket    = 2
	tokenTypeTag            = 3
)

var semanticTokenTypes = []protocol.SemanticTokenTypes{
	"wikilink",
	"unresolvedWikilink",
	"wikiBracket",
	"tag",
}

var semanticTokenModifiers = []protocol.SemanticTokenModifiers{}

// tagRe matches #tag patterns (letter followed by word chars, hyphens, slashes).
var tagRe = regexp.MustCompile(`(?:^|[ \t])#([a-zA-Z][a-zA-Z0-9_/-]*)`)

// rawToken represents a semantic token before relative encoding.
type rawToken struct {
	line      uint32
	startChar uint32
	length    uint32
	tokenType uint32
}

func (s *Server) handleSemanticTokensFull(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.SemanticTokensParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	text := s.getDoc(params.TextDocument.URI)
	if text == "" {
		return reply(ctx, &protocol.SemanticTokens{}, nil)
	}

	var tokens []rawToken

	// Tokenize wikilinks.
	files := s.noteFilenames()
	links := wikilink.Extract(text)

	for _, link := range links {
		// Classify as resolved or unresolved.
		normalized := wikilink.NormalizeTarget(link.Target)
		contentType := uint32(tokenTypeWikilink)
		if _, ok := files[normalized]; !ok {
			contentType = tokenTypeUnresolvedLink
		}

		startLine, startChar := offsetToPosition(text, link.Start)
		endLine, endChar := offsetToPosition(text, link.End)

		// Opening [[ bracket.
		tokens = append(tokens, rawToken{
			line:      uint32(startLine),
			startChar: uint32(startChar),
			length:    2,
			tokenType: tokenTypeWikiBracket,
		})

		// Inner content (everything between [[ and ]]).
		innerStart := link.Start + 2
		innerEnd := link.End - 2
		if innerEnd > innerStart {
			iLine, iChar := offsetToPosition(text, innerStart)
			tokens = append(tokens, rawToken{
				line:      uint32(iLine),
				startChar: uint32(iChar),
				length:    uint32(innerEnd - innerStart),
				tokenType: contentType,
			})
		}

		// Closing ]] bracket.
		tokens = append(tokens, rawToken{
			line:      uint32(endLine),
			startChar: uint32(endChar) - 2,
			length:    2,
			tokenType: tokenTypeWikiBracket,
		})
	}

	// Tokenize #tags (outside code fences and headings).
	tokens = append(tokens, extractTagTokens(text)...)

	// Sort by position (required by the protocol).
	sort.Slice(tokens, func(i, j int) bool {
		if tokens[i].line != tokens[j].line {
			return tokens[i].line < tokens[j].line
		}
		return tokens[i].startChar < tokens[j].startChar
	})

	// Encode as relative positions.
	data := encodeTokens(tokens)

	return reply(ctx, &protocol.SemanticTokens{
		Data: data,
	}, nil)
}

// extractTagTokens finds #tag patterns in text, skipping code fences and heading lines.
func extractTagTokens(text string) []rawToken {
	lines := strings.Split(text, "\n")
	var tokens []rawToken
	inFence := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		// Skip heading lines (start with #).
		if strings.HasPrefix(trimmed, "#") && len(trimmed) > 1 && trimmed[1] == ' ' {
			continue
		}

		// Find tags in this line.
		matches := tagRe.FindAllStringIndex(line, -1)
		for _, m := range matches {
			// The regex may match a leading space; find the actual # position.
			hashPos := strings.Index(line[m[0]:m[1]], "#")
			if hashPos == -1 {
				continue
			}
			tagStart := m[0] + hashPos
			tagLen := m[1] - tagStart

			tokens = append(tokens, rawToken{
				line:      uint32(i),
				startChar: uint32(tagStart),
				length:    uint32(tagLen),
				tokenType: tokenTypeTag,
			})
		}
	}

	return tokens
}

// encodeTokens converts raw tokens to the LSP relative encoding.
// Each token becomes 5 uint32s: deltaLine, deltaStartChar, length, tokenType, tokenModifiers.
func encodeTokens(tokens []rawToken) []uint32 {
	data := make([]uint32, 0, len(tokens)*5)
	var prevLine, prevChar uint32

	for _, t := range tokens {
		deltaLine := t.line - prevLine
		deltaChar := t.startChar
		if deltaLine == 0 {
			deltaChar = t.startChar - prevChar
		}

		data = append(data, deltaLine, deltaChar, t.length, t.tokenType, 0)
		prevLine = t.line
		prevChar = t.startChar
	}

	return data
}

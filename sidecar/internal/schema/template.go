package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TemplateVars are the variables a template renderer expands.
//
// Variables intentionally form a small closed set — Dendron's mistake was
// allowing template logic to sprawl into a Turing-complete DSL. The supported
// names are documented in docs/specs/.../shape.md.
type TemplateVars struct {
	Now  time.Time
	User string
	Name string // hierarchy name of the note being created
}

var templateVarRe = regexp.MustCompile(`{{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*}}`)

// Render expands template variables in s. Unknown variables produce an error
// rather than silently leaving the literal {{...}} in the output.
func Render(s string, vars TemplateVars) (string, error) {
	var firstErr error
	out := templateVarRe.ReplaceAllStringFunc(s, func(match string) string {
		// Strip {{ }} and surrounding whitespace.
		inner := strings.TrimSpace(match[2 : len(match)-2])
		val, ok := lookupVar(inner, vars)
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("unknown template variable %q", inner)
			}
			return match
		}
		return val
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func lookupVar(name string, vars TemplateVars) (string, bool) {
	switch name {
	case "today":
		return vars.Now.Format("2006-01-02"), true
	case "now":
		return vars.Now.Format(time.RFC3339), true
	case "user":
		return vars.User, true
	case "name":
		return vars.Name, true
	}
	return "", false
}

// DefaultUser returns the operating-system username, or "unknown" if it can't
// be determined. Callers that have a richer source (e.g., git config) should
// override this in the TemplateVars they pass to Render.
func DefaultUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("USERNAME"); u != "" {
		return u
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Base(h)
	}
	return "unknown"
}
